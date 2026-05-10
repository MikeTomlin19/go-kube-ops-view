package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestNewOAuthManager(t *testing.T) {
	config := OAuthConfig{
		AuthorizeURL: "https://example.com/auth",
		TokenURL:     "https://example.com/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scope:        "openid profile",
		RedirectURL:  "https://app.example.com/callback",
	}

	manager := NewOAuthManager(config)

	assert.NotNil(t, manager)
	assert.Equal(t, config.ClientID, manager.config.ClientID)
	assert.Equal(t, config.ClientSecret, manager.config.ClientSecret)
	assert.Equal(t, config.AuthorizeURL, manager.config.Endpoint.AuthURL)
	assert.Equal(t, config.TokenURL, manager.config.Endpoint.TokenURL)
	assert.Equal(t, config.RedirectURL, manager.config.RedirectURL)
	assert.Equal(t, []string{"openid", "profile"}, manager.config.Scopes)
}

func TestOAuthManager_GetAuthURL(t *testing.T) {
	config := OAuthConfig{
		AuthorizeURL: "https://example.com/auth",
		TokenURL:     "https://example.com/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scope:        "openid",
		RedirectURL:  "https://app.example.com/callback",
	}

	manager := NewOAuthManager(config)

	authURL, state, err := manager.GetAuthURL()

	require.NoError(t, err)
	assert.NotEmpty(t, authURL)
	assert.NotEmpty(t, state)
	assert.Contains(t, authURL, "https://example.com/auth")
	assert.Contains(t, authURL, "client_id=test-client")
	assert.Contains(t, authURL, "state=")

	// Verify state is stored
	assert.True(t, manager.validateState(state))
}

func TestOAuthManager_ValidateState(t *testing.T) {
	manager := NewOAuthManager(OAuthConfig{})

	// Test invalid state
	assert.False(t, manager.validateState("invalid-state"))

	// Test valid state
	_, state, err := manager.GetAuthURL()
	require.NoError(t, err)
	assert.True(t, manager.validateState(state))

	// Test expired state
	manager.stateStore[state] = time.Now().Add(-1 * time.Hour)
	assert.False(t, manager.validateState(state))
}

func TestOAuthManager_ExchangeCodeForToken(t *testing.T) {
	// Create mock OAuth server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "test-access-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		}
	}))
	defer server.Close()

	config := OAuthConfig{
		AuthorizeURL: server.URL + "/auth",
		TokenURL:     server.URL + "/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scope:        "openid",
		RedirectURL:  "https://app.example.com/callback",
	}

	manager := NewOAuthManager(config)

	// Generate state first
	_, state, err := manager.GetAuthURL()
	require.NoError(t, err)

	// Test successful token exchange
	token, err := manager.ExchangeCodeForToken(context.Background(), "test-code", state)

	require.NoError(t, err)
	assert.NotNil(t, token)
	assert.Equal(t, "test-access-token", token.AccessToken)
	assert.Equal(t, "Bearer", token.TokenType)

	// Test invalid state
	_, err = manager.ExchangeCodeForToken(context.Background(), "test-code", "invalid-state")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired state")
}

func TestOAuthManager_GetUserInfo(t *testing.T) {
	// Create mock user info server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/userinfo" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"sub":                "user123",
				"email":              "test@example.com",
				"name":               "Test User",
				"preferred_username": "testuser",
				"groups":             []string{"admin", "users"},
			})
		}
	}))
	defer server.Close()

	manager := NewOAuthManager(OAuthConfig{})
	manager.SetUserInfoURL(server.URL + "/userinfo")

	token := &oauth2.Token{
		AccessToken: "test-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour),
	}

	user, err := manager.GetUserInfo(context.Background(), token)

	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "user123", user.ID)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, []string{"admin", "users"}, user.Groups)
}

func TestOAuthManager_ParseIDToken(t *testing.T) {
	manager := NewOAuthManager(OAuthConfig{})

	// Create a simple JWT-like token (not properly signed, just for testing structure)
	// Base64 encode parts
	headerB64 := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"
	payloadB64 := "eyJzdWIiOiJ1c2VyMTIzIiwiZW1haWwiOiJ0ZXN0QGV4YW1wbGUuY29tIiwibmFtZSI6IlRlc3QgVXNlciIsInByZWZlcnJlZF91c2VybmFtZSI6InRlc3R1c2VyIn0"
	signatureB64 := "c2lnbmF0dXJl"

	idToken := headerB64 + "." + payloadB64 + "." + signatureB64

	user, err := manager.parseIDToken(idToken)

	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "user123", user.ID)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, "testuser", user.Username)

	// Test invalid token format
	_, err = manager.parseIDToken("invalid-token")
	assert.Error(t, err)
}

func TestOAuthManager_MapUserInfo(t *testing.T) {
	manager := NewOAuthManager(OAuthConfig{})

	userInfo := map[string]interface{}{
		"sub":                "user123",
		"email":              "test@example.com",
		"name":               "Test User",
		"preferred_username": "testuser",
		"groups":             []interface{}{"admin", "users"},
	}

	user := manager.mapUserInfo(userInfo)

	assert.NotNil(t, user)
	assert.Equal(t, "user123", user.ID)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, []string{"admin", "users"}, user.Groups)

	// Test with minimal info
	minimalInfo := map[string]interface{}{
		"sub": "user456",
	}

	user2 := manager.mapUserInfo(minimalInfo)
	assert.Equal(t, "user456", user2.ID)
	assert.Equal(t, "user456", user2.Username) // Should fallback to ID
	assert.Equal(t, "user456", user2.Name)     // Should fallback to username
}

func TestOAuthManager_ValidateToken(t *testing.T) {
	manager := NewOAuthManager(OAuthConfig{})

	// Test nil token
	err := manager.ValidateToken(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token is nil")

	// Test expired token
	expiredToken := &oauth2.Token{
		AccessToken: "test-token",
		Expiry:      time.Now().Add(-time.Hour),
	}
	err = manager.ValidateToken(context.Background(), expiredToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired or invalid")

	// Test valid token
	validToken := &oauth2.Token{
		AccessToken: "test-token",
		Expiry:      time.Now().Add(time.Hour),
	}
	err = manager.ValidateToken(context.Background(), validToken)
	assert.NoError(t, err)
}

func TestOAuthManager_CleanupExpiredStates(t *testing.T) {
	manager := NewOAuthManager(OAuthConfig{})

	// Add some states
	manager.stateStore["valid"] = time.Now().Add(time.Hour)
	manager.stateStore["expired"] = time.Now().Add(-time.Hour)

	manager.cleanupExpiredStates()

	// Valid state should remain
	assert.True(t, manager.validateState("valid"))
	// Expired state should be removed
	assert.False(t, manager.validateState("expired"))
}
