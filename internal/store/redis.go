package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"kube-ops-view/internal/models"

	"github.com/redis/go-redis/v9"
)

// RedisStore implements the Store interface using Redis
type RedisStore struct {
	client   redis.UniversalClient
	pubsub   *redis.PubSub
	events   []chan Event
	eventsMu sync.RWMutex
	tokenTTL time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
	closed   bool
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	TokenTTL time.Duration
}

// NewRedisStore creates a new Redis-backed store
func NewRedisStore(config RedisConfig) (*RedisStore, error) {
	ctx, cancel := context.WithCancel(context.Background())

	client := redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
		DB:       config.DB,
	})

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	tokenTTL := config.TokenTTL
	if tokenTTL == 0 {
		tokenTTL = 24 * time.Hour // Default token TTL
	}

	store := &RedisStore{
		client:   client,
		events:   make([]chan Event, 0),
		tokenTTL: tokenTTL,
		ctx:      ctx,
		cancel:   cancel,
	}

	// Initialize pub/sub
	store.pubsub = client.Subscribe(ctx, "kube-ops-view:events")

	// Start event listener
	go store.eventListener()

	return store, nil
}

// NewRedisClusterStore creates a new Redis Cluster-backed store
func NewRedisClusterStore(addrs []string, password string, tokenTTL time.Duration) (*RedisStore, error) {
	ctx, cancel := context.WithCancel(context.Background())

	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    addrs,
		Password: password,
	})

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to Redis cluster: %w", err)
	}

	if tokenTTL == 0 {
		tokenTTL = 24 * time.Hour // Default token TTL
	}

	store := &RedisStore{
		client:   client,
		events:   make([]chan Event, 0),
		tokenTTL: tokenTTL,
		ctx:      ctx,
		cancel:   cancel,
	}

	// Initialize pub/sub
	store.pubsub = client.Subscribe(ctx, "kube-ops-view:events")

	// Start event listener
	go store.eventListener()

	return store, nil
}

// Redis key prefixes
const (
	clusterDataPrefix   = "kube-ops-view:cluster:data:"
	clusterStatusPrefix = "kube-ops-view:cluster:status:"
	tokenPrefix         = "kube-ops-view:token:"
	clusterSetKey       = "kube-ops-view:clusters"
	eventChannel        = "kube-ops-view:events"
)

// GetClusterIDs returns all cluster IDs
func (r *RedisStore) GetClusterIDs() []string {
	members, err := r.client.SMembers(r.ctx, clusterSetKey).Result()
	if err != nil {
		return []string{}
	}
	return members
}

// GetClusterData retrieves cluster data by ID
func (r *RedisStore) GetClusterData(clusterID string) (*models.ClusterData, error) {
	key := clusterDataPrefix + clusterID
	data, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrClusterNotFound
		}
		return nil, fmt.Errorf("failed to get cluster data: %w", err)
	}

	var clusterData models.ClusterData
	if err := json.Unmarshal([]byte(data), &clusterData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster data: %w", err)
	}

	return &clusterData, nil
}

// SetClusterData stores cluster data
func (r *RedisStore) SetClusterData(clusterID string, data *models.ClusterData) error {
	if data == nil {
		return fmt.Errorf("cluster data cannot be nil")
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal cluster data: %w", err)
	}

	key := clusterDataPrefix + clusterID

	// Use pipeline for atomic operations
	pipe := r.client.Pipeline()
	pipe.Set(r.ctx, key, jsonData, 0)
	pipe.SAdd(r.ctx, clusterSetKey, clusterID)

	if _, err := pipe.Exec(r.ctx); err != nil {
		return fmt.Errorf("failed to set cluster data: %w", err)
	}

	return nil
}

// GetClusterStatus retrieves cluster status by ID
func (r *RedisStore) GetClusterStatus(clusterID string) (*ClusterStatus, error) {
	key := clusterStatusPrefix + clusterID
	data, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrClusterNotFound
		}
		return nil, fmt.Errorf("failed to get cluster status: %w", err)
	}

	var status ClusterStatus
	if err := json.Unmarshal([]byte(data), &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster status: %w", err)
	}

	return &status, nil
}

// SetClusterStatus stores cluster status
func (r *RedisStore) SetClusterStatus(clusterID string, status *ClusterStatus) error {
	if status == nil {
		return fmt.Errorf("cluster status cannot be nil")
	}

	jsonData, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to marshal cluster status: %w", err)
	}

	key := clusterStatusPrefix + clusterID
	if err := r.client.Set(r.ctx, key, jsonData, 0).Err(); err != nil {
		return fmt.Errorf("failed to set cluster status: %w", err)
	}

	return nil
}

// DeleteCluster removes cluster data and status
func (r *RedisStore) DeleteCluster(clusterID string) error {
	pipe := r.client.Pipeline()
	pipe.Del(r.ctx, clusterDataPrefix+clusterID)
	pipe.Del(r.ctx, clusterStatusPrefix+clusterID)
	pipe.SRem(r.ctx, clusterSetKey, clusterID)

	if _, err := pipe.Exec(r.ctx); err != nil {
		return fmt.Errorf("failed to delete cluster: %w", err)
	}

	return nil
}

// PublishEvent publishes an event to all subscribers
func (r *RedisStore) PublishEvent(eventType string, data interface{}) error {
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

	jsonData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := r.client.Publish(r.ctx, eventChannel, jsonData).Err(); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	return nil
}

// Subscribe creates a new event subscription channel
func (r *RedisStore) Subscribe() (<-chan Event, error) {
	r.eventsMu.Lock()
	defer r.eventsMu.Unlock()

	if r.closed {
		return nil, ErrOperationFailed
	}

	// Create buffered channel to prevent blocking
	ch := make(chan Event, 100)
	r.events = append(r.events, ch)
	return ch, nil
}

// Unsubscribe removes an event subscription channel
func (r *RedisStore) Unsubscribe(ch <-chan Event) error {
	r.eventsMu.Lock()
	defer r.eventsMu.Unlock()

	// Find and remove the channel
	for i, eventCh := range r.events {
		if eventCh == ch {
			// Close the channel
			close(eventCh)
			// Remove from slice
			r.events = append(r.events[:i], r.events[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("subscription not found")
}

// CreateScreenToken creates a new screen token
func (r *RedisStore) CreateScreenToken() (string, error) {
	// Generate random token
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	token := hex.EncodeToString(bytes)

	// Create token data
	screenToken := &ScreenToken{
		Token:     token,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(r.tokenTTL),
	}

	jsonData, err := json.Marshal(screenToken)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token: %w", err)
	}

	key := tokenPrefix + token
	if err := r.client.Set(r.ctx, key, jsonData, r.tokenTTL).Err(); err != nil {
		return "", fmt.Errorf("failed to store token: %w", err)
	}

	return token, nil
}

// RedeemScreenToken redeems a screen token for authentication
func (r *RedisStore) RedeemScreenToken(token, remoteAddr string) error {
	key := tokenPrefix + token

	// Get token data
	data, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return ErrTokenNotFound
		}
		return fmt.Errorf("failed to get token: %w", err)
	}

	var screenToken ScreenToken
	if err := json.Unmarshal([]byte(data), &screenToken); err != nil {
		return fmt.Errorf("failed to unmarshal token: %w", err)
	}

	// Check if token is expired
	if time.Now().After(screenToken.ExpiresAt) {
		r.client.Del(r.ctx, key)
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

	jsonData, err := json.Marshal(screenToken)
	if err != nil {
		return fmt.Errorf("failed to marshal updated token: %w", err)
	}

	// Update token with remaining TTL
	ttl := r.client.TTL(r.ctx, key).Val()
	if err := r.client.Set(r.ctx, key, jsonData, ttl).Err(); err != nil {
		return fmt.Errorf("failed to update token: %w", err)
	}

	return nil
}

// ValidateScreenToken validates a screen token without redeeming it
func (r *RedisStore) ValidateScreenToken(token string) bool {
	key := tokenPrefix + token

	data, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		return false
	}

	var screenToken ScreenToken
	if err := json.Unmarshal([]byte(data), &screenToken); err != nil {
		return false
	}

	// Check if token is expired
	if time.Now().After(screenToken.ExpiresAt) {
		return false
	}

	return true
}

// DeleteScreenToken removes a screen token
func (r *RedisStore) DeleteScreenToken(token string) error {
	key := tokenPrefix + token
	if err := r.client.Del(r.ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete token: %w", err)
	}
	return nil
}

// Close closes the store and cleans up resources
func (r *RedisStore) Close() error {
	r.eventsMu.Lock()
	defer r.eventsMu.Unlock()

	r.closed = true

	// Close all event channels
	for _, ch := range r.events {
		close(ch)
	}
	r.events = nil

	// Close pub/sub
	if r.pubsub != nil {
		r.pubsub.Close()
	}

	// Cancel context
	r.cancel()

	// Close Redis client
	return r.client.Close()
}

// Ping checks if the store is available
func (r *RedisStore) Ping() error {
	if r.closed {
		return ErrConnectionFailed
	}
	return r.client.Ping(r.ctx).Err()
}

// eventListener listens for Redis pub/sub events and forwards them to subscribers
func (r *RedisStore) eventListener() {
	defer func() {
		if r.pubsub != nil {
			r.pubsub.Close()
		}
	}()

	ch := r.pubsub.Channel()
	for {
		select {
		case msg := <-ch:
			if msg == nil {
				return
			}

			var event Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				continue // Skip malformed events
			}

			r.eventsMu.RLock()
			// Send to all subscribers (non-blocking)
			for _, eventCh := range r.events {
				select {
				case eventCh <- event:
				default:
					// Channel is full, skip this subscriber
				}
			}
			r.eventsMu.RUnlock()

		case <-r.ctx.Done():
			return
		}
	}
}

// GetTokenCount returns the number of active tokens (for testing)
func (r *RedisStore) GetTokenCount() int {
	pattern := tokenPrefix + "*"
	keys, err := r.client.Keys(r.ctx, pattern).Result()
	if err != nil {
		return 0
	}
	return len(keys)
}

// FlushAll removes all data (for testing only)
func (r *RedisStore) FlushAll() error {
	return r.client.FlushAll(r.ctx).Err()
}
