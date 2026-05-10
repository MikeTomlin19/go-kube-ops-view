package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSessionManager(t *testing.T) {
	// Test with custom TTL
	sm := NewSessionManager(2 * time.Hour)
	assert.NotNil(t, sm)
	assert.Equal(t, 2*time.Hour, sm.sessionTTL)

	// Test with zero TTL (should default to 24 hours)
	sm2 := NewSessionManager(0)
	assert.Equal(t, 24*time.Hour, sm2.sessionTTL)
}

func TestSessionManager_CreateSession(t *testing.T) {
	sm := NewSessionManager(time.Hour)

	user := &User{
		ID:       "user123",
		Email:    "test@example.com",
		Name:     "Test User",
		Username: "testuser",
	}

	session, err := sm.CreateSession(user)

	require.NoError(t, err)
	assert.NotNil(t, session)
	assert.NotEmpty(t, session.ID)
	assert.Equal(t, user.ID, session.UserID)
	assert.Equal(t, user, session.User)
	assert.False(t, session.CreatedAt.IsZero())
	assert.False(t, session.ExpiresAt.IsZero())
	assert.True(t, session.ExpiresAt.After(session.CreatedAt))

	// Verify session is stored
	retrievedSession, err := sm.GetSession(session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.ID, retrievedSession.ID)
}

func TestSessionManager_GetSession(t *testing.T) {
	sm := NewSessionManager(time.Hour)

	user := &User{
		ID:       "user123",
		Username: "testuser",
	}

	// Test non-existent session
	_, err := sm.GetSession("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")

	// Create and retrieve session
	session, err := sm.CreateSession(user)
	require.NoError(t, err)

	// Wait a bit to ensure time difference
	time.Sleep(1 * time.Millisecond)

	retrievedSession, err := sm.GetSession(session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.ID, retrievedSession.ID)
	assert.True(t, retrievedSession.LastSeen.After(session.LastSeen) || retrievedSession.LastSeen.Equal(session.LastSeen))

	// Test expired session
	sm.sessions[session.ID].ExpiresAt = time.Now().Add(-time.Hour)
	_, err = sm.GetSession(session.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session expired")

	// Verify expired session is removed
	_, exists := sm.sessions[session.ID]
	assert.False(t, exists)
}

func TestSessionManager_DeleteSession(t *testing.T) {
	sm := NewSessionManager(time.Hour)

	user := &User{
		ID:       "user123",
		Username: "testuser",
	}

	session, err := sm.CreateSession(user)
	require.NoError(t, err)

	// Verify session exists
	_, err = sm.GetSession(session.ID)
	require.NoError(t, err)

	// Delete session
	err = sm.DeleteSession(session.ID)
	require.NoError(t, err)

	// Verify session is gone
	_, err = sm.GetSession(session.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestSessionManager_RefreshSession(t *testing.T) {
	sm := NewSessionManager(time.Hour)

	user := &User{
		ID:       "user123",
		Username: "testuser",
	}

	session, err := sm.CreateSession(user)
	require.NoError(t, err)

	originalExpiry := session.ExpiresAt

	// Wait a bit to ensure time difference
	time.Sleep(10 * time.Millisecond)

	refreshedSession, err := sm.RefreshSession(session.ID)
	require.NoError(t, err)
	assert.True(t, refreshedSession.ExpiresAt.After(originalExpiry))

	// Test refreshing non-existent session
	_, err = sm.RefreshSession("non-existent")
	assert.Error(t, err)
}

func TestSessionManager_GetUserSessions(t *testing.T) {
	sm := NewSessionManager(time.Hour)

	user1 := &User{ID: "user1", Username: "user1"}
	user2 := &User{ID: "user2", Username: "user2"}

	// Create multiple sessions for user1
	session1, err := sm.CreateSession(user1)
	require.NoError(t, err)
	session2, err := sm.CreateSession(user1)
	require.NoError(t, err)

	// Create session for user2
	session3, err := sm.CreateSession(user2)
	require.NoError(t, err)

	// Get sessions for user1
	user1Sessions := sm.GetUserSessions("user1")
	assert.Len(t, user1Sessions, 2)

	sessionIDs := []string{user1Sessions[0].ID, user1Sessions[1].ID}
	assert.Contains(t, sessionIDs, session1.ID)
	assert.Contains(t, sessionIDs, session2.ID)

	// Get sessions for user2
	user2Sessions := sm.GetUserSessions("user2")
	assert.Len(t, user2Sessions, 1)
	assert.Equal(t, session3.ID, user2Sessions[0].ID)

	// Get sessions for non-existent user
	noSessions := sm.GetUserSessions("non-existent")
	assert.Len(t, noSessions, 0)
}

func TestSessionManager_DeleteUserSessions(t *testing.T) {
	sm := NewSessionManager(time.Hour)

	user1 := &User{ID: "user1", Username: "user1"}
	user2 := &User{ID: "user2", Username: "user2"}

	// Create sessions for both users
	session1, err := sm.CreateSession(user1)
	require.NoError(t, err)
	session2, err := sm.CreateSession(user1)
	require.NoError(t, err)
	session3, err := sm.CreateSession(user2)
	require.NoError(t, err)

	// Delete all sessions for user1
	err = sm.DeleteUserSessions("user1")
	require.NoError(t, err)

	// Verify user1 sessions are gone
	user1Sessions := sm.GetUserSessions("user1")
	assert.Len(t, user1Sessions, 0)

	// Verify user2 session still exists
	user2Sessions := sm.GetUserSessions("user2")
	assert.Len(t, user2Sessions, 1)
	assert.Equal(t, session3.ID, user2Sessions[0].ID)

	// Verify individual session access
	_, err = sm.GetSession(session1.ID)
	assert.Error(t, err)
	_, err = sm.GetSession(session2.ID)
	assert.Error(t, err)
	_, err = sm.GetSession(session3.ID)
	assert.NoError(t, err)
}

func TestSessionManager_GetActiveSessions(t *testing.T) {
	sm := NewSessionManager(time.Hour)

	user := &User{ID: "user1", Username: "user1"}

	// Create sessions
	session1, err := sm.CreateSession(user)
	require.NoError(t, err)
	session2, err := sm.CreateSession(user)
	require.NoError(t, err)

	// All sessions should be active
	activeSessions := sm.GetActiveSessions()
	assert.Len(t, activeSessions, 2)

	// Expire one session
	sm.sessions[session1.ID].ExpiresAt = time.Now().Add(-time.Hour)

	// Only one should be active now
	activeSessions = sm.GetActiveSessions()
	assert.Len(t, activeSessions, 1)
	assert.Equal(t, session2.ID, activeSessions[0].ID)
}

func TestSessionManager_GetSessionCount(t *testing.T) {
	sm := NewSessionManager(time.Hour)

	user := &User{ID: "user1", Username: "user1"}

	// Initially no sessions
	assert.Equal(t, 0, sm.GetSessionCount())

	// Create sessions
	session1, err := sm.CreateSession(user)
	require.NoError(t, err)
	assert.Equal(t, 1, sm.GetSessionCount())

	session2, err := sm.CreateSession(user)
	require.NoError(t, err)
	assert.Equal(t, 2, sm.GetSessionCount())

	// Expire one session
	sm.sessions[session1.ID].ExpiresAt = time.Now().Add(-time.Hour)
	assert.Equal(t, 1, sm.GetSessionCount())

	// Delete remaining session
	sm.DeleteSession(session2.ID)
	assert.Equal(t, 0, sm.GetSessionCount())
}

func TestSessionManager_GenerateSessionID(t *testing.T) {
	sm := NewSessionManager(time.Hour)

	id1, err := sm.generateSessionID()
	require.NoError(t, err)
	assert.NotEmpty(t, id1)

	id2, err := sm.generateSessionID()
	require.NoError(t, err)
	assert.NotEmpty(t, id2)

	// IDs should be different
	assert.NotEqual(t, id1, id2)

	// IDs should be base64 encoded (including padding)
	assert.Regexp(t, `^[A-Za-z0-9_-]+=*$`, id1)
	assert.Regexp(t, `^[A-Za-z0-9_-]+=*$`, id2)
}

func TestSessionManager_Close(t *testing.T) {
	sm := NewSessionManager(time.Hour)

	user := &User{ID: "user1", Username: "user1"}

	// Create session
	_, err := sm.CreateSession(user)
	require.NoError(t, err)
	assert.Equal(t, 1, sm.GetSessionCount())

	// Close session manager
	err = sm.Close()
	require.NoError(t, err)

	// All sessions should be cleared
	assert.Equal(t, 0, sm.GetSessionCount())
}
