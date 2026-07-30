# ADR 0002: GitHub client boundary

- Status: Accepted
- Date: 2026-07-30

## Context

GitHub REST and GraphQL differ in transport shape, errors, pagination, partial
data, and rate limits. Letting handlers or domain rules consume them directly
would spread credential and retry behavior across the product.

## Decision

Use application-owned GitHub reader ports. Concrete real and deterministic
mock adapters implement those ports, bound every request and response, preserve
context, normalize errors, and return internal models. Handlers never call
GitHub, and the browser calls only IssueScout.

## Consequences

The server token has one trust boundary. Rules and use cases are deterministic
and network-free in tests. Adapter code performs explicit mapping and must
maintain complete retry, partial-data, size, cancellation, and rate-limit tests.
