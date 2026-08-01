// Package issue defines transport-independent issue discovery, eligibility,
// deterministic analysis, effort, and recommendation rules.
//
// Functions in this package are pure and perform no I/O. They operate on
// bounded normalized inputs, preserve unknown evidence as unknown, and return
// stable values suitable for caching and HTTP presentation.
package issue
