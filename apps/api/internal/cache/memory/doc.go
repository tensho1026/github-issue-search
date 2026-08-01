// Package memory provides bounded, concurrency-safe, process-local cache
// adapters for public GitHub analysis results.
//
// Entries are copied at the port boundary, expire lazily by TTL, and are
// evicted by least-recently-used order. Cancellation is checked before cache
// work; no cache operation performs network or database I/O.
package memory
