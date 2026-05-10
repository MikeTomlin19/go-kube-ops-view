package auth

import (
	"time"
)

// User represents an authenticated user
type User struct {
	ID       string                 `json:"id"`
	Email    string                 `json:"email"`
	Name     string                 `json:"name"`
	Username string                 `json:"username"`
	Groups   []string               `json:"groups,omitempty"`
	Claims   map[string]interface{} `json:"claims,omitempty"`
}

// Session represents a user session
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	User      *User     `json:"user"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	LastSeen  time.Time `json:"last_seen"`
}

// OAuthConfig contains OAuth2 configuration
type OAuthConfig struct {
	AuthorizeURL string `json:"authorize_url"`
	TokenURL     string `json:"token_url"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Scope        string `json:"scope"`
	RedirectURL  string `json:"redirect_url"`
}

// AuthError represents authentication-related errors
type AuthError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Cause   error  `json:"-"`
}

func (e *AuthError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// Error codes
const (
	ErrCodeInvalidCredentials = "INVALID_CREDENTIALS"
	ErrCodeSessionExpired     = "SESSION_EXPIRED"
	ErrCodeSessionNotFound    = "SESSION_NOT_FOUND"
	ErrCodeOAuthError         = "OAUTH_ERROR"
	ErrCodeTokenInvalid       = "TOKEN_INVALID"
	ErrCodeUnauthorized       = "UNAUTHORIZED"
	ErrCodeForbidden          = "FORBIDDEN"
)

// NewAuthError creates a new authentication error
func NewAuthError(code, message string, cause error) *AuthError {
	return &AuthError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}
