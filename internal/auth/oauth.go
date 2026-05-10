package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// OAuthManager handles OAuth2 authentication flows
type OAuthManager struct {
	config      *oauth2.Config
	stateStore  map[string]time.Time // Simple in-memory state store
	httpClient  *http.Client
	userInfoURL string
}

// NewOAuthManager creates a new OAuth manager
func NewOAuthManager(config OAuthConfig) *OAuthManager {
	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  config.AuthorizeURL,
			TokenURL: config.TokenURL,
		},
		RedirectURL: config.RedirectURL,
		Scopes:      strings.Split(config.Scope, " "),
	}

	return &OAuthManager{
		config:     oauth2Config,
		stateStore: make(map[string]time.Time),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetAuthURL generates an OAuth2 authorization URL with state
func (m *OAuthManager) GetAuthURL() (string, string, error) {
	state, err := m.generateState()
	if err != nil {
		return "", "", NewAuthError(ErrCodeOAuthError, "failed to generate state", err)
	}

	// Store state with expiration (10 minutes)
	m.stateStore[state] = time.Now().Add(10 * time.Minute)

	// Clean up expired states
	m.cleanupExpiredStates()

	authURL := m.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
	return authURL, state, nil
}

// ExchangeCodeForToken exchanges authorization code for access token
func (m *OAuthManager) ExchangeCodeForToken(ctx context.Context, code, state string) (*oauth2.Token, error) {
	// Validate state
	if !m.validateState(state) {
		return nil, NewAuthError(ErrCodeOAuthError, "invalid or expired state", nil)
	}

	// Remove used state
	delete(m.stateStore, state)

	// Exchange code for token
	token, err := m.config.Exchange(ctx, code)
	if err != nil {
		return nil, NewAuthError(ErrCodeOAuthError, "failed to exchange code for token", err)
	}

	return token, nil
}

// GetUserInfo retrieves user information using the access token
func (m *OAuthManager) GetUserInfo(ctx context.Context, token *oauth2.Token) (*User, error) {
	if m.userInfoURL == "" {
		// Try to extract user info from ID token if available
		if idToken, ok := token.Extra("id_token").(string); ok {
			return m.parseIDToken(idToken)
		}
		return nil, NewAuthError(ErrCodeOAuthError, "no user info URL configured and no ID token available", nil)
	}

	client := m.config.Client(ctx, token)
	resp, err := client.Get(m.userInfoURL)
	if err != nil {
		return nil, NewAuthError(ErrCodeOAuthError, "failed to get user info", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, NewAuthError(ErrCodeOAuthError, fmt.Sprintf("user info request failed with status %d", resp.StatusCode), nil)
	}

	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, NewAuthError(ErrCodeOAuthError, "failed to decode user info", err)
	}

	return m.mapUserInfo(userInfo), nil
}

// SetUserInfoURL sets the URL for retrieving user information
func (m *OAuthManager) SetUserInfoURL(url string) {
	m.userInfoURL = url
}

// generateState generates a cryptographically secure random state string
func (m *OAuthManager) generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// validateState validates the OAuth2 state parameter
func (m *OAuthManager) validateState(state string) bool {
	expiry, exists := m.stateStore[state]
	if !exists {
		return false
	}
	return time.Now().Before(expiry)
}

// cleanupExpiredStates removes expired state entries
func (m *OAuthManager) cleanupExpiredStates() {
	now := time.Now()
	for state, expiry := range m.stateStore {
		if now.After(expiry) {
			delete(m.stateStore, state)
		}
	}
}

// parseIDToken parses user information from JWT ID token (basic implementation)
func (m *OAuthManager) parseIDToken(idToken string) (*User, error) {
	// This is a simplified implementation - in production, you should properly validate JWT signatures
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, NewAuthError(ErrCodeOAuthError, "invalid ID token format", nil)
	}

	// Decode payload (second part)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, NewAuthError(ErrCodeOAuthError, "failed to decode ID token payload", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, NewAuthError(ErrCodeOAuthError, "failed to parse ID token claims", err)
	}

	return m.mapUserInfo(claims), nil
}

// mapUserInfo maps OAuth2 user info to internal User struct
func (m *OAuthManager) mapUserInfo(userInfo map[string]interface{}) *User {
	user := &User{
		Claims: userInfo,
	}

	// Extract standard claims
	if id, ok := userInfo["sub"].(string); ok {
		user.ID = id
	} else if id, ok := userInfo["id"].(string); ok {
		user.ID = id
	}

	if email, ok := userInfo["email"].(string); ok {
		user.Email = email
	}

	if name, ok := userInfo["name"].(string); ok {
		user.Name = name
	}

	if username, ok := userInfo["preferred_username"].(string); ok {
		user.Username = username
	} else if username, ok := userInfo["username"].(string); ok {
		user.Username = username
	}

	// Extract groups if available
	if groups, ok := userInfo["groups"].([]interface{}); ok {
		user.Groups = make([]string, len(groups))
		for i, group := range groups {
			if groupStr, ok := group.(string); ok {
				user.Groups[i] = groupStr
			}
		}
	}

	// Use ID as fallback for username if not available
	if user.Username == "" {
		if user.Email != "" {
			user.Username = user.Email
		} else {
			user.Username = user.ID
		}
	}

	// Use username as fallback for name if not available
	if user.Name == "" {
		user.Name = user.Username
	}

	return user
}

// ValidateToken validates an OAuth2 token
func (m *OAuthManager) ValidateToken(ctx context.Context, token *oauth2.Token) error {
	if token == nil {
		return NewAuthError(ErrCodeTokenInvalid, "token is nil", nil)
	}

	if !token.Valid() {
		return NewAuthError(ErrCodeTokenInvalid, "token is expired or invalid", nil)
	}

	return nil
}
