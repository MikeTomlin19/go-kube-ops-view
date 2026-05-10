# Asset Management

This package handles static asset management and HTML template rendering for the Kube Ops View Go application.

## Features

- **Static Asset Embedding**: All static files (JavaScript, CSS, images, fonts) are embedded into the Go binary using `go:embed`
- **Template Rendering**: HTML templates are parsed and rendered with Go's `html/template` package
- **Caching Headers**: Appropriate HTTP caching headers are set based on file types and naming patterns
- **Webpack Integration**: Build pipeline integrates with the existing webpack configuration
- **Dynamic Asset Discovery**: Automatically detects hashed JavaScript filenames from webpack builds

## Directory Structure

```
assets/
├── static/           # Static files embedded into binary
│   ├── build/        # Webpack build output (JavaScript bundles)
│   ├── favicon.ico   # Application favicon
│   └── sharetechmono.woff2  # Font file
├── templates/        # HTML templates
│   ├── index.html    # Main application page
│   └── screen-tokens.html  # Screen token management page
├── assets.go         # Main asset manager implementation
├── assets_test.go    # Unit tests
├── integration_test.go  # Integration tests
└── README.md         # This file
```

## Usage

### Creating an Asset Manager

```go
import "kube-ops-view/assets"

// Create asset manager
am, err := assets.NewAssetManager(debug)
if err != nil {
    log.Fatal(err)
}
```

### Serving Static Files

```go
// Set up Gin router
router := gin.New()

// Serve static files with caching headers
router.GET("/static/*filepath", am.ServeStatic("/"))
```

### Rendering Templates

```go
// Render the main page
data := map[string]interface{}{
    "Version":       "v1.0.0",
    "RoutePrefix":   "/",
    "AppJS":         am.GetAppJSFilename(),
    "AppConfigJSON": `{"debug": true}`,
}

err := am.RenderTemplate(c, "index", data)
```

## Build Process

The build process is handled by the `scripts/build-assets.sh` script and integrated into the Makefile:

1. **Frontend Build**: Runs webpack to compile JavaScript assets
2. **Asset Copying**: Copies static files to the correct locations
3. **Go Build**: Embeds assets and builds the Go binary

### Building Assets

```bash
# Build frontend assets only
./scripts/build-assets.sh

# Build complete application (assets + Go binary)
make build-go
```

## Caching Strategy

The asset manager implements intelligent caching based on file types:

- **Hashed Files** (e.g., `app-abc123.js`): 1 year cache with `immutable` flag
- **Non-hashed JS/CSS**: 1 hour cache with `must-revalidate`
- **Fonts**: 1 year cache
- **Images**: 1 week cache
- **Other files**: 1 hour cache

## Template Functions

Custom template functions are available:

- `safeJS`: Renders JavaScript code without HTML escaping

## Testing

The package includes comprehensive tests:

```bash
# Run all tests
go test ./assets -v

# Run specific test suites
go test ./assets -v -run TestAssetIntegration
go test ./assets -v -run TestAssetCaching
```

## Requirements Satisfied

This implementation satisfies the following requirements from the specification:

- **Requirement 7.1**: Maintains the same WebGL-based visual rendering by preserving existing JavaScript assets
- **Requirement 8.1**: Produces a single Go binary with embedded static assets

## Future Enhancements

- CSS minification and bundling
- Asset versioning and cache busting
- Gzip compression for static assets
- Development mode with live reloading