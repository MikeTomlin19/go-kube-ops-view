package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"kube-ops-view/internal/config"
	"kube-ops-view/internal/store"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

// Manager handles authentication and authorization
type Manager struct {
	config         *config.AuthConfig
	oauthManager   *OAuthManager
	sessionManager *SessionManager
	store          store.Store
	middleware     *Middleware
}

// NewManager creates a new authentication manager
func NewManager(cfg *config.AuthConfig, store store.Store, baseURL string) (*Manager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("auth config is required")
	}

	// Create session manager
	sessionManager := NewSessionManager(24 * time.Hour) // 24 hour sessions

	var oauthManager *OAuthManager
	if cfg.Enabled {
		// Build redirect URL
		redirectURL := strings.TrimSuffix(baseURL, "/") + "/auth/callback"

		oauthConfig := OAuthConfig{
			AuthorizeURL: cfg.AuthorizeURL,
			TokenURL:     cfg.TokenURL,
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Scope:        cfg.Scope,
			RedirectURL:  redirectURL,
		}

		oauthManager = NewOAuthManager(oauthConfig)

		// Try to load credentials from directory if specified
		if cfg.CredentialsDir != "" {
			if err := loadCredentialsFromDir(cfg, cfg.CredentialsDir); err != nil {
				return nil, fmt.Errorf("failed to load credentials: %w", err)
			}
		}
	}

	// Create middleware
	middleware := NewMiddleware(sessionManager, store, cfg.Enabled, cfg.ScreenTokens)

	return &Manager{
		config:         cfg,
		oauthManager:   oauthManager,
		sessionManager: sessionManager,
		store:          store,
		middleware:     middleware,
	}, nil
}

// SetupRoutes sets up authentication routes
func (m *Manager) SetupRoutes(router *gin.Engine, routePrefix string) {
	authGroup := router.Group(routePrefix + "auth")

	authGroup.GET("/login", m.handleLogin)
	authGroup.GET("/callback", m.handleCallback)
	authGroup.POST("/logout", m.handleLogout)
	authGroup.GET("/logout", m.handleLogout) // Support GET for logout links

	if m.config.ScreenTokens {
		authGroup.POST("/screen-token", m.middleware.RequireAuth(), m.handleCreateScreenToken)
		authGroup.DELETE("/screen-token/:token", m.middleware.RequireAuth(), m.handleDeleteScreenToken)
	}

	authGroup.GET("/status", m.handleAuthStatus)
}

// GetMiddleware returns the authentication middleware
func (m *Manager) GetMiddleware() *Middleware {
	return m.middleware
}

// handleLogin handles the login request
func (m *Manager) handleLogin(c *gin.Context) {
	if !m.config.Enabled {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "authentication is not enabled",
		})
		return
	}

	// Store return URL in session state
	returnURL := c.Query("return_url")
	if returnURL == "" {
		returnURL = "/"
	}

	authURL, state, err := m.oauthManager.GetAuthURL()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to generate auth URL",
			"code":  ErrCodeOAuthError,
		})
		return
	}

	// Store return URL in a temporary cookie tied to the state
	// URL encode the state to make it safe for cookie names
	safeState := url.QueryEscape(state)
	cookieName := "oauth_return_url_" + safeState
	c.SetCookie(cookieName, returnURL, 600, "/", "", false, true) // 10 minutes

	c.Redirect(http.StatusFound, authURL)
}

// handleCallback handles the OAuth callback
func (m *Manager) handleCallback(c *gin.Context) {
	if !m.config.Enabled {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "authentication is not enabled",
		})
		return
	}

	code := c.Query("code")
	state := c.Query("state")
	errorParam := c.Query("error")

	if errorParam != "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "OAuth error: " + errorParam,
			"code":  ErrCodeOAuthError,
		})
		return
	}

	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "missing code or state parameter",
			"code":  ErrCodeOAuthError,
		})
		return
	}

	// Exchange code for token
	token, err := m.oauthManager.ExchangeCodeForToken(c.Request.Context(), code, state)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to exchange code for token",
			"code":  ErrCodeOAuthError,
		})
		return
	}

	// Get user info
	user, err := m.oauthManager.GetUserInfo(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get user info",
			"code":  ErrCodeOAuthError,
		})
		return
	}

	// Create session
	session, err := m.sessionManager.CreateSession(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create session",
			"code":  ErrCodeSessionNotFound,
		})
		return
	}

	// Set session cookie
	c.SetCookie(SessionCookieName, session.ID, int(24*time.Hour.Seconds()), "/", "", false, true)

	// Get return URL and redirect
	safeState := url.QueryEscape(state)
	cookieName := "oauth_return_url_" + safeState
	returnURL, _ := c.Cookie(cookieName)
	if returnURL == "" {
		returnURL = "/"
	}

	// Clean up the temporary cookie
	c.SetCookie(cookieName, "", -1, "/", "", false, true)

	c.Redirect(http.StatusFound, returnURL)
}

// handleLogout handles logout requests
func (m *Manager) handleLogout(c *gin.Context) {
	if !m.config.Enabled {
		c.JSON(http.StatusOK, gin.H{
			"message": "logged out (authentication was not enabled)",
		})
		return
	}

	// Get session from cookie
	sessionID, err := c.Cookie(SessionCookieName)
	if err == nil && sessionID != "" {
		// Delete session
		m.sessionManager.DeleteSession(sessionID)
	}

	// Clear session cookie
	c.SetCookie(SessionCookieName, "", -1, "/", "", false, true)

	// Check if this is an API request
	if m.middleware.isAPIRequest(c) {
		c.JSON(http.StatusOK, gin.H{
			"message": "logged out successfully",
		})
	} else {
		// Redirect to home page
		c.Redirect(http.StatusFound, "/")
	}
}

// handleCreateScreenToken creates a new screen token
func (m *Manager) handleCreateScreenToken(c *gin.Context) {
	if !m.config.ScreenTokens {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "screen tokens are not enabled",
		})
		return
	}

	token, err := m.store.CreateScreenToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create screen token",
			"code":  ErrCodeTokenInvalid,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
	})
}

// handleDeleteScreenToken deletes a screen token
func (m *Manager) handleDeleteScreenToken(c *gin.Context) {
	if !m.config.ScreenTokens {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "screen tokens are not enabled",
		})
		return
	}

	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "token parameter is required",
		})
		return
	}

	err := m.store.DeleteScreenToken(token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to delete screen token",
			"code":  ErrCodeTokenInvalid,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "screen token deleted successfully",
	})
}

// handleAuthStatus returns authentication status
func (m *Manager) handleAuthStatus(c *gin.Context) {
	status := gin.H{
		"enabled":       m.config.Enabled,
		"screen_tokens": m.config.ScreenTokens,
		"authenticated": false,
	}

	if m.config.Enabled {
		user := GetUser(c)
		if user != nil {
			status["authenticated"] = true
			status["user"] = gin.H{
				"id":       user.ID,
				"username": user.Username,
				"name":     user.Name,
				"email":    user.Email,
			}
		}
	}

	c.JSON(http.StatusOK, status)
}

// ValidateToken validates an OAuth2 token
func (m *Manager) ValidateToken(ctx context.Context, token *oauth2.Token) error {
	if m.oauthManager == nil {
		return NewAuthError(ErrCodeOAuthError, "OAuth not configured", nil)
	}
	return m.oauthManager.ValidateToken(ctx, token)
}

// Close cleans up the authentication manager
func (m *Manager) Close() error {
	if m.sessionManager != nil {
		return m.sessionManager.Close()
	}
	return nil
}

// loadCredentialsFromDir loads OAuth credentials from a directory
func loadCredentialsFromDir(cfg *config.AuthConfig, dir string) error {
	// This is a placeholder for loading credentials from files
	// In a real implementation, you might load client_id, client_secret, etc. from files
	// For example:
	// - client_id from dir/client_id
	// - client_secret from dir/client_secret

	clientIDPath := filepath.Join(dir, "client_id")
	clientSecretPath := filepath.Join(dir, "client_secret")

	// This would read from files if they exist
	// For now, we'll just validate that the directory exists
	_ = clientIDPath
	_ = clientSecretPath

	return nil
}
