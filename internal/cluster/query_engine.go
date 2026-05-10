package cluster

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"kube-ops-view/internal/models"

	corev1 "k8s.io/api/core/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// Type aliases for Kubernetes metrics
type NodeMetrics = metricsv1beta1.NodeMetrics
type PodMetrics = metricsv1beta1.PodMetrics

// QueryEngine manages concurrent cluster polling and data collection
type QueryEngine struct {
	clients   map[string]*ClusterClient
	interval  time.Duration
	timeout   time.Duration
	store     Store
	publisher EventPublisher
	metrics   MetricsRecorder
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// Store interface for storing cluster data
type Store interface {
	SetClusterData(clusterID string, data *models.ClusterData) error
	GetClusterData(clusterID string) (*models.ClusterData, error)
	SetClusterStatus(clusterID string, status *ClusterStatus) error
	GetClusterStatus(clusterID string) (*ClusterStatus, error)
	DeleteCluster(clusterID string) error
}

// EventPublisher interface for publishing cluster events
type EventPublisher interface {
	PublishClusterUpdate(clusterID string, data *models.ClusterData) error
	PublishClusterStatus(clusterID string, status *ClusterStatus) error
}

// QueryEngineConfig holds configuration for the query engine
type QueryEngineConfig struct {
	QueryInterval time.Duration
	Timeout       time.Duration
	RetryAttempts int
	RetryDelay    time.Duration
}

// NewQueryEngine creates a new QueryEngine instance
func NewQueryEngine(config *QueryEngineConfig, store Store, publisher EventPublisher, metricsRecorder MetricsRecorder) *QueryEngine {
	if metricsRecorder == nil {
		metricsRecorder = &NoOpMetricsRecorder{}
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &QueryEngine{
		clients:   make(map[string]*ClusterClient),
		interval:  config.QueryInterval,
		timeout:   config.Timeout,
		store:     store,
		publisher: publisher,
		metrics:   metricsRecorder,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// AddCluster adds a cluster client to the query engine
func (qe *QueryEngine) AddCluster(client *ClusterClient) {
	qe.mu.Lock()
	defer qe.mu.Unlock()

	qe.clients[client.ID] = client
	log.Printf("Added cluster %s to query engine", client.ID)
}

// RemoveCluster removes a cluster client from the query engine
func (qe *QueryEngine) RemoveCluster(clusterID string) {
	qe.mu.Lock()
	defer qe.mu.Unlock()

	if client, exists := qe.clients[clusterID]; exists {
		client.Close()
		delete(qe.clients, clusterID)
		log.Printf("Removed cluster %s from query engine", clusterID)
	}
}

// Start begins the query engine polling process
func (qe *QueryEngine) Start() {
	qe.mu.RLock()
	clients := make(map[string]*ClusterClient)
	for id, client := range qe.clients {
		clients[id] = client
	}
	qe.mu.RUnlock()

	// Start a goroutine for each cluster
	for _, client := range clients {
		qe.wg.Add(1)
		go qe.pollCluster(client)
	}

	log.Printf("Query engine started with %d clusters", len(clients))
}

// Stop gracefully stops the query engine
func (qe *QueryEngine) Stop() {
	log.Println("Stopping query engine...")
	qe.cancel()
	qe.wg.Wait()

	qe.mu.Lock()
	for _, client := range qe.clients {
		client.Close()
	}
	qe.mu.Unlock()

	log.Println("Query engine stopped")
}

// pollCluster continuously polls a single cluster for data
func (qe *QueryEngine) pollCluster(client *ClusterClient) {
	defer qe.wg.Done()

	ticker := time.NewTicker(qe.interval)
	defer ticker.Stop()

	log.Printf("Started polling cluster %s every %v", client.ID, qe.interval)

	// Initial query
	qe.queryClusterWithRetry(client)

	for {
		select {
		case <-qe.ctx.Done():
			log.Printf("Stopped polling cluster %s", client.ID)
			return
		case <-ticker.C:
			qe.queryClusterWithRetry(client)
		}
	}
}

// queryClusterWithRetry queries a cluster with retry logic
func (qe *QueryEngine) queryClusterWithRetry(client *ClusterClient) {
	const maxRetries = 3
	const retryDelay = 2 * time.Second

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-qe.ctx.Done():
				return
			case <-time.After(retryDelay):
			}
		}

		ctx, cancel := context.WithTimeout(qe.ctx, qe.timeout)
		err := qe.queryCluster(ctx, client)
		cancel()

		if err == nil {
			// Success - update cluster status
			status := &ClusterStatus{
				ID:        client.ID,
				Available: true,
				LastSeen:  time.Now(),
			}
			qe.updateClusterStatus(client.ID, status)
			return
		}

		lastErr = err
		log.Printf("Attempt %d failed for cluster %s: %v", attempt+1, client.ID, err)

		// Record metrics for failed query attempt
		qe.metrics.RecordClusterQuery(client.ID, "query", 0, err)
	}

	// All retries failed - update cluster status
	status := &ClusterStatus{
		ID:        client.ID,
		Available: false,
		LastSeen:  time.Now(),
		Error:     lastErr.Error(),
	}
	qe.updateClusterStatus(client.ID, status)
}

// queryCluster performs a single query operation on a cluster
func (qe *QueryEngine) queryCluster(ctx context.Context, client *ClusterClient) error {
	startTime := time.Now()

	// Query nodes and pods concurrently
	nodesChan := make(chan *corev1.NodeList, 1)
	podsChan := make(chan *corev1.PodList, 1)
	nodeMetricsChan := make(chan []NodeMetrics, 1)
	podMetricsChan := make(chan []PodMetrics, 1)
	errChan := make(chan error, 4)

	// Query nodes
	go func() {
		nodes, err := client.GetNodes(ctx)
		if err != nil {
			errChan <- fmt.Errorf("failed to get nodes: %w", err)
			return
		}
		nodesChan <- nodes
	}()

	// Query pods
	go func() {
		pods, err := client.GetPods(ctx)
		if err != nil {
			errChan <- fmt.Errorf("failed to get pods: %w", err)
			return
		}
		podsChan <- pods
	}()

	// Query node metrics (optional)
	go func() {
		if client.IsMetricsAvailable(ctx) {
			metrics, err := client.GetNodeMetrics(ctx)
			if err != nil {
				log.Printf("Warning: failed to get node metrics for cluster %s: %v", client.ID, err)
				nodeMetricsChan <- nil
				return
			}
			nodeMetricsChan <- metrics
		} else {
			nodeMetricsChan <- nil
		}
	}()

	// Query pod metrics (optional)
	go func() {
		if client.IsMetricsAvailable(ctx) {
			metrics, err := client.GetPodMetrics(ctx)
			if err != nil {
				log.Printf("Warning: failed to get pod metrics for cluster %s: %v", client.ID, err)
				podMetricsChan <- nil
				return
			}
			podMetricsChan <- metrics
		} else {
			podMetricsChan <- nil
		}
	}()

	// Collect results
	var nodes *corev1.NodeList
	var pods *corev1.PodList
	var nodeMetrics []NodeMetrics
	var podMetrics []PodMetrics

	for i := 0; i < 4; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errChan:
			return err
		case n := <-nodesChan:
			nodes = n
		case p := <-podsChan:
			pods = p
		case nm := <-nodeMetricsChan:
			nodeMetrics = nm
		case pm := <-podMetricsChan:
			podMetrics = pm
		}
	}

	// Convert to internal data model
	clusterData, err := qe.convertToClusterData(client, nodes, pods, nodeMetrics, podMetrics)
	if err != nil {
		return fmt.Errorf("failed to convert cluster data: %w", err)
	}

	// Store the data
	if err := qe.store.SetClusterData(client.ID, clusterData); err != nil {
		log.Printf("Warning: failed to store cluster data for %s: %v", client.ID, err)
	}

	// Publish update event
	if qe.publisher != nil {
		if err := qe.publisher.PublishClusterUpdate(client.ID, clusterData); err != nil {
			log.Printf("Warning: failed to publish cluster update for %s: %v", client.ID, err)
		}
	}

	queryDuration := time.Since(startTime)
	log.Printf("Successfully queried cluster %s in %v (nodes: %d, pods: %d)",
		client.ID, queryDuration, len(nodes.Items), len(pods.Items))

	// Record metrics for successful query
	qe.metrics.RecordClusterQuery(client.ID, "query", queryDuration, nil)

	// Update cluster statistics
	runningPods := 0
	pendingPods := 0
	errorPods := 0
	for _, pod := range pods.Items {
		switch pod.Status.Phase {
		case corev1.PodRunning:
			runningPods++
		case corev1.PodPending:
			pendingPods++
		case corev1.PodFailed:
			errorPods++
		}
	}
	qe.metrics.UpdateClusterStats(client.ID, len(nodes.Items), len(pods.Items), runningPods, pendingPods, errorPods)

	return nil
}

// convertToClusterData converts Kubernetes API objects to internal data model
func (qe *QueryEngine) convertToClusterData(
	client *ClusterClient,
	nodes *corev1.NodeList,
	pods *corev1.PodList,
	nodeMetrics []NodeMetrics,
	podMetrics []PodMetrics,
) (*models.ClusterData, error) {

	clusterData := &models.ClusterData{
		ID:             client.ID,
		APIServerURL:   client.APIServer,
		Nodes:          make(map[string]*models.Node),
		UnassignedPods: make(map[string]*models.Pod),
		LastUpdate:     time.Now(),
	}

	// Create metrics lookup maps
	nodeMetricsMap := make(map[string]NodeMetrics)
	for _, nm := range nodeMetrics {
		nodeMetricsMap[nm.Name] = nm
	}

	podMetricsMap := make(map[string]PodMetrics)
	for _, pm := range podMetrics {
		key := fmt.Sprintf("%s/%s", pm.Namespace, pm.Name)
		podMetricsMap[key] = pm
	}

	// Convert nodes
	for _, node := range nodes.Items {
		modelNode, err := qe.convertNode(&node, nodeMetricsMap[node.Name])
		if err != nil {
			return nil, fmt.Errorf("failed to convert node %s: %w", node.Name, err)
		}
		clusterData.Nodes[node.Name] = modelNode
	}

	// Convert pods and assign to nodes
	for _, pod := range pods.Items {
		podKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
		modelPod, err := qe.convertPod(&pod, podMetricsMap[podKey])
		if err != nil {
			return nil, fmt.Errorf("failed to convert pod %s: %w", podKey, err)
		}

		// Assign pod to node or unassigned list
		if pod.Spec.NodeName != "" {
			if node, exists := clusterData.Nodes[pod.Spec.NodeName]; exists {
				node.Pods[podKey] = modelPod
			} else {
				log.Printf("Warning: pod %s assigned to non-existent node %s", podKey, pod.Spec.NodeName)
				clusterData.UnassignedPods[podKey] = modelPod
			}
		} else {
			clusterData.UnassignedPods[podKey] = modelPod
		}
	}

	// Calculate resource usage for nodes
	for _, node := range clusterData.Nodes {
		qe.calculateNodeResourceUsage(node)
	}

	return clusterData, nil
}

// convertNode converts a Kubernetes Node to internal Node model
func (qe *QueryEngine) convertNode(k8sNode *corev1.Node, metrics NodeMetrics) (*models.Node, error) {
	node := &models.Node{
		Name:        k8sNode.Name,
		Labels:      k8sNode.Labels,
		Annotations: k8sNode.Annotations,
		Pods:        make(map[string]*models.Pod),
		Capacity:    make(models.ResourceList),
		Allocatable: make(models.ResourceList),
	}

	// Convert node status
	node.Status = models.NodeStatus{
		Ready: qe.isNodeReady(k8sNode),
	}

	// Convert addresses
	for _, addr := range k8sNode.Status.Addresses {
		node.Status.Addresses = append(node.Status.Addresses, models.NodeAddress{
			Type:    string(addr.Type),
			Address: addr.Address,
		})
	}

	// Convert conditions
	for _, cond := range k8sNode.Status.Conditions {
		node.Status.Conditions = append(node.Status.Conditions, models.NodeCondition{
			Type:               string(cond.Type),
			Status:             string(cond.Status),
			LastHeartbeatTime:  cond.LastHeartbeatTime.Time,
			LastTransitionTime: cond.LastTransitionTime.Time,
			Reason:             cond.Reason,
			Message:            cond.Message,
		})
	}

	// Convert capacity and allocatable resources
	for resourceName, quantity := range k8sNode.Status.Capacity {
		node.Capacity[string(resourceName)] = quantity.String()
	}

	for resourceName, quantity := range k8sNode.Status.Allocatable {
		node.Allocatable[string(resourceName)] = quantity.String()
	}

	// Add metrics if available
	if metrics.Name != "" && len(metrics.Usage) > 0 {
		node.Usage = &models.ResourceUsage{}
		if cpu, exists := metrics.Usage[corev1.ResourceCPU]; exists {
			node.Usage.CPU = cpu.String()
		}
		if memory, exists := metrics.Usage[corev1.ResourceMemory]; exists {
			node.Usage.Memory = memory.String()
		}
	}

	return node, nil
}

// convertPod converts a Kubernetes Pod to internal Pod model
func (qe *QueryEngine) convertPod(k8sPod *corev1.Pod, metrics PodMetrics) (*models.Pod, error) {
	pod := &models.Pod{
		Name:      k8sPod.Name,
		Namespace: k8sPod.Namespace,
		Labels:    k8sPod.Labels,
		Phase:     string(k8sPod.Status.Phase),
		NodeName:  k8sPod.Spec.NodeName,
		Ready:     qe.isPodReady(k8sPod),
	}

	// Set start time
	if k8sPod.Status.StartTime != nil {
		pod.StartTime = &k8sPod.Status.StartTime.Time
	}

	// Set owner information
	if len(k8sPod.OwnerReferences) > 0 {
		owner := k8sPod.OwnerReferences[0]
		pod.OwnerKind = owner.Kind
		pod.OwnerName = owner.Name
	}

	// Convert containers
	for i, k8sContainer := range k8sPod.Spec.Containers {
		container := models.Container{
			Name:  k8sContainer.Name,
			Image: k8sContainer.Image,
			Resources: models.ContainerResources{
				Requests: make(models.ResourceList),
				Limits:   make(models.ResourceList),
			},
		}

		// Convert resource requests
		for resourceName, quantity := range k8sContainer.Resources.Requests {
			container.Resources.Requests[string(resourceName)] = quantity.String()
		}

		// Convert resource limits
		for resourceName, quantity := range k8sContainer.Resources.Limits {
			container.Resources.Limits[string(resourceName)] = quantity.String()
		}

		// Set container status if available
		if i < len(k8sPod.Status.ContainerStatuses) {
			containerStatus := k8sPod.Status.ContainerStatuses[i]
			container.Ready = containerStatus.Ready
			container.RestartCount = containerStatus.RestartCount

			// Convert container state
			container.State = make(map[string]interface{})
			if containerStatus.State.Running != nil {
				container.State["running"] = map[string]interface{}{
					"startedAt": containerStatus.State.Running.StartedAt.Time,
				}
			} else if containerStatus.State.Waiting != nil {
				container.State["waiting"] = map[string]interface{}{
					"reason":  containerStatus.State.Waiting.Reason,
					"message": containerStatus.State.Waiting.Message,
				}
			} else if containerStatus.State.Terminated != nil {
				container.State["terminated"] = map[string]interface{}{
					"exitCode":   containerStatus.State.Terminated.ExitCode,
					"reason":     containerStatus.State.Terminated.Reason,
					"message":    containerStatus.State.Terminated.Message,
					"startedAt":  containerStatus.State.Terminated.StartedAt.Time,
					"finishedAt": containerStatus.State.Terminated.FinishedAt.Time,
				}
			}
		}

		// Add container metrics if available
		if metrics.Name != "" && len(metrics.Containers) > 0 {
			for _, containerMetrics := range metrics.Containers {
				if containerMetrics.Name == container.Name {
					container.Usage = &models.ResourceUsage{}
					if cpu, exists := containerMetrics.Usage[corev1.ResourceCPU]; exists {
						container.Usage.CPU = cpu.String()
					}
					if memory, exists := containerMetrics.Usage[corev1.ResourceMemory]; exists {
						container.Usage.Memory = memory.String()
					}
					break
				}
			}
		}

		pod.Containers = append(pod.Containers, container)
	}

	// Calculate total pod resource usage from container metrics
	if metrics.Name != "" && len(metrics.Containers) > 0 {
		qe.calculatePodResourceUsage(pod)
	}

	return pod, nil
}

// calculateNodeResourceUsage calculates total resource usage for a node based on its pods
func (qe *QueryEngine) calculateNodeResourceUsage(node *models.Node) {
	var totalCPURequests, totalMemoryRequests int64
	var totalCPULimits, totalMemoryLimits int64

	for _, pod := range node.Pods {
		if cpuReq, err := pod.GetTotalCPURequests(); err == nil {
			totalCPURequests += cpuReq
		}

		if memReq, err := pod.GetTotalMemoryRequests(); err == nil {
			totalMemoryRequests += memReq
		}

		if cpuLimit, err := pod.GetTotalCPULimits(); err == nil {
			totalCPULimits += cpuLimit
		}

		if memLimit, err := pod.GetTotalMemoryLimits(); err == nil {
			totalMemoryLimits += memLimit
		}
	}

	// If we don't have actual metrics, use requests as approximation
	if node.Usage == nil {
		node.Usage = &models.ResourceUsage{}

		if totalCPURequests > 0 {
			node.Usage.CPU = models.FormatCPU(totalCPURequests)
		}

		if totalMemoryRequests > 0 {
			node.Usage.Memory = models.FormatMemory(totalMemoryRequests)
		}
	}
}

// calculatePodResourceUsage calculates total resource usage for a pod from its containers
func (qe *QueryEngine) calculatePodResourceUsage(pod *models.Pod) {
	var totalCPU, totalMemory int64

	for _, container := range pod.Containers {
		if container.Usage != nil {
			if container.Usage.CPU != "" {
				if parsed, err := models.ParseCPU(container.Usage.CPU); err == nil && parsed != nil {
					totalCPU += parsed.MilliUnit
				}
			}

			if container.Usage.Memory != "" {
				if parsed, err := models.ParseMemory(container.Usage.Memory); err == nil && parsed != nil {
					totalMemory += parsed.MilliUnit
				}
			}
		}
	}

	if totalCPU > 0 || totalMemory > 0 {
		pod.Usage = &models.ResourceUsage{}

		if totalCPU > 0 {
			pod.Usage.CPU = models.FormatCPU(totalCPU)
		}

		if totalMemory > 0 {
			pod.Usage.Memory = models.FormatMemory(totalMemory)
		}
	}
}

// isNodeReady checks if a node is in Ready condition
func (qe *QueryEngine) isNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// isPodReady checks if a pod is ready (all containers ready)
func (qe *QueryEngine) isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// updateClusterStatus updates the cluster status in store and publishes event
func (qe *QueryEngine) updateClusterStatus(clusterID string, status *ClusterStatus) {
	if err := qe.store.SetClusterStatus(clusterID, status); err != nil {
		log.Printf("Warning: failed to store cluster status for %s: %v", clusterID, err)
	}

	if qe.publisher != nil {
		if err := qe.publisher.PublishClusterStatus(clusterID, status); err != nil {
			log.Printf("Warning: failed to publish cluster status for %s: %v", clusterID, err)
		}
	}
}

// GetClusterData retrieves the latest cluster data from store
func (qe *QueryEngine) GetClusterData(clusterID string) (*models.ClusterData, error) {
	return qe.store.GetClusterData(clusterID)
}

// GetClusterStatus retrieves the latest cluster status from store
func (qe *QueryEngine) GetClusterStatus(clusterID string) (*ClusterStatus, error) {
	return qe.store.GetClusterStatus(clusterID)
}

// GetAllClusterIDs returns all cluster IDs currently managed by the query engine
func (qe *QueryEngine) GetAllClusterIDs() []string {
	qe.mu.RLock()
	defer qe.mu.RUnlock()

	ids := make([]string, 0, len(qe.clients))
	for id := range qe.clients {
		ids = append(ids, id)
	}
	return ids
}

// SetPublisher sets the event publisher for the query engine
func (qe *QueryEngine) SetPublisher(publisher EventPublisher) {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	qe.publisher = publisher
}

// GetPublisher returns the current event publisher
func (qe *QueryEngine) GetPublisher() EventPublisher {
	qe.mu.RLock()
	defer qe.mu.RUnlock()
	return qe.publisher
}
