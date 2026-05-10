package assets

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAssetManager(t *testing.T) {
	tests := []struct {
		name  string
		debug bool
	}{
		{
			name:  "production mode",
			debug: false,
		},
		{
			name:  "debug mode",
			debug: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			am, err := NewAssetManager(tt.debug)
			require.NoError(t, err)
			assert.NotNil(t, am)
			assert.Equal(t, tt.debug, am.debug)
			assert.NotNil(t, am.templates)
		})
	}
}

func TestAssetManager_HasTemplate(t *testing.T) {
	am, err := NewAssetManager(false)
	require.NoError(t, err)

	// Test existing template
	assert.True(t, am.HasTemplate("index"))
	assert.True(t, am.HasTemplate("screen-tokens"))

	// Test non-existing template
	assert.False(t, am.HasTemplate("nonexistent"))
}

func TestAssetManager_GetTemplate(t *testing.T) {
	am, err := NewAssetManager(false)
	require.NoError(t, err)

	// Test existing template
	tmpl := am.GetTemplate("index")
	assert.NotNil(t, tmpl)

	// Test non-existing template
	tmpl = am.GetTemplate("nonexistent")
	assert.Nil(t, tmpl)
}

func TestAssetManager_RenderTemplate(t *testing.T) {
	am, err := NewAssetManager(false)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	data := map[string]interface{}{
		"Version":       "test-version",
		"RoutePrefix":   "/",
		"AppJS":         "app.js",
		"AppConfigJSON": "{}",
	}

	err = am.RenderTemplate(c, "index", data)
	require.NoError(t, err)

	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "test-version")
	assert.Contains(t, w.Body.String(), "app.js")
}

func TestAssetManager_ServeStatic(t *testing.T) {
	am, err := NewAssetManager(false)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Set up static file serving
	router.GET("/static/*filepath", am.ServeStatic("/"))

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedHeader string
	}{
		{
			name:           "serve existing file",
			path:           "/static/favicon.ico",
			expectedStatus: http.StatusOK,
			expectedHeader: "public, max-age=604800",
		},
		{
			name:           "serve non-existing file",
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
			if tt.expectedHeader != "" {
				assert.Contains(t, w.Header().Get("Cache-Control"), tt.expectedHeader)
			}
		})
	}
}

func TestAssetManager_setCacheHeaders(t *testing.T) {
	am, err := NewAssetManager(false)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		path          string
		expectedCache string
	}{
		{
			name:          "JavaScript file with hash",
			path:          "/app-abc123.js",
			expectedCache: "public, max-age=31536000, immutable",
		},
		{
			name:          "JavaScript file without hash",
			path:          "/app.js",
			expectedCache: "public, max-age=3600, must-revalidate",
		},
		{
			name:          "CSS file with hash",
			path:          "/style-abc123.css",
			expectedCache: "public, max-age=31536000, immutable",
		},
		{
			name:          "Font file",
			path:          "/font.woff2",
			expectedCache: "public, max-age=31536000",
		},
		{
			name:          "Image file",
			path:          "/image.png",
			expectedCache: "public, max-age=604800",
		},
		{
			name:          "Other file",
			path:          "/data.json",
			expectedCache: "public, max-age=3600",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			am.setCacheHeaders(c, tt.path)

			assert.Contains(t, w.Header().Get("Cache-Control"), tt.expectedCache)
		})
	}
}

func TestAssetManager_loadTemplates(t *testing.T) {
	am := &AssetManager{
		templateFS: templateFiles,
		debug:      false,
	}

	err := am.loadTemplates()
	require.NoError(t, err)

	// Check that templates were loaded
	assert.NotNil(t, am.templates)
	assert.True(t, am.HasTemplate("index"))
	assert.True(t, am.HasTemplate("screen-tokens"))
}

func TestAssetManager_GetAppJSFilename(t *testing.T) {
	am, err := NewAssetManager(false)
	require.NoError(t, err)

	filename := am.GetAppJSFilename()
	assert.NotEmpty(t, filename)
	assert.True(t, strings.HasPrefix(filename, "app"))
	assert.True(t, strings.HasSuffix(filename, ".js"))
}

// Test template rendering with different data
func TestTemplateRendering(t *testing.T) {
	am, err := NewAssetManager(false)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		template     string
		data         interface{}
		expectedText []string
	}{
		{
			name:     "index template",
			template: "index",
			data: map[string]interface{}{
				"Version":       "v1.0.0",
				"RoutePrefix":   "/app/",
				"AppJS":         "app-hash123.js",
				"AppConfigJSON": `{"debug": true}`,
			},
			expectedText: []string{"v1.0.0", "/app/static/favicon.ico", "app-hash123.js", `{"debug": true}`},
		},
		{
			name:     "screen-tokens template with token",
			template: "screen-tokens",
			data: map[string]interface{}{
				"RoutePrefix": "/",
				"NewToken":    "abc123def456",
			},
			expectedText: []string{"Screen Tokens", "abc123def456", "Create new token"},
		},
		{
			name:     "screen-tokens template without token",
			template: "screen-tokens",
			data: map[string]interface{}{
				"RoutePrefix": "/",
			},
			expectedText: []string{"Screen Tokens", "Create new token"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			err := am.RenderTemplate(c, tt.template, tt.data)
			require.NoError(t, err)

			body := w.Body.String()
			for _, expected := range tt.expectedText {
				assert.Contains(t, body, expected)
			}
		})
	}
}
