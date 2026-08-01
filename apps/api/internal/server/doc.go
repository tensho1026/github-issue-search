// Package server owns the IssueScout HTTP server lifecycle.
//
// Run coordinates serving, process cancellation, bounded graceful shutdown,
// and forced close without coupling lifecycle behavior to application routing.
package server
