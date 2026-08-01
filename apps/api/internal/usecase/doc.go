// Package usecase coordinates IssueScout domain rules and infrastructure ports.
//
// Use cases validate construction dependencies, propagate request contexts,
// bound external fan-out, preserve partial-result metadata, and map adapter
// failures into stable application errors. They do not depend on HTTP or
// concrete persistence packages.
package usecase
