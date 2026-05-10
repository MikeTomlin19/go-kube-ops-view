package auth

import (
	"kube-ops-view/internal/models"
	"kube-ops-view/internal/store"
)

// Mock store for testing
type mockStore struct {
	tokens map[string]bool
}

func newMockStore() *mockStore {
	return &mockStore{
		tokens: make(map[string]bool),
	}
}

func (m *mockStore) ValidateScreenToken(token string) bool {
	return m.tokens[token]
}

func (m *mockStore) CreateScreenToken() (string, error) {
	token := "test-token-123"
	m.tokens[token] = true
	return token, nil
}

func (m *mockStore) DeleteScreenToken(token string) error {
	delete(m.tokens, token)
	return nil
}

func (m *mockStore) RedeemScreenToken(token, remoteAddr string) error {
	if m.tokens[token] {
		delete(m.tokens, token)
		return nil
	}
	return store.ErrTokenNotFound
}

// Implement other Store interface methods (not used in tests)
func (m *mockStore) GetClusterIDs() []string                                              { return nil }
func (m *mockStore) GetClusterData(clusterID string) (*models.ClusterData, error)         { return nil, nil }
func (m *mockStore) SetClusterData(clusterID string, data *models.ClusterData) error      { return nil }
func (m *mockStore) GetClusterStatus(clusterID string) (*store.ClusterStatus, error)      { return nil, nil }
func (m *mockStore) SetClusterStatus(clusterID string, status *store.ClusterStatus) error { return nil }
func (m *mockStore) DeleteCluster(clusterID string) error                                 { return nil }
func (m *mockStore) PublishEvent(eventType string, data interface{}) error                { return nil }
func (m *mockStore) Subscribe() (<-chan store.Event, error)                               { return nil, nil }
func (m *mockStore) Unsubscribe(ch <-chan store.Event) error                              { return nil }
func (m *mockStore) Close() error                                                         { return nil }
func (m *mockStore) Ping() error                                                          { return nil }
