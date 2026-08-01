// Package github adapts the public GitHub REST and GraphQL APIs to IssueScout's
// application ports.
//
// The client validates all input before I/O, applies bounded pagination,
// concurrency, retry, and response-size policies, propagates context
// cancellation, and converts upstream payloads into domain-owned values.
// Returned errors expose only stable port classifications.
package github
