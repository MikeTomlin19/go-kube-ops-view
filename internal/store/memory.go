package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"kube-ops-view/internal/models"
)

// MemoryStore implements the Store interface using in-memory storage
type MemoryStore struct {
	clusters map[string]*models.ClusterData
	statuses map[string]*ClusterStatus
	tokens   map[string]*ScreenToken
	events   []chan Event
	mu       sync.RWMutex
	eventsMu sync.RWMutex
	tokenTTL time.Duration
	closed   bool
}

// NewMemoryStore creates a new in-memory store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		clusters: make(map[string]*models.ClusterData),
		statuses: make(map[string]*ClusterStatus),
		tokens:   make(map[string]*ScreenToken),
		events:   make([]chan Event, 0),
		tokenTTL: 24 * time.Hour, // Default token TTL
	}
}

// NewMemoryStoreWithTTL creates a new in-memory store with custom token TTL
func NewMemoryStoreWithTTL(tokenTTL time.Duration) *MemoryStore {
	store := NewMemoryStore()
	store.tokenTTL = tokenTTL
	return store
}

// GetClusterIDs returns all cluster IDs
func (m *MemoryStore) GetClusterIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.clusters))
	for id := range m.clusters {
		ids = append(ids, id)
	}
	return ids
}

// GetClusterData retrieves cluster data by ID
func (m *MemoryStore) GetClusterData(clusterID string) (*models.ClusterData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, exists := m.clusters[clusterID]
	if !exists {
		return nil, ErrClusterNotFound
	}

	// Return a deep copy to prevent external modifications
	return data.Clone(), nil
}

// SetClusterData stores cluster data
func (m *MemoryStore) SetClusterData(clusterID string, data *models.ClusterData) error {
	if data == nil {
		return fmt.Errorf("cluster data cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Store a deep copy to prevent external modifications
	m.clusters[clusterID] = data.Clone()
	return nil
}

// GetClusterStatus retrieves cluster status by ID
func (m *MemoryStore) GetClusterStatus(clusterID string) (*ClusterStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status, exists := m.statuses[clusterID]
	if !exists {
		return nil, ErrClusterNotFound
	}

	// Return a copy
	statusCopy := *status
	return &statusCopy, nil
}

// SetClusterStatus stores cluster status
func (m *MemoryStore) SetClusterStatus(clusterID string, status *ClusterStatus) error {
	if status == nil {
		return fmt.Errorf("cluster status cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Store a copy
	statusCopy := *status
	m.statuses[clusterID] = &statusCopy
	return nil
}

// DeleteCluster removes cluster data and status
func (m *MemoryStore) DeleteCluster(clusterID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.clusters, clusterID)
	delete(m.statuses, clusterID)
	return nil
}

// PublishEvent publishes an event to all subscribers
func (m *MemoryStore) PublishEvent(eventType string, data interface{}) error {
	m.eventsMu.RLock()
	defer m.eventsMu.RUnlock()

	if m.closed {
		return ErrOperationFailed
	}

	event := Event{
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now(),
	}

	// Extract cluster ID if data is cluster-related
	if clusterData, ok := data.(*models.ClusterData); ok {
		event.ClusterID = clusterData.ID
	} else if clusterStatus, ok := data.(*ClusterStatus); ok {
		event.ClusterID = clusterStatus.ID
	}

	// Send to all subscribers (non-blocking)
	for _, ch := range m.events {
		select {
		case ch <- event:
		default:
			// Channel is full, skip this subscriber
		}
	}

	return nil
}

// Subscribe creates a new event subscription channel
func (m *MemoryStore) Subscribe() (<-chan Event, error) {
	m.eventsMu.Lock()
	defer m.eventsMu.Unlock()

	if m.closed {
		return nil, ErrOperationFailed
	}

	// Create buffered channel to prevent blocking
	ch := make(chan Event, 100)
	m.events = append(m.events, ch)
	return ch, nil
}

// Unsubscribe removes an event subscription channel
func (m *MemoryStore) Unsubscribe(ch <-chan Event) error {
	m.eventsMu.Lock()
	defer m.eventsMu.Unlock()

	// Find and remove the channel
	for i, eventCh := range m.events {
		if eventCh == ch {
			// Close the channel
			close(eventCh)
			// Remove from slice
			m.events = append(m.events[:i], m.events[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("subscription not found")
}

// CreateScreenToken creates a new screen token
func (m *MemoryStore) CreateScreenToken() (string, error) {
	// Generate random token
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	token := hex.EncodeToString(bytes)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Clean up expired tokens
	m.cleanupExpiredTokens()

	// Create new token
	screenToken := &ScreenToken{
		Token:     token,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(m.tokenTTL),
	}

	m.tokens[token] = screenToken
	return token, nil
}

// RedeemScreenToken redeems a screen token for authentication
func (m *MemoryStore) RedeemScreenToken(token, remoteAddr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	screenToken, exists := m.tokens[token]
	if !exists {
		return ErrTokenNotFound
	}

	// Check if token is expired
	if time.Now().After(screenToken.ExpiresAt) {
		delete(m.tokens, token)
		return ErrTokenExpired
	}

	// Check if token is already used
	if screenToken.UsedAt != nil {
		return ErrTokenAlreadyUsed
	}

	// Mark token as used
	now := time.Now()
	screenToken.UsedAt = &now
	screenToken.RemoteAddr = remoteAddr

	return nil
}

// ValidateScreenToken validates a screen token without redeeming it
func (m *MemoryStore) ValidateScreenToken(token string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	screenToken, exists := m.tokens[token]
	if !exists {
		return false
	}

	// Check if token is expired
	if time.Now().After(screenToken.ExpiresAt) {
		return false
	}

	return true
}

// DeleteScreenToken removes a screen token
func (m *MemoryStore) DeleteScreenToken(token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.tokens, token)
	return nil
}

// Close closes the store and cleans up resources
func (m *MemoryStore) Close() error {
	m.eventsMu.Lock()
	defer m.eventsMu.Unlock()

	m.closed = true

	// Close all event channels
	for _, ch := range m.events {
		close(ch)
	}
	m.events = nil

	return nil
}

// Ping checks if the store is available
func (m *MemoryStore) Ping() error {
	if m.closed {
		return ErrConnectionFailed
	}
	return nil
}

// cleanupExpiredTokens removes expired tokens (must be called with lock held)
func (m *MemoryStore) cleanupExpiredTokens() {
	now := time.Now()
	for token, screenToken := range m.tokens {
		if now.After(screenToken.ExpiresAt) {
			delete(m.tokens, token)
		}
	}
}

// GetTokenCount returns the number of active tokens (for testing)
func (m *MemoryStore) GetTokenCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.cleanupExpiredTokens()
	return len(m.tokens)
}
