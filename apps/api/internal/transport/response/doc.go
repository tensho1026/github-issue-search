// Package response writes IssueScout's uniform JSON success and error
// envelopes.
//
// Every envelope carries request correlation metadata. Unknown internal errors
// are reduced to a stable generic response; wrapped causes are never serialized
// or logged by this package.
package response
