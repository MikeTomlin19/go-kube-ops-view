package auth

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// SessionManager manages user sessions
type SessionManager struct {
	sessions   map[string]*Session
	mu         sync.RWMutex
	sessionTTL time.Duration
}

// NewSessionManager creates a new session manager
func NewSessionManager(sessionTTL time.Duration) *SessionManager {
	if sessionTTL <= 0 {
		sessionTTL = 24 * time.Hour // Default to 24 hours
	}

	sm := &SessionManager{
		sessions:   make(map[string]*Session),
		sessionTTL: sessionTTL,
	}

	// Start cleanup goroutine
	go sm.cleanupExpiredSessions()

	return sm
}

// CreateSession creates a new session for the user
func (sm *SessionManager) CreateSession(user *User) (*Session, error) {
	sessionID, err := sm.generateSessionID()
	if err != nil {
		return nil, NewAuthError(ErrCodeSessionNotFound, "failed to generate session ID", err)
	}

	now := time.Now()
	session := &Session{
		ID:        sessionID,
		UserID:    user.ID,
		User:      user,
		CreatedAt: now,
		ExpiresAt: now.Add(sm.sessionTTL),
		LastSeen:  now,
	}

	sm.mu.Lock()
	sm.sessions[sessionID] = session
	sm.mu.Unlock()

	return session, nil
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*Session, error) {
	sm.mu.RLock()
	session, exists := sm.sessions[sessionID]
	sm.mu.RUnlock()

	if !exists {
		return nil, NewAuthError(ErrCodeSessionNotFound, "session not found", nil)
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		sm.DeleteSession(sessionID)
		return nil, NewAuthError(ErrCodeSessionExpired, "session expired", nil)
	}

	// Update last seen time
	sm.mu.Lock()
	session.LastSeen = time.Now()
	sm.mu.Unlock()

	return session, nil
}

// DeleteSession removes a session
func (sm *SessionManager) DeleteSession(sessionID string) error {
	sm.mu.Lock()
	delete(sm.sessions, sessionID)
	sm.mu.Unlock()
	return nil
}

// RefreshSession extends the session expiration time
func (sm *SessionManager) RefreshSession(sessionID string) (*Session, error) {
	session, err := sm.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	sm.mu.Lock()
	session.ExpiresAt = time.Now().Add(sm.sessionTTL)
	sm.mu.Unlock()

	return session, nil
}

// GetUserSessions returns all active sessions for a user
func (sm *SessionManager) GetUserSessions(userID string) []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var userSessions []*Session
	for _, session := range sm.sessions {
		if session.UserID == userID && time.Now().Before(session.ExpiresAt) {
			userSessions = append(userSessions, session)
		}
	}

	return userSessions
}

// DeleteUserSessions removes all sessions for a user
func (sm *SessionManager) DeleteUserSessions(userID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for sessionID, session := range sm.sessions {
		if session.UserID == userID {
			delete(sm.sessions, sessionID)
		}
	}

	return nil
}

// GetActiveSessions returns all active sessions
func (sm *SessionManager) GetActiveSessions() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var activeSessions []*Session
	now := time.Now()
	for _, session := range sm.sessions {
		if now.Before(session.ExpiresAt) {
			activeSessions = append(activeSessions, session)
		}
	}

	return activeSessions
}

// GetSessionCount returns the number of active sessions
func (sm *SessionManager) GetSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	count := 0
	now := time.Now()
	for _, session := range sm.sessions {
		if now.Before(session.ExpiresAt) {
			count++
		}
	}

	return count
}

// generateSessionID generates a cryptographically secure session ID
func (sm *SessionManager) generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// cleanupExpiredSessions periodically removes expired sessions
func (sm *SessionManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(1 * time.Hour) // Cleanup every hour
	defer ticker.Stop()

	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()
		for sessionID, session := range sm.sessions {
			if now.After(session.ExpiresAt) {
				delete(sm.sessions, sessionID)
			}
		}
		sm.mu.Unlock()
	}
}

// Close stops the session manager and cleans up resources
func (sm *SessionManager) Close() error {
	sm.mu.Lock()
	sm.sessions = make(map[string]*Session)
	sm.mu.Unlock()
	return nil
}
