// Command api composes and runs the IssueScout HTTP service.
//
// It is the production composition root: configuration, infrastructure
// adapters, bounded in-memory caches, use cases, HTTP routing, and graceful
// process shutdown are wired here. Domain and transport packages remain
// independently testable and must not import this command.
package main
