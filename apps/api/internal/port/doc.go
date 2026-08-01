// Package port declares the infrastructure boundaries required by IssueScout
// use cases.
//
// Interfaces are owned by their callers. Implementations must honor context
// cancellation, return domain-owned data, avoid retaining mutable caller
// slices, and classify external failures using the stable errors documented by
// each port.
package port
