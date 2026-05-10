package assets

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAssetIntegration tests the complete asset serving pipeline
func TestAssetIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create asset manager
	am, err := NewAssetManager(false)
	require.NoError(t, err)

	// Create a test router
	router := gin.New()

	// Set up routes like a real server would
	router.GET("/static/*filepath", am.ServeStatic("/"))
	router.GET("/", func(c *gin.Context) {
		data := map[string]interface{}{
			"Version":       "test-version",
			"RoutePrefix":   "/",
			"AppJS":         am.GetAppJSFilename(),
			"AppConfigJSON": `{"debug": true, "clusters": []}`,
		}
		err := am.RenderTemplate(c, "index", data)
		if err != nil {
			c.String(http.StatusInternalServerError, "Template error: %v", err)
		}
	})

	tests := []struct {
		name            string
		path            string
		expectedStatus  int
		expectedContent string
	}{
		{
			name:            "serve main page",
			path:            "/",
			expectedStatus:  http.StatusOK,
			expectedContent: "Kubernetes Operational View",
		},
		{
			name:            "serve JavaScript file",
			path:            "/static/build/" + am.GetAppJSFilename(),
			expectedStatus:  http.StatusOK,
			expectedContent: "App",
		},
		{
			name:           "serve favicon",
			path:           "/static/favicon.ico",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "serve font file",
			path:           "/static/sharetechmono.woff2",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "404 for non-existent file",
			path:           "/static/nonexistent.js",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", tt.path, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedContent != "" {
				assert.Contains(t, w.Body.String(), tt.expectedContent)
			}
		})
	}
}

// TestAssetCaching tests that proper cache headers are set
func TestAssetCaching(t *testing.T) {
	gin.SetMode(gin.TestMode)

	am, err := NewAssetManager(false)
	require.NoError(t, err)

	router := gin.New()
	router.GET("/static/*filepath", am.ServeStatic("/"))

	tests := []struct {
		name          string
		path          string
		expectedCache string
	}{
		{
			name:          "hashed JavaScript file",
			path:          "/static/build/" + am.GetAppJSFilename(),
			expectedCache: "public, max-age=31536000, immutable",
		},
		{
			name:          "font file",
			path:          "/static/sharetechmono.woff2",
			expectedCache: "public, max-age=31536000",
		},
		{
			name:          "favicon",
			path:          "/static/favicon.ico",
			expectedCache: "public, max-age=604800",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", tt.path, nil)
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				cacheControl := w.Header().Get("Cache-Control")
				assert.Contains(t, cacheControl, tt.expectedCache)
			}
		})
	}
}

// TestTemplateWithRealAssets tests template rendering with actual asset filenames
func TestTemplateWithRealAssets(t *testing.T) {
	am, err := NewAssetManager(false)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Use real asset filename
	appJS := am.GetAppJSFilename()

	data := map[string]interface{}{
		"Version":       "v1.0.0",
		"RoutePrefix":   "/",
		"AppJS":         appJS,
		"AppConfigJSON": `{"debug": false, "clusters": ["cluster1"]}`,
	}

	err = am.RenderTemplate(c, "index", data)
	require.NoError(t, err)

	body := w.Body.String()

	// Check that the template contains the correct asset references
	assert.Contains(t, body, "v1.0.0")
	assert.Contains(t, body, appJS)
	assert.Contains(t, body, `{"debug": false, "clusters": ["cluster1"]}`)
	assert.Contains(t, body, "/static/favicon.ico")
	assert.Contains(t, body, "/static/sharetechmono.woff2")
}
