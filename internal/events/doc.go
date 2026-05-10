// Package events provides event publishing and Server-Sent Events (SSE) functionality
// for real-time cluster updates in the Kubernetes Operational View application.
//
// The package implements a complete event streaming system that allows clients to
// subscribe to cluster updates, deltas, and status changes through HTTP SSE connections.
//
// Key Components:
//
// EventPublisher: Interface for publishing cluster events with filtering support
// Publisher: Core implementation that manages event subscriptions and broadcasting
// SSEHandler: HTTP handler for Server-Sent Events with client management
// SSEClient: Represents a connected SSE client with filtering capabilities
//
// Features:
//
// - Real-time event streaming via Server-Sent Events
// - Cluster-based event filtering for selective updates
// - Connection management with automatic cleanup
// - Ping/keepalive mechanism for connection health
// - Graceful error handling and recovery
// - Support for multiple concurrent clients
// - Integration with store layer for persistence
//
// Usage Example:
//
//	// Create publisher with store backend
//	publisher := events.NewPublisher(store)
//
//	// Create SSE handler with configuration
//	config := events.SSEConfig{
//		PingInterval:  30 * time.Second,
//		ClientTimeout: 5 * time.Minute,
//		MaxClients:    1000,
//	}
//	handler := events.NewSSEHandler(publisher, config)
//
//	// Set up HTTP routes
//	router.GET("/events", handler.HandleSSE)
//
//	// Publish cluster updates
//	err := publisher.PublishClusterUpdate("cluster-1", clusterData)
//
// Event Types:
//
// The system supports several event types defined in the store package:
// - cluster_update: Full cluster data updates
// - cluster_delta: Incremental cluster changes
// - cluster_status: Cluster availability status
// - cluster_delete: Cluster removal notifications
//
// Client Filtering:
//
// Clients can filter events by cluster IDs using query parameters:
//
//	GET /events?clusters=cluster-1,cluster-2
//
// This allows clients to receive only events for specific clusters,
// reducing bandwidth and processing overhead.
package events
