// Package config loads and validates IssueScout process configuration from
// environment variables.
//
// Load applies secure, bounded defaults and rejects internally inconsistent
// authentication or database settings before listeners or outbound clients are
// created. Secret values use redacting types and must never be logged.
package config
