package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestMiddleware(enabled, screenTokens bool) (*Middleware, *SessionManager, *mockStore) {
	sessionManager := NewSessionManager(time.Hour)
	mockStore := newMockStore()
	middleware := NewMiddleware(sessionManager, mockStore, enabled, screenTokens)
	return middleware, sessionManager, mockStore
}

func TestNewMiddleware(t *testing.T) {
	sessionManager := NewSessionManager(time.Hour)
	mockStore := newMockStore()

	middleware := NewMiddleware(sessionManager, mockStore, true, true)

	assert.NotNil(t, middleware)
	assert.Equal(t, sessionManager, middleware.sessionManager)
	assert.Equal(t, mockStore, middleware.store)
	assert.True(t, middleware.enabled)
	assert.True(t, middleware.screenTokens)
}

func TestMiddleware_RequireAuth_Disabled(t *testing.T) {
	middleware, _, _ := setupTestMiddleware(false, false)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequireAuth())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMiddleware_RequireAuth_WithValidSession(t *testing.T) {
	middleware, sessionManager, _ := setupTestMiddleware(true, false)

	// Create a test user and session
	user := &User{
		ID:       "user123",
		Username: "testuser",
		Email:    "test@example.com",
	}
	session, err := sessionManager.CreateSession(user)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequireAuth())
	router.GET("/test", func(c *gin.Context) {
		authUser := GetUser(c)
		authSession := GetSession(c)
		c.JSON(http.StatusOK, gin.H{
			"user":    authUser,
			"session": authSession.ID,
		})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: session.ID,
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMiddleware_RequireAuth_WithInvalidSession(t *testing.T) {
	middleware, _, _ := setupTestMiddleware(true, false)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequireAuth())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: "invalid-session-id",
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code) // Redirect to login
	assert.Contains(t, w.Header().Get("Location"), "/auth/login")
}

func TestMiddleware_RequireAuth_WithScreenToken(t *testing.T) {
	middleware, _, mockStore := setupTestMiddleware(true, true)

	// Add a valid screen token
	token, err := mockStore.CreateScreenToken()
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequireAuth())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(ScreenTokenHeader, token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMiddleware_RequireAuth_APIRequest(t *testing.T) {
	middleware, _, _ := setupTestMiddleware(true, false)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequireAuth())
	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "error")
	assert.Contains(t, w.Body.String(), ErrCodeUnauthorized)
}

func TestMiddleware_OptionalAuth(t *testing.T) {
	middleware, sessionManager, _ := setupTestMiddleware(true, false)

	// Create a test user and session
	user := &User{
		ID:       "user123",
		Username: "testuser",
	}
	session, err := sessionManager.CreateSession(user)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.OptionalAuth())
	router.GET("/test", func(c *gin.Context) {
		authUser := GetUser(c)
		authenticated := IsAuthenticated(c)
		response := gin.H{"authenticated": authenticated}
		if authUser != nil {
			response["user"] = authUser.Username
		}
		c.JSON(http.StatusOK, response)
	})

	// Test with valid session
	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: session.ID,
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"authenticated":true`)
	assert.Contains(t, w.Body.String(), `"user":"testuser"`)

	// Test without session
	req2 := httptest.NewRequest("GET", "/test", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Contains(t, w2.Body.String(), `"authenticated":false`)
}

func TestMiddleware_ScreenTokenAuth(t *testing.T) {
	middleware, _, mockStore := setupTestMiddleware(true, true)

	// Add a valid screen token
	token, err := mockStore.CreateScreenToken()
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.ScreenTokenAuth())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Test with valid token in header
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(ScreenTokenHeader, token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Test with valid token in query parameter
	req2 := httptest.NewRequest("GET", "/test?screen_token="+token, nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)

	// Test with invalid token
	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.Header.Set(ScreenTokenHeader, "invalid-token")
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)

	assert.Equal(t, http.StatusUnauthorized, w3.Code)

	// Test without token
	req4 := httptest.NewRequest("GET", "/test", nil)
	w4 := httptest.NewRecorder()
	router.ServeHTTP(w4, req4)

	assert.Equal(t, http.StatusUnauthorized, w4.Code)
}

func TestMiddleware_ScreenTokenAuth_Disabled(t *testing.T) {
	middleware, _, _ := setupTestMiddleware(true, false) // Screen tokens disabled

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.ScreenTokenAuth())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code) // Should pass through when disabled
}

func TestMiddleware_GetSessionFromRequest(t *testing.T) {
	middleware, sessionManager, _ := setupTestMiddleware(true, false)

	user := &User{ID: "user123", Username: "testuser"}
	session, err := sessionManager.CreateSession(user)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)

	// Test with cookie
	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: session.ID,
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	retrievedSession, err := middleware.getSessionFromRequest(c)
	require.NoError(t, err)
	assert.Equal(t, session.ID, retrievedSession.ID)

	// Test with Authorization header
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set(AuthorizationHeader, "Bearer "+session.ID)
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Request = req2

	retrievedSession2, err := middleware.getSessionFromRequest(c2)
	require.NoError(t, err)
	assert.Equal(t, session.ID, retrievedSession2.ID)

	// Test with no session
	req3 := httptest.NewRequest("GET", "/test", nil)
	c3, _ := gin.CreateTestContext(httptest.NewRecorder())
	c3.Request = req3

	_, err = middleware.getSessionFromRequest(c3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no session found")
}

func TestMiddleware_GetScreenToken(t *testing.T) {
	middleware, _, _ := setupTestMiddleware(true, true)

	gin.SetMode(gin.TestMode)

	// Test with header
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(ScreenTokenHeader, "header-token")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	token := middleware.getScreenToken(c)
	assert.Equal(t, "header-token", token)

	// Test with query parameter
	req2 := httptest.NewRequest("GET", "/test?screen_token=query-token", nil)
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Request = req2

	token2 := middleware.getScreenToken(c2)
	assert.Equal(t, "query-token", token2)

	// Test with no token
	req3 := httptest.NewRequest("GET", "/test", nil)
	c3, _ := gin.CreateTestContext(httptest.NewRecorder())
	c3.Request = req3

	token3 := middleware.getScreenToken(c3)
	assert.Empty(t, token3)
}

func TestMiddleware_IsAPIRequest(t *testing.T) {
	middleware, _, _ := setupTestMiddleware(true, false)

	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		path     string
		headers  map[string]string
		expected bool
	}{
		{
			name:     "JSON Content-Type",
			path:     "/test",
			headers:  map[string]string{"Content-Type": "application/json"},
			expected: true,
		},
		{
			name:     "JSON Accept header",
			path:     "/test",
			headers:  map[string]string{"Accept": "application/json"},
			expected: true,
		},
		{
			name:     "API path",
			path:     "/api/test",
			headers:  map[string]string{},
			expected: true,
		},
		{
			name:     "AJAX request",
			path:     "/test",
			headers:  map[string]string{"X-Requested-With": "XMLHttpRequest"},
			expected: true,
		},
		{
			name:     "Regular web request",
			path:     "/test",
			headers:  map[string]string{"Accept": "text/html"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = req

			result := middleware.isAPIRequest(c)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user := &User{
		ID:       "user123",
		Username: "testuser",
	}

	// Test with user in context
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(UserContextKey, user)

	retrievedUser := GetUser(c)
	assert.Equal(t, user, retrievedUser)

	// Test without user in context
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	retrievedUser2 := GetUser(c2)
	assert.Nil(t, retrievedUser2)

	// Test with wrong type in context
	c3, _ := gin.CreateTestContext(httptest.NewRecorder())
	c3.Set(UserContextKey, "not-a-user")
	retrievedUser3 := GetUser(c3)
	assert.Nil(t, retrievedUser3)
}

func TestGetSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	session := &Session{
		ID:     "session123",
		UserID: "user123",
	}

	// Test with session in context
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(SessionContextKey, session)

	retrievedSession := GetSession(c)
	assert.Equal(t, session, retrievedSession)

	// Test without session in context
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	retrievedSession2 := GetSession(c2)
	assert.Nil(t, retrievedSession2)
}

func TestIsAuthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user := &User{
		ID:       "user123",
		Username: "testuser",
	}

	// Test with authenticated user
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(UserContextKey, user)

	assert.True(t, IsAuthenticated(c))

	// Test without user
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.False(t, IsAuthenticated(c2))
}
