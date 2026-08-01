// Package apidocs serves IssueScout's self-contained interactive API
// reference and the exact OpenAPI source embedded in release binaries.
//
// The package is isolated from application handlers and use cases. Its
// dependency on Swagger UI is injected as an http.Handler, so deployments can
// omit the routes without changing anonymous or authenticated API behavior.
package apidocs
