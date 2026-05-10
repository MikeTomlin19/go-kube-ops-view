package test

import (
	"fmt"
	"testing"
	"time"

	"kube-ops-view/internal/cluster"
	"kube-ops-view/internal/models"
	"kube-ops-view/internal/store"
)

// BenchmarkClusterManagerCreation benchmarks setting up the Go cluster manager.
func BenchmarkClusterManagerCreation(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		memStore := store.NewMemoryStore()
		manager, err := cluster.NewManager(cluster.Config{
			QueryInterval: 5 * time.Second,
			Mock:          true,
		}, &benchmarkStoreAdapter{store: memStore}, &benchmarkPublisher{}, nil)
		if err != nil {
			b.Fatal(err)
		}
		if manager == nil {
			b.Fatal("expected manager")
		}
		_ = memStore.Close()
	}
}

// BenchmarkMemoryStore benchmarks memory store operations
func BenchmarkMemoryStore(b *testing.B) {
	store := store.NewMemoryStore()
	defer store.Close()

	// Create test cluster data
	clusterData := createTestClusterData("test-cluster", 50, 500)

	b.ResetTimer()
	b.ReportAllocs()

	b.Run("SetClusterData", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := store.SetClusterData(fmt.Sprintf("cluster-%d", i), clusterData)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("GetClusterData", func(b *testing.B) {
		// Setup data first
		store.SetClusterData("test-cluster", clusterData)

		for i := 0; i < b.N; i++ {
			_, err := store.GetClusterData("test-cluster")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func createTestClusterData(clusterID string, nodeCount, podCount int) *models.ClusterData {
	data := &models.ClusterData{
		ID:             clusterID,
		APIServerURL:   "https://test.example.com",
		Nodes:          make(map[string]*models.Node, nodeCount),
		UnassignedPods: make(map[string]*models.Pod),
		LastUpdate:     time.Now(),
	}

	podsPerNode := podCount / nodeCount
	for nodeIndex := 0; nodeIndex < nodeCount; nodeIndex++ {
		nodeName := fmt.Sprintf("node-%d", nodeIndex)
		node := &models.Node{
			Name:   nodeName,
			Labels: map[string]string{"kubernetes.io/hostname": nodeName},
			Status: models.NodeStatus{
				Ready: true,
				Capacity: map[string]string{
					"cpu":    "4",
					"memory": "8Gi",
				},
			},
			Pods: make(map[string]*models.Pod, podsPerNode),
		}

		for podIndex := 0; podIndex < podsPerNode; podIndex++ {
			podName := fmt.Sprintf("pod-%d-%d", nodeIndex, podIndex)
			node.Pods[podName] = &models.Pod{
				Name:      podName,
				Namespace: "default",
				NodeName:  nodeName,
				Phase:     "Running",
			}
		}

		data.Nodes[nodeName] = node
	}

	return data
}

type benchmarkStoreAdapter struct {
	store store.Store
}

func (b *benchmarkStoreAdapter) SetClusterData(clusterID string, data *models.ClusterData) error {
	return b.store.SetClusterData(clusterID, data)
}

func (b *benchmarkStoreAdapter) GetClusterData(clusterID string) (*models.ClusterData, error) {
	return b.store.GetClusterData(clusterID)
}

func (b *benchmarkStoreAdapter) SetClusterStatus(clusterID string, status *cluster.ClusterStatus) error {
	return b.store.SetClusterStatus(clusterID, &store.ClusterStatus{
		ID:           status.ID,
		Available:    status.Available,
		LastSeen:     status.LastSeen,
		ErrorMessage: status.Error,
	})
}

func (b *benchmarkStoreAdapter) GetClusterStatus(clusterID string) (*cluster.ClusterStatus, error) {
	status, err := b.store.GetClusterStatus(clusterID)
	if err != nil {
		return nil, err
	}
	return &cluster.ClusterStatus{
		ID:        status.ID,
		Available: status.Available,
		LastSeen:  status.LastSeen,
		Error:     status.ErrorMessage,
	}, nil
}

func (b *benchmarkStoreAdapter) DeleteCluster(clusterID string) error {
	return b.store.DeleteCluster(clusterID)
}

type benchmarkPublisher struct{}

func (b *benchmarkPublisher) PublishClusterUpdate(clusterID string, data *models.ClusterData) error {
	return nil
}

func (b *benchmarkPublisher) PublishClusterStatus(clusterID string, status *cluster.ClusterStatus) error {
	return nil
}
