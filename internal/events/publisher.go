package events

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"kube-ops-view/internal/models"
	"kube-ops-view/internal/store"
)

// EventPublisher defines the interface for publishing cluster events
type EventPublisher interface {
	PublishClusterUpdate(clusterID string, data *models.ClusterData) error
	PublishClusterDelta(clusterID string, delta interface{}) error
	PublishClusterStatus(clusterID string, status *store.ClusterStatus) error
	PublishClusterDelete(clusterID string) error
	Snapshot(clusterIDs []string) ([]store.Event, error)
	Subscribe(clusterIDs []string) (<-chan store.Event, error)
	Unsubscribe(ch <-chan store.Event) error
	Close() error
}

// Snapshot returns the current cluster data and status events for a new SSE client.
func (p *Publisher) Snapshot(clusterIDs []string) ([]store.Event, error) {
	if p.closed {
		return nil, fmt.Errorf("publisher is closed")
	}

	filter := make(map[string]bool, len(clusterIDs))
	for _, clusterID := range clusterIDs {
		filter[clusterID] = true
	}

	ids := clusterIDs
	if len(ids) == 0 {
		ids = p.store.GetClusterIDs()
	}

	events := make([]store.Event, 0, len(ids)*2+1)
	for _, clusterID := range ids {
		if len(filter) > 0 && !filter[clusterID] {
			continue
		}

		data, err := p.store.GetClusterData(clusterID)
		if err == nil && data != nil {
			events = append(events, store.Event{
				Type:      store.EventTypeClusterUpdate,
				Data:      data,
				Timestamp: time.Now(),
				ClusterID: clusterID,
			})
		}

		status, err := p.store.GetClusterStatus(clusterID)
		if err == nil && status != nil {
			events = append(events, store.Event{
				Type:      store.EventTypeClusterStatus,
				Data:      status,
				Timestamp: time.Now(),
				ClusterID: clusterID,
			})
		}
	}

	events = append(events, store.Event{
		Type:      "bootstrap_end",
		Data:      map[string]interface{}{},
		Timestamp: time.Now(),
	})

	return events, nil
}

// Publisher implements EventPublisher interface
type Publisher struct {
	store       store.Store
	subscribers map[string]*Subscriber
	mu          sync.RWMutex
	closed      bool
	nextID      int64
}

// Subscriber represents a client subscription with filtering
type Subscriber struct {
	ID         string
	Channel    chan store.Event
	ClusterIDs map[string]bool // nil means all clusters
	CreatedAt  time.Time
}

// NewPublisher creates a new event publisher
func NewPublisher(store store.Store) *Publisher {
	return &Publisher{
		store:       store,
		subscribers: make(map[string]*Subscriber),
		nextID:      1,
	}
}

// PublishClusterUpdate publishes a cluster data update event
func (p *Publisher) PublishClusterUpdate(clusterID string, data *models.ClusterData) error {
	if p.closed {
		return fmt.Errorf("publisher is closed")
	}

	event := store.Event{
		Type:      store.EventTypeClusterUpdate,
		Data:      data,
		Timestamp: time.Now(),
		ClusterID: clusterID,
	}

	// Publish to store for persistence and Redis pub/sub
	if err := p.store.PublishEvent(store.EventTypeClusterUpdate, data); err != nil {
		log.Printf("Failed to publish cluster update to store: %v", err)
		return err
	}

	// Broadcast to local subscribers
	p.broadcastEvent(event)

	return nil
}

// PublishClusterDelta publishes a cluster delta event
func (p *Publisher) PublishClusterDelta(clusterID string, delta interface{}) error {
	if p.closed {
		return fmt.Errorf("publisher is closed")
	}

	event := store.Event{
		Type:      store.EventTypeClusterDelta,
		Data:      delta,
		Timestamp: time.Now(),
		ClusterID: clusterID,
	}

	// Publish to store for persistence and Redis pub/sub
	if err := p.store.PublishEvent(store.EventTypeClusterDelta, delta); err != nil {
		log.Printf("Failed to publish cluster delta to store: %v", err)
		return err
	}

	// Broadcast to local subscribers
	p.broadcastEvent(event)

	return nil
}

// PublishClusterStatus publishes a cluster status event
func (p *Publisher) PublishClusterStatus(clusterID string, status *store.ClusterStatus) error {
	if p.closed {
		return fmt.Errorf("publisher is closed")
	}

	event := store.Event{
		Type:      store.EventTypeClusterStatus,
		Data:      status,
		Timestamp: time.Now(),
		ClusterID: clusterID,
	}

	// Publish to store for persistence and Redis pub/sub
	if err := p.store.PublishEvent(store.EventTypeClusterStatus, status); err != nil {
		log.Printf("Failed to publish cluster status to store: %v", err)
		return err
	}

	// Broadcast to local subscribers
	p.broadcastEvent(event)

	return nil
}

// PublishClusterDelete publishes a cluster deletion event
func (p *Publisher) PublishClusterDelete(clusterID string) error {
	if p.closed {
		return fmt.Errorf("publisher is closed")
	}

	event := store.Event{
		Type:      store.EventTypeClusterDelete,
		Data:      map[string]string{"cluster_id": clusterID},
		Timestamp: time.Now(),
		ClusterID: clusterID,
	}

	// Publish to store for persistence and Redis pub/sub
	if err := p.store.PublishEvent(store.EventTypeClusterDelete, map[string]string{"cluster_id": clusterID}); err != nil {
		log.Printf("Failed to publish cluster delete to store: %v", err)
		return err
	}

	// Broadcast to local subscribers
	p.broadcastEvent(event)

	return nil
}

// Subscribe creates a new subscription for events, optionally filtered by cluster IDs
func (p *Publisher) Subscribe(clusterIDs []string) (<-chan store.Event, error) {
	if p.closed {
		return nil, fmt.Errorf("publisher is closed")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Generate unique ID for subscriber
	subscriberID := fmt.Sprintf("sub-%d", p.nextID)
	p.nextID++

	// Create subscriber with buffered channel to prevent blocking
	subscriber := &Subscriber{
		ID:        subscriberID,
		Channel:   make(chan store.Event, 100), // Buffer to handle bursts
		CreatedAt: time.Now(),
	}

	// Set up cluster filtering
	if len(clusterIDs) > 0 {
		subscriber.ClusterIDs = make(map[string]bool)
		for _, id := range clusterIDs {
			subscriber.ClusterIDs[id] = true
		}
	}

	p.subscribers[subscriberID] = subscriber

	return subscriber.Channel, nil
}

// Unsubscribe removes a subscription
func (p *Publisher) Unsubscribe(ch <-chan store.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Find subscriber by channel
	var subscriberID string
	for id, subscriber := range p.subscribers {
		if subscriber.Channel == ch {
			subscriberID = id
			break
		}
	}

	if subscriberID != "" {
		if subscriber, exists := p.subscribers[subscriberID]; exists {
			close(subscriber.Channel)
			delete(p.subscribers, subscriberID)
		}
	}

	return nil
}

// broadcastEvent sends an event to all matching subscribers
func (p *Publisher) broadcastEvent(event store.Event) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, subscriber := range p.subscribers {
		// Check if subscriber is interested in this cluster
		if subscriber.ClusterIDs != nil {
			if _, interested := subscriber.ClusterIDs[event.ClusterID]; !interested {
				continue
			}
		}

		// Non-blocking send to prevent slow subscribers from blocking others
		select {
		case subscriber.Channel <- event:
		default:
			// Channel is full, log warning but don't block
			log.Printf("Subscriber channel full, dropping event for cluster %s", event.ClusterID)
		}
	}
}

// Close shuts down the publisher and closes all subscriptions
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	// Close all subscriber channels
	for _, subscriber := range p.subscribers {
		close(subscriber.Channel)
	}

	// Clear subscribers
	p.subscribers = make(map[string]*Subscriber)

	return nil
}

// GetSubscriberCount returns the number of active subscribers
func (p *Publisher) GetSubscriberCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.subscribers)
}

// GetSubscriberInfo returns information about active subscribers
func (p *Publisher) GetSubscriberInfo() []SubscriberInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	info := make([]SubscriberInfo, 0, len(p.subscribers))
	for _, subscriber := range p.subscribers {
		clusterIDs := make([]string, 0)
		if subscriber.ClusterIDs != nil {
			for id := range subscriber.ClusterIDs {
				clusterIDs = append(clusterIDs, id)
			}
		}

		info = append(info, SubscriberInfo{
			CreatedAt:  subscriber.CreatedAt,
			ClusterIDs: clusterIDs,
			BufferSize: len(subscriber.Channel),
		})
	}

	return info
}

// SubscriberInfo provides information about a subscriber
type SubscriberInfo struct {
	CreatedAt  time.Time `json:"created_at"`
	ClusterIDs []string  `json:"cluster_ids"`
	BufferSize int       `json:"buffer_size"`
}

// EventData represents the JSON structure sent over SSE
type EventData struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
	ClusterID string      `json:"cluster_id,omitempty"`
}

// FormatSSEEvent formats an event for Server-Sent Events
func FormatSSEEvent(event store.Event) ([]byte, error) {
	data, err := json.Marshal(ssePayload(event))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event data: %w", err)
	}

	sseData := fmt.Sprintf("event: %s\ndata: %s\n\n", sseEventName(event.Type), string(data))
	return []byte(sseData), nil
}

func sseEventName(eventType string) string {
	switch eventType {
	case store.EventTypeClusterUpdate:
		return "clusterupdate"
	case store.EventTypeClusterDelta:
		return "clusterdelta"
	case store.EventTypeClusterStatus:
		return "clusterstatus"
	case store.EventTypeClusterDelete:
		return "clusterdelete"
	case "bootstrap_end":
		return "bootstrapend"
	default:
		return eventType
	}
}

func ssePayload(event store.Event) interface{} {
	if event.Type == store.EventTypeClusterStatus {
		clusterID := event.ClusterID
		if status, ok := event.Data.(*store.ClusterStatus); ok && clusterID == "" {
			clusterID = status.ID
		}
		return map[string]interface{}{
			"cluster_id": clusterID,
			"status":     event.Data,
		}
	}
	return event.Data
}
