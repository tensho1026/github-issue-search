// Package middleware contains the ordered HTTP boundary policies shared by
// every IssueScout route.
//
// Middleware establishes request identity, secure response headers, CORS,
// structured logging, panic recovery, deadlines, authentication, and CSRF
// checks. Authentication middleware is applied only to account route groups so
// anonymous product routes never depend on PostgreSQL.
package middleware
