package store

import (
	"time"

	"kube-ops-view/internal/models"
)

// Store defines the interface for data storage operations
type Store interface {
	// Cluster operations
	GetClusterIDs() []string
	GetClusterData(clusterID string) (*models.ClusterData, error)
	SetClusterData(clusterID string, data *models.ClusterData) error
	GetClusterStatus(clusterID string) (*ClusterStatus, error)
	SetClusterStatus(clusterID string, status *ClusterStatus) error
	DeleteCluster(clusterID string) error

	// Event operations
	PublishEvent(eventType string, data interface{}) error
	Subscribe() (<-chan Event, error)
	Unsubscribe(ch <-chan Event) error

	// Screen token operations
	CreateScreenToken() (string, error)
	RedeemScreenToken(token, remoteAddr string) error
	ValidateScreenToken(token string) bool
	DeleteScreenToken(token string) error

	// Health and cleanup
	Close() error
	Ping() error
}

// ClusterStatus represents the status of a cluster
type ClusterStatus struct {
	ID           string    `json:"id"`
	Available    bool      `json:"available"`
	LastSeen     time.Time `json:"last_seen"`
	ErrorMessage string    `json:"error_message,omitempty"`
	APIServerURL string    `json:"api_server_url"`
}

// Event represents a system event that can be published/subscribed to
type Event struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
	ClusterID string      `json:"cluster_id,omitempty"`
}

// ScreenToken represents an authentication token for display screens
type ScreenToken struct {
	Token      string     `json:"token"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	UsedAt     *time.Time `json:"used_at,omitempty"`
	RemoteAddr string     `json:"remote_addr,omitempty"`
}

// Event types
const (
	EventTypeClusterUpdate = "cluster_update"
	EventTypeClusterDelta  = "cluster_delta"
	EventTypeClusterStatus = "cluster_status"
	EventTypeClusterDelete = "cluster_delete"
)

// Error types
const (
	ErrClusterNotFound  = StoreError("cluster not found")
	ErrTokenNotFound    = StoreError("token not found")
	ErrTokenExpired     = StoreError("token expired")
	ErrTokenAlreadyUsed = StoreError("token already used")
	ErrInvalidToken     = StoreError("invalid token")
	ErrConnectionFailed = StoreError("connection failed")
	ErrOperationFailed  = StoreError("operation failed")
)

// StoreError represents a store-specific error
type StoreError string

func (e StoreError) Error() string {
	return string(e)
}
