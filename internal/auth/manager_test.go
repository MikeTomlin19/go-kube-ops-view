package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kube-ops-view/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled:      true,
		AuthorizeURL: "https://example.com/auth",
		TokenURL:     "https://example.com/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scope:        "openid",
		ScreenTokens: true,
	}

	mockStore := newMockStore()
	baseURL := "https://app.example.com"

	manager, err := NewManager(cfg, mockStore, baseURL)

	require.NoError(t, err)
	assert.NotNil(t, manager)
	assert.Equal(t, cfg, manager.config)
	assert.NotNil(t, manager.oauthManager)
	assert.NotNil(t, manager.sessionManager)
	assert.NotNil(t, manager.middleware)

	// Test with nil config
	_, err = NewManager(nil, mockStore, baseURL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "auth config is required")
}

func TestNewManager_DisabledAuth(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled: false,
	}

	mockStore := newMockStore()
	baseURL := "https://app.example.com"

	manager, err := NewManager(cfg, mockStore, baseURL)

	require.NoError(t, err)
	assert.NotNil(t, manager)
	assert.Nil(t, manager.oauthManager) // Should be nil when disabled
}

func TestManager_SetupRoutes(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled:      true,
		AuthorizeURL: "https://example.com/auth",
		TokenURL:     "https://example.com/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scope:        "openid",
		ScreenTokens: true,
	}

	mockStore := newMockStore()
	manager, err := NewManager(cfg, mockStore, "https://app.example.com")
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	manager.SetupRoutes(router, "/")

	// Test that routes are registered by making requests
	routes := []string{
		"/auth/login",
		"/auth/callback",
		"/auth/logout",
		"/auth/status",
	}

	for _, route := range routes {
		req := httptest.NewRequest("GET", route, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should not return 404 (route exists)
		assert.NotEqual(t, http.StatusNotFound, w.Code, "Route %s should exist", route)
	}
}

func TestManager_HandleLogin_Disabled(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled: false,
	}

	mockStore := newMockStore()
	manager, err := NewManager(cfg, mockStore, "https://app.example.com")
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	manager.SetupRoutes(router, "/")

	req := httptest.NewRequest("GET", "/auth/login", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
	assert.Contains(t, w.Body.String(), "authentication is not enabled")
}

func TestManager_HandleLogin_Enabled(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled:      true,
		AuthorizeURL: "https://example.com/auth",
		TokenURL:     "https://example.com/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scope:        "openid",
	}

	mockStore := newMockStore()
	manager, err := NewManager(cfg, mockStore, "https://app.example.com")
	require.NoError(t, err)
	require.NotNil(t, manager.oauthManager, "OAuth manager should not be nil when auth is enabled")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	manager.SetupRoutes(router, "/")

	req := httptest.NewRequest("GET", "/auth/login?return_url=/dashboard", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	t.Logf("Response status: %d", w.Code)
	t.Logf("Response body: %s", w.Body.String())
	t.Logf("Response headers: %v", w.Header())

	// Check Set-Cookie header directly
	setCookieHeaders := w.Header()["Set-Cookie"]
	t.Logf("Set-Cookie headers: %v", setCookieHeaders)

	// Check cookies
	responseCookies := w.Result().Cookies()
	t.Logf("Number of cookies: %d", len(responseCookies))
	for i, cookie := range responseCookies {
		t.Logf("Cookie %d: %s = %s", i, cookie.Name, cookie.Value)
	}

	if w.Code != http.StatusFound {
		t.Fatalf("Expected status %d, got %d. Body: %s", http.StatusFound, w.Code, w.Body.String())
	}

	location := w.Header().Get("Location")
	if location == "" {
		t.Fatal("No Location header in redirect response")
	}

	assert.Contains(t, location, "https://example.com/auth")
	assert.Contains(t, location, "client_id=test-client")

	// Extract state from URL to check cookie name
	stateStart := strings.Index(location, "state=")
	if stateStart == -1 {
		t.Fatal("No state parameter in auth URL")
	}
	stateStart += 6 // len("state=")
	stateEnd := strings.Index(location[stateStart:], "&")
	if stateEnd == -1 {
		stateEnd = len(location)
	} else {
		stateEnd += stateStart
	}
	urlEncodedState := location[stateStart:stateEnd]
	t.Logf("Extracted URL-encoded state: %s", urlEncodedState)

	expectedCookieName := "oauth_return_url_" + urlEncodedState
	t.Logf("Expected cookie name: %s", expectedCookieName)

	// Check that return URL cookie is set
	cookies := w.Result().Cookies()
	var returnURLCookie *http.Cookie
	for _, cookie := range cookies {
		t.Logf("Checking cookie: %s", cookie.Name)
		if cookie.Name == expectedCookieName {
			returnURLCookie = cookie
			break
		}
	}

	if returnURLCookie == nil {
		t.Fatal("Return URL cookie not found")
	}

	assert.Equal(t, "%2Fdashboard", returnURLCookie.Value)
}

func TestManager_HandleCallback_Disabled(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled: false,
	}

	mockStore := newMockStore()
	manager, err := NewManager(cfg, mockStore, "https://app.example.com")
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	manager.SetupRoutes(router, "/")

	req := httptest.NewRequest("GET", "/auth/callback?code=test&state=test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestManager_HandleCallback_MissingParams(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled:      true,
		AuthorizeURL: "https://example.com/auth",
		TokenURL:     "https://example.com/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	}

	mockStore := newMockStore()
	manager, err := NewManager(cfg, mockStore, "https://app.example.com")
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	manager.SetupRoutes(router, "/")

	// Test missing code
	req := httptest.NewRequest("GET", "/auth/callback?state=test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "missing code or state")

	// Test missing state
	req2 := httptest.NewRequest("GET", "/auth/callback?code=test", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusBadRequest, w2.Code)
	assert.Contains(t, w2.Body.String(), "missing code or state")

	// Test OAuth error
	req3 := httptest.NewRequest("GET", "/auth/callback?error=access_denied", nil)
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)

	assert.Equal(t, http.StatusBadRequest, w3.Code)
	assert.Contains(t, w3.Body.String(), "OAuth error: access_denied")
}

func TestManager_HandleLogout_Disabled(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled: false,
	}

	mockStore := newMockStore()
	manager, err := NewManager(cfg, mockStore, "https://app.example.com")
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	manager.SetupRoutes(router, "/")

	req := httptest.NewRequest("POST", "/auth/logout", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "logged out")
}

func TestManager_HandleLogout_WithSession(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled:      true,
		AuthorizeURL: "https://example.com/auth",
		TokenURL:     "https://example.com/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	}

	mockStore := newMockStore()
	manager, err := NewManager(cfg, mockStore, "https://app.example.com")
	require.NoError(t, err)

	// Create a session
	user := &User{ID: "user123", Username: "testuser"}
	session, err := manager.sessionManager.CreateSession(user)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	manager.SetupRoutes(router, "/")

	// Test logout with session cookie
	req := httptest.NewRequest("POST", "/auth/logout", nil)
	req.AddCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: session.ID,
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code) // Redirect for web request
	assert.Equal(t, "/", w.Header().Get("Location"))

	// Verify session is deleted
	_, err = manager.sessionManager.GetSession(session.ID)
	assert.Error(t, err)

	// Verify cookie is cleared
	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == SessionCookieName {
			sessionCookie = cookie
			break
		}
	}
	assert.NotNil(t, sessionCookie)
	assert.Equal(t, "", sessionCookie.Value)
	assert.True(t, sessionCookie.MaxAge < 0)
}

func TestManager_HandleCreateScreenToken(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled:      true,
		ScreenTokens: true,
	}

	mockStore := newMockStore()
	manager, err := NewManager(cfg, mockStore, "https://app.example.com")
	require.NoError(t, err)

	// Create authenticated session
	user := &User{ID: "user123", Username: "testuser"}
	session, err := manager.sessionManager.CreateSession(user)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	manager.SetupRoutes(router, "/")

	req := httptest.NewRequest("POST", "/auth/screen-token", nil)
	req.AddCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: session.ID,
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "token")
}

func TestManager_HandleCreateScreenToken_Disabled(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled:      true,
		ScreenTokens: false, // Disabled
	}

	mockStore := newMockStore()
	manager, err := NewManager(cfg, mockStore, "https://app.example.com")
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	manager.SetupRoutes(router, "/")

	req := httptest.NewRequest("POST", "/auth/screen-token", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestManager_HandleDeleteScreenToken(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled:      true,
		ScreenTokens: true,
	}

	mockStore := newMockStore()
	manager, err := NewManager(cfg, mockStore, "https://app.example.com")
	require.NoError(t, err)

	// Create authenticated session
	user := &User{ID: "user123", Username: "testuser"}
	session, err := manager.sessionManager.CreateSession(user)
	require.NoError(t, err)

	// Create a screen token
	token, err := mockStore.CreateScreenToken()
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	manager.SetupRoutes(router, "/")

	req := httptest.NewRequest("DELETE", "/auth/screen-token/"+token, nil)
	req.AddCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: session.ID,
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "deleted successfully")

	// Verify token is deleted
	assert.False(t, mockStore.ValidateScreenToken(token))
}

func TestManager_HandleAuthStatus(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled:      true,
		ScreenTokens: true,
	}

	mockStore := newMockStore()
	manager, err := NewManager(cfg, mockStore, "https://app.example.com")
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(manager.middleware.OptionalAuth())
	manager.SetupRoutes(router, "/")

	// Test without authentication
	req := httptest.NewRequest("GET", "/auth/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"enabled":true`)
	assert.Contains(t, w.Body.String(), `"screen_tokens":true`)
	assert.Contains(t, w.Body.String(), `"authenticated":false`)

	// Test with authentication
	user := &User{
		ID:       "user123",
		Username: "testuser",
		Name:     "Test User",
		Email:    "test@example.com",
	}
	session, err := manager.sessionManager.CreateSession(user)
	require.NoError(t, err)

	req2 := httptest.NewRequest("GET", "/auth/status", nil)
	req2.AddCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: session.ID,
	})
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Contains(t, w2.Body.String(), `"authenticated":true`)
	assert.Contains(t, w2.Body.String(), `"id":"user123"`)
	assert.Contains(t, w2.Body.String(), `"username":"testuser"`)
}

func TestManager_GetMiddleware(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled: true,
	}

	mockStore := newMockStore()
	manager, err := NewManager(cfg, mockStore, "https://app.example.com")
	require.NoError(t, err)

	middleware := manager.GetMiddleware()
	assert.NotNil(t, middleware)
	assert.Equal(t, manager.middleware, middleware)
}

func TestManager_Close(t *testing.T) {
	cfg := &config.AuthConfig{
		Enabled: true,
	}

	mockStore := newMockStore()
	manager, err := NewManager(cfg, mockStore, "https://app.example.com")
	require.NoError(t, err)

	// Create a session to verify cleanup
	user := &User{ID: "user123", Username: "testuser"}
	_, err = manager.sessionManager.CreateSession(user)
	require.NoError(t, err)

	assert.Equal(t, 1, manager.sessionManager.GetSessionCount())

	err = manager.Close()
	require.NoError(t, err)

	assert.Equal(t, 0, manager.sessionManager.GetSessionCount())
}
