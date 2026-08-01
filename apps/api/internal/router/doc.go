// Package router composes IssueScout HTTP middleware and route groups from
// explicit application dependencies.
//
// New validates required dependencies before exposing a handler. Optional
// authentication and account routes degrade independently from anonymous
// profile, repository, and issue features.
package router
