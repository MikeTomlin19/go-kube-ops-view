package events

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"kube-ops-view/internal/store"

	"github.com/gin-gonic/gin"
)

// SSEHandler handles Server-Sent Events for real-time cluster updates
type SSEHandler struct {
	publisher EventPublisher
	clients   map[string]*SSEClient
	mu        sync.RWMutex
	config    SSEConfig
}

// SSEClient represents a connected SSE client
type SSEClient struct {
	ID         string
	Writer     gin.ResponseWriter
	Context    context.Context
	Cancel     context.CancelFunc
	ClusterIDs []string
	Connected  time.Time
	LastPing   time.Time
	Events     <-chan store.Event
}

// SSEConfig holds configuration for SSE handler
type SSEConfig struct {
	PingInterval     time.Duration `yaml:"ping_interval" default:"30s"`
	ClientTimeout    time.Duration `yaml:"client_timeout" default:"5m"`
	MaxClients       int           `yaml:"max_clients" default:"1000"`
	BufferSize       int           `yaml:"buffer_size" default:"100"`
	EnableCORS       bool          `yaml:"enable_cors" default:"true"`
	AllowedOrigins   []string      `yaml:"allowed_origins"`
	CompressionLevel int           `yaml:"compression_level" default:"0"`
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler(publisher EventPublisher, config SSEConfig) *SSEHandler {
	// Set defaults if not provided
	if config.PingInterval == 0 {
		config.PingInterval = 30 * time.Second
	}
	if config.ClientTimeout == 0 {
		config.ClientTimeout = 5 * time.Minute
	}
	if config.MaxClients == 0 {
		config.MaxClients = 1000
	}
	if config.BufferSize == 0 {
		config.BufferSize = 100
	}

	handler := &SSEHandler{
		publisher: publisher,
		clients:   make(map[string]*SSEClient),
		config:    config,
	}

	// Start cleanup goroutine
	go handler.cleanupClients()

	return handler
}

// HandleSSE handles incoming SSE connections
func (h *SSEHandler) HandleSSE(c *gin.Context) {
	// Check if we've reached max clients
	h.mu.RLock()
	clientCount := len(h.clients)
	h.mu.RUnlock()

	if clientCount >= h.config.MaxClients {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": "Maximum number of SSE clients reached",
		})
		return
	}

	// Set SSE headers
	h.setSSEHeaders(c)

	// Parse cluster filter from query parameters
	clusterIDs := h.parseClusterFilter(c)

	// Create client context with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.config.ClientTimeout)
	defer cancel()

	// Generate client ID
	clientID := h.generateClientID(c)

	// Subscribe to events
	events, err := h.publisher.Subscribe(clusterIDs)
	if err != nil {
		log.Printf("Failed to subscribe to events for client %s: %v", clientID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to subscribe to events",
		})
		return
	}

	// Create SSE client
	client := &SSEClient{
		ID:         clientID,
		Writer:     c.Writer,
		Context:    ctx,
		Cancel:     cancel,
		ClusterIDs: clusterIDs,
		Connected:  time.Now(),
		LastPing:   time.Now(),
		Events:     events,
	}

	// Register client
	h.mu.Lock()
	h.clients[clientID] = client
	h.mu.Unlock()

	log.Printf("SSE client %s connected (clusters: %v)", clientID, clusterIDs)

	// Send initial connection message
	h.sendEvent(client, store.Event{
		Type:      "connection",
		Data:      map[string]interface{}{"status": "connected", "client_id": clientID},
		Timestamp: time.Now(),
	})

	bootstrapEvents, err := h.publisher.Snapshot(clusterIDs)
	if err != nil {
		log.Printf("Failed to send bootstrap events to client %s: %v", clientID, err)
	} else {
		for _, event := range bootstrapEvents {
			if err := h.sendEvent(client, event); err != nil {
				log.Printf("Failed to send bootstrap event to SSE client %s: %v", clientID, err)
				h.cleanupClient(clientID)
				return
			}
		}
	}

	// Start ping goroutine
	go h.pingClient(client)

	// Handle events
	h.handleClientEvents(client)

	// Cleanup on disconnect
	h.cleanupClient(clientID)
}

// setSSEHeaders sets the necessary headers for Server-Sent Events
func (h *SSEHandler) setSSEHeaders(c *gin.Context) {
	writer := c.Writer
	header := writer.Header()

	// SSE headers
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Allow-Headers", "Cache-Control")

	// CORS headers if enabled
	if h.config.EnableCORS {
		if len(h.config.AllowedOrigins) > 0 {
			origin := c.GetHeader("Origin")
			for _, allowed := range h.config.AllowedOrigins {
				if origin == allowed || allowed == "*" {
					header.Set("Access-Control-Allow-Origin", origin)
					break
				}
			}
		} else {
			header.Set("Access-Control-Allow-Origin", "*")
		}
		header.Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		header.Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization")
	}

	// Flush headers
	writer.WriteHeader(http.StatusOK)
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

// parseClusterFilter extracts cluster IDs from query parameters
func (h *SSEHandler) parseClusterFilter(c *gin.Context) []string {
	clusterParam := c.Query("clusters")
	if clusterParam == "" {
		clusterParam = c.Query("cluster_ids")
	}
	if clusterParam == "" {
		return nil // No filter, subscribe to all clusters
	}

	// Split by comma and trim whitespace
	clusters := strings.Split(clusterParam, ",")
	result := make([]string, 0, len(clusters))
	for _, cluster := range clusters {
		cluster = strings.TrimSpace(cluster)
		if cluster != "" {
			result = append(result, cluster)
		}
	}

	return result
}

// generateClientID creates a unique client identifier
func (h *SSEHandler) generateClientID(c *gin.Context) string {
	timestamp := time.Now().UnixNano()
	remoteAddr := c.ClientIP()
	return fmt.Sprintf("%s-%d", remoteAddr, timestamp)
}

// handleClientEvents processes events for a specific client
func (h *SSEHandler) handleClientEvents(client *SSEClient) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("SSE client %s handler panicked: %v", client.ID, r)
		}
	}()

	for {
		select {
		case <-client.Context.Done():
			log.Printf("SSE client %s context cancelled", client.ID)
			return

		case event, ok := <-client.Events:
			if !ok {
				log.Printf("SSE client %s event channel closed", client.ID)
				return
			}

			if err := h.sendEvent(client, event); err != nil {
				log.Printf("Failed to send event to SSE client %s: %v", client.ID, err)
				return
			}
		}
	}
}

// sendEvent sends an event to a specific client
func (h *SSEHandler) sendEvent(client *SSEClient, event store.Event) error {
	// Format event as SSE
	sseData, err := FormatSSEEvent(event)
	if err != nil {
		return fmt.Errorf("failed to format SSE event: %w", err)
	}

	// Write to client
	if _, err := client.Writer.Write(sseData); err != nil {
		return fmt.Errorf("failed to write to client: %w", err)
	}

	// Flush immediately
	if flusher, ok := client.Writer.(http.Flusher); ok {
		flusher.Flush()
	}

	return nil
}

// pingClient sends periodic ping messages to keep the connection alive
func (h *SSEHandler) pingClient(client *SSEClient) {
	ticker := time.NewTicker(h.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-client.Context.Done():
			return

		case <-ticker.C:
			pingEvent := store.Event{
				Type:      "ping",
				Data:      map[string]interface{}{"timestamp": time.Now().Unix()},
				Timestamp: time.Now(),
			}

			if err := h.sendEvent(client, pingEvent); err != nil {
				log.Printf("Failed to send ping to SSE client %s: %v", client.ID, err)
				return
			}

			client.LastPing = time.Now()
		}
	}
}

// cleanupClient removes a client and unsubscribes from events
func (h *SSEHandler) cleanupClient(clientID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if client, exists := h.clients[clientID]; exists {
		// Cancel context
		client.Cancel()

		// Unsubscribe from events
		if err := h.publisher.Unsubscribe(client.Events); err != nil {
			log.Printf("Failed to unsubscribe SSE client %s: %v", clientID, err)
		}

		// Remove from clients map
		delete(h.clients, clientID)

		log.Printf("SSE client %s disconnected", clientID)
	}
}

// cleanupClients periodically removes stale clients
func (h *SSEHandler) cleanupClients() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		h.mu.Lock()
		now := time.Now()
		staleClients := make([]string, 0)

		for clientID, client := range h.clients {
			// Check if client has been inactive for too long
			if now.Sub(client.LastPing) > h.config.ClientTimeout {
				staleClients = append(staleClients, clientID)
			}
		}
		h.mu.Unlock()

		// Cleanup stale clients
		for _, clientID := range staleClients {
			log.Printf("Cleaning up stale SSE client %s", clientID)
			h.cleanupClient(clientID)
		}
	}
}

// GetClientCount returns the number of connected SSE clients
func (h *SSEHandler) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetClientInfo returns information about connected clients
func (h *SSEHandler) GetClientInfo() []SSEClientInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()

	info := make([]SSEClientInfo, 0, len(h.clients))
	now := time.Now()

	for _, client := range h.clients {
		info = append(info, SSEClientInfo{
			ID:         client.ID,
			Connected:  client.Connected,
			LastPing:   client.LastPing,
			Duration:   now.Sub(client.Connected),
			ClusterIDs: client.ClusterIDs,
		})
	}

	return info
}

// SSEClientInfo provides information about an SSE client
type SSEClientInfo struct {
	ID         string        `json:"id"`
	Connected  time.Time     `json:"connected"`
	LastPing   time.Time     `json:"last_ping"`
	Duration   time.Duration `json:"duration"`
	ClusterIDs []string      `json:"cluster_ids"`
}

// BroadcastMessage sends a message to all connected clients
func (h *SSEHandler) BroadcastMessage(messageType string, data interface{}) {
	event := store.Event{
		Type:      messageType,
		Data:      data,
		Timestamp: time.Now(),
	}

	h.mu.RLock()
	clients := make([]*SSEClient, 0, len(h.clients))
	for _, client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	// Send to all clients
	for _, client := range clients {
		if err := h.sendEvent(client, event); err != nil {
			log.Printf("Failed to broadcast message to SSE client %s: %v", client.ID, err)
		}
	}
}

// Close shuts down the SSE handler and disconnects all clients
func (h *SSEHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Disconnect all clients
	for clientID := range h.clients {
		h.cleanupClient(clientID)
	}

	return nil
}

// Middleware for handling SSE preflight requests
func (h *SSEHandler) CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "OPTIONS" {
			h.setSSEHeaders(c)
			c.AbortWithStatus(http.StatusOK)
			return
		}
		c.Next()
	}
}

// HealthCheck returns the health status of the SSE handler
func (h *SSEHandler) HealthCheck() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return map[string]interface{}{
		"connected_clients": len(h.clients),
		"max_clients":       h.config.MaxClients,
		"ping_interval":     h.config.PingInterval.String(),
		"client_timeout":    h.config.ClientTimeout.String(),
	}
}
