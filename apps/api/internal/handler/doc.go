// Package handler translates validated HTTP input into IssueScout use-case
// calls and converts application output into the shared response envelope.
//
// Handlers contain no persistence or GitHub client logic. They preserve
// request cancellation, apply explicit request-size limits, and delegate safe
// error rendering to the response package.
package handler
