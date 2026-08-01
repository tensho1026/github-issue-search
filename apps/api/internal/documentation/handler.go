package apidocs

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
)

const (
	// RootPath is the canonical route without a trailing slash.
	RootPath = "/docs"
	// IndexPath is the canonical interactive-reference URL.
	IndexPath = RootPath + "/"
	// OpenAPIPath is the stable machine-readable contract URL.
	OpenAPIPath = "/openapi.yaml"
)

const (
	javascriptPath  = "/docs/issuescout-swagger.js"
	stylesheetPath  = "/docs/issuescout-swagger.css"
	documentMaxAge  = "public, max-age=300, must-revalidate"
	staticMaxAge    = "public, max-age=3600, must-revalidate"
	contentSecurity = "default-src 'none'; base-uri 'none'; connect-src 'self'; font-src 'self'; form-action 'none'; frame-ancestors 'none'; img-src 'self' data:; manifest-src 'none'; object-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; worker-src 'none'"
)

var allowedSwaggerAssets = map[string]struct{}{
	"/docs/favicon-16x16.png":               {},
	"/docs/favicon-32x32.png":               {},
	"/docs/swagger-ui-bundle.js":            {},
	"/docs/swagger-ui-standalone-preset.js": {},
	"/docs/swagger-ui.css":                  {},
}

type document struct {
	body        []byte
	cachePolicy string
	contentType string
	etag        string
}

// Handler serves the documentation index, IssueScout bootstrap assets, the
// embedded OpenAPI contract, and an allowlisted set of Swagger UI assets.
type Handler struct {
	assets    http.Handler
	documents map[string]document
}

// New creates an immutable documentation handler around injected embedded
// Swagger UI assets. It performs all document hashing once during startup.
func New(assets http.Handler) (*Handler, error) {
	if assets == nil {
		return nil, fmt.Errorf("compose API documentation: assets are required")
	}
	if len(openAPISource) == 0 {
		return nil, fmt.Errorf("compose API documentation: OpenAPI source is empty")
	}

	documents := map[string]document{
		IndexPath: newDocument(
			[]byte(indexHTML),
			"text/html; charset=utf-8",
			documentMaxAge,
		),
		OpenAPIPath: newDocument(
			openAPISource,
			"application/yaml; charset=utf-8",
			documentMaxAge,
		),
		javascriptPath: newDocument(
			[]byte(bootstrapJavaScript),
			"text/javascript; charset=utf-8",
			staticMaxAge,
		),
		stylesheetPath: newDocument(
			[]byte(documentationCSS),
			"text/css; charset=utf-8",
			staticMaxAge,
		),
	}

	return &Handler{
		assets:    assets,
		documents: documents,
	}, nil
}

// ServeHTTP routes only the public documentation surface. Unknown asset names
// never reach the embedded filesystem.
func (handler *Handler) ServeHTTP(
	writer http.ResponseWriter,
	request *http.Request,
) {
	setDocumentationSecurityHeaders(writer.Header())
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.Header().Set("Allow", "GET, HEAD")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)

		return
	}

	if request.URL.Path == RootPath {
		http.Redirect(
			writer,
			request,
			IndexPath,
			http.StatusPermanentRedirect,
		)

		return
	}

	if value, exists := handler.documents[request.URL.Path]; exists {
		serveDocument(writer, request, value)

		return
	}

	if _, exists := allowedSwaggerAssets[request.URL.Path]; exists {
		writer.Header().Set("Cache-Control", staticMaxAge)
		handler.assets.ServeHTTP(writer, request)

		return
	}

	http.NotFound(writer, request)
}

func newDocument(body []byte, contentType, cachePolicy string) document {
	hash := sha256.Sum256(body)

	return document{
		body:        body,
		cachePolicy: cachePolicy,
		contentType: contentType,
		etag:        fmt.Sprintf(`"%x"`, hash),
	}
}

func serveDocument(
	writer http.ResponseWriter,
	request *http.Request,
	value document,
) {
	writer.Header().Set("Cache-Control", value.cachePolicy)
	writer.Header().Set("Content-Type", value.contentType)
	writer.Header().Set("ETag", value.etag)
	if etagMatches(request.Header.Get("If-None-Match"), value.etag) {
		writer.WriteHeader(http.StatusNotModified)

		return
	}
	writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(value.body)))
	if request.Method == http.MethodHead {
		writer.WriteHeader(http.StatusOK)

		return
	}
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(value.body)
}

func etagMatches(header, etag string) bool {
	for candidate := range strings.SplitSeq(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" ||
			strings.TrimPrefix(candidate, "W/") == etag {
			return true
		}
	}

	return false
}

func setDocumentationSecurityHeaders(header http.Header) {
	header.Set("Content-Security-Policy", contentSecurity)
	header.Set("Cross-Origin-Opener-Policy", "same-origin")
	header.Set("Cross-Origin-Resource-Policy", "same-origin")
	header.Set(
		"Permissions-Policy",
		"camera=(), geolocation=(), microphone=(), payment=(), usb=()",
	)
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
	header.Set("X-Robots-Tag", "noindex, nofollow")
}
