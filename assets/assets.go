package assets

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

//go:generate ../scripts/build-assets.sh

//go:embed static
var staticFiles embed.FS

//go:embed templates
var templateFiles embed.FS

// AssetManager handles static assets and templates
type AssetManager struct {
	staticFS   embed.FS
	templateFS embed.FS
	templates  *template.Template
	debug      bool
}

// NewAssetManager creates a new asset manager
func NewAssetManager(debug bool) (*AssetManager, error) {
	am := &AssetManager{
		staticFS:   staticFiles,
		templateFS: templateFiles,
		debug:      debug,
	}

	// Parse templates
	if err := am.loadTemplates(); err != nil {
		return nil, err
	}

	return am, nil
}

// loadTemplates parses all HTML templates
func (am *AssetManager) loadTemplates() error {
	// Create template with custom functions
	tmpl := template.New("").Funcs(template.FuncMap{
		"safeJS": func(s string) template.JS {
			return template.JS(s)
		},
	})

	// Walk through template files
	err := fs.WalkDir(am.templateFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}

		// Read template content
		content, err := am.templateFS.ReadFile(path)
		if err != nil {
			return err
		}

		// Get template name (remove templates/ prefix and .html suffix)
		name := strings.TrimPrefix(path, "templates/")
		name = strings.TrimSuffix(name, ".html")

		// Parse template
		_, err = tmpl.New(name).Parse(string(content))
		return err
	})

	if err != nil {
		return err
	}

	am.templates = tmpl
	return nil
}

// ServeStatic returns a Gin handler for serving static files
func (am *AssetManager) ServeStatic(routePrefix string) gin.HandlerFunc {
	// Create a sub-filesystem for static files
	staticFS, err := fs.Sub(am.staticFS, "static")
	if err != nil {
		panic(err)
	}

	fileServer := http.FileServer(http.FS(staticFS))

	return func(c *gin.Context) {
		// Remove the route prefix and /static from the path
		fullPrefix := routePrefix
		if fullPrefix == "/" {
			fullPrefix = ""
		}
		path := strings.TrimPrefix(c.Request.URL.Path, fullPrefix+"/static")
		if path == "" || path[0] != '/' {
			path = "/" + path
		}

		// Set caching headers for static assets
		am.setCacheHeaders(c, path)

		// Create a new request with the modified path
		c.Request.URL.Path = path
		fileServer.ServeHTTP(c.Writer, c.Request)
	}
}

// setCacheHeaders sets appropriate caching headers for static assets
func (am *AssetManager) setCacheHeaders(c *gin.Context, path string) {
	ext := filepath.Ext(path)

	// Set cache headers based on file type
	switch ext {
	case ".js", ".css":
		// JavaScript and CSS files - cache for 1 year if they have hash in name
		if strings.Contains(path, "-") && len(strings.Split(filepath.Base(path), "-")) > 1 {
			// Hashed files can be cached for a long time
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			// Non-hashed files should be revalidated
			c.Header("Cache-Control", "public, max-age=3600, must-revalidate")
		}
	case ".woff", ".woff2", ".ttf", ".eot":
		// Font files - cache for 1 year
		c.Header("Cache-Control", "public, max-age=31536000")
	case ".ico", ".png", ".jpg", ".jpeg", ".gif", ".svg":
		// Image files - cache for 1 week
		c.Header("Cache-Control", "public, max-age=604800")
	default:
		// Other files - cache for 1 hour
		c.Header("Cache-Control", "public, max-age=3600")
	}

	// Set Last-Modified header (use build time in production)
	if !am.debug {
		c.Header("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	}
}

// RenderTemplate renders an HTML template with the given data
func (am *AssetManager) RenderTemplate(c *gin.Context, name string, data interface{}) error {
	c.Header("Content-Type", "text/html; charset=utf-8")
	return am.templates.ExecuteTemplate(c.Writer, name, data)
}

// GetTemplate returns a parsed template by name
func (am *AssetManager) GetTemplate(name string) *template.Template {
	return am.templates.Lookup(name)
}

// HasTemplate checks if a template exists
func (am *AssetManager) HasTemplate(name string) bool {
	return am.templates.Lookup(name) != nil
}

// GetAppJSFilename returns the name of the main JavaScript file
// It looks for files matching app*.js pattern and returns the first one found
func (am *AssetManager) GetAppJSFilename() string {
	entries, err := fs.ReadDir(am.staticFS, "static/build")
	if err != nil {
		return "app.js" // fallback
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "app") && strings.HasSuffix(entry.Name(), ".js") {
			return entry.Name()
		}
	}

	return "app.js" // fallback
}
