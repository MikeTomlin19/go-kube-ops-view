package auth

import (
	"net/http"
	"strings"

	"kube-ops-view/internal/store"

	"github.com/gin-gonic/gin"
)

const (
	// Context keys
	UserContextKey    = "auth_user"
	SessionContextKey = "auth_session"

	// Cookie names
	SessionCookieName = "kube-ops-view-session"

	// Header names
	AuthorizationHeader = "Authorization"
	ScreenTokenHeader   = "X-Screen-Token"
)

// Middleware provides authentication middleware for Gin
type Middleware struct {
	sessionManager *SessionManager
	store          store.Store
	enabled        bool
	screenTokens   bool
}

// NewMiddleware creates a new authentication middleware
func NewMiddleware(sessionManager *SessionManager, store store.Store, enabled, screenTokens bool) *Middleware {
	return &Middleware{
		sessionManager: sessionManager,
		store:          store,
		enabled:        enabled,
		screenTokens:   screenTokens,
	}
}

// RequireAuth middleware that requires authentication
func (m *Middleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.enabled {
			// Authentication disabled, allow all requests
			c.Next()
			return
		}

		// Check for screen token first (if enabled)
		if m.screenTokens {
			if token := m.getScreenToken(c); token != "" {
				if m.store.ValidateScreenToken(token) {
					// Valid screen token, allow access
					c.Next()
					return
				}
			}
		}

		// Check for session authentication
		session, err := m.getSessionFromRequest(c)
		if err != nil {
			m.handleAuthError(c, err)
			return
		}

		// Set user and session in context
		c.Set(UserContextKey, session.User)
		c.Set(SessionContextKey, session)

		c.Next()
	}
}

// OptionalAuth middleware that optionally authenticates users
func (m *Middleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.enabled {
			c.Next()
			return
		}

		// Try to get session, but don't fail if not found
		session, err := m.getSessionFromRequest(c)
		if err == nil && session != nil {
			c.Set(UserContextKey, session.User)
			c.Set(SessionContextKey, session)
		}

		c.Next()
	}
}

// ScreenTokenAuth middleware that only allows screen token authentication
func (m *Middleware) ScreenTokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.enabled || !m.screenTokens {
			c.Next()
			return
		}

		token := m.getScreenToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "screen token required",
				"code":  ErrCodeUnauthorized,
			})
			c.Abort()
			return
		}

		if !m.store.ValidateScreenToken(token) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid screen token",
				"code":  ErrCodeTokenInvalid,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// getSessionFromRequest extracts session from request
func (m *Middleware) getSessionFromRequest(c *gin.Context) (*Session, error) {
	// Try to get session ID from cookie
	sessionID, err := c.Cookie(SessionCookieName)
	if err != nil {
		// Try Authorization header as fallback
		authHeader := c.GetHeader(AuthorizationHeader)
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			sessionID = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			return nil, NewAuthError(ErrCodeUnauthorized, "no session found", nil)
		}
	}

	if sessionID == "" {
		return nil, NewAuthError(ErrCodeUnauthorized, "no session ID provided", nil)
	}

	// Get session from session manager
	session, err := m.sessionManager.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	return session, nil
}

// getScreenToken extracts screen token from request
func (m *Middleware) getScreenToken(c *gin.Context) string {
	// Try header first
	if token := c.GetHeader(ScreenTokenHeader); token != "" {
		return token
	}

	// Try query parameter
	if token := c.Query("screen_token"); token != "" {
		return token
	}

	// Try form parameter
	if token := c.PostForm("screen_token"); token != "" {
		return token
	}

	return ""
}

// handleAuthError handles authentication errors
func (m *Middleware) handleAuthError(c *gin.Context, err error) {
	authErr, ok := err.(*AuthError)
	if !ok {
		authErr = NewAuthError(ErrCodeUnauthorized, "authentication failed", err)
	}

	// Check if this is an API request (JSON content type or Accept header)
	if m.isAPIRequest(c) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": authErr.Message,
			"code":  authErr.Code,
		})
	} else {
		// Redirect to login for web requests
		loginURL := "/auth/login"
		if returnURL := c.Request.URL.String(); returnURL != "" && returnURL != "/" {
			loginURL += "?return_url=" + returnURL
		}
		c.Redirect(http.StatusFound, loginURL)
	}

	c.Abort()
}

// isAPIRequest determines if the request is an API request
func (m *Middleware) isAPIRequest(c *gin.Context) bool {
	// Check Content-Type
	contentType := c.GetHeader("Content-Type")
	if strings.Contains(contentType, "application/json") {
		return true
	}

	// Check Accept header
	accept := c.GetHeader("Accept")
	if strings.Contains(accept, "application/json") {
		return true
	}

	// Check if path starts with /api/
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		return true
	}

	// Check for AJAX requests
	if c.GetHeader("X-Requested-With") == "XMLHttpRequest" {
		return true
	}

	return false
}

// GetUser returns the authenticated user from context
func GetUser(c *gin.Context) *User {
	if user, exists := c.Get(UserContextKey); exists {
		if u, ok := user.(*User); ok {
			return u
		}
	}
	return nil
}

// GetSession returns the session from context
func GetSession(c *gin.Context) *Session {
	if session, exists := c.Get(SessionContextKey); exists {
		if s, ok := session.(*Session); ok {
			return s
		}
	}
	return nil
}

// IsAuthenticated checks if the request is authenticated
func IsAuthenticated(c *gin.Context) bool {
	return GetUser(c) != nil
}
