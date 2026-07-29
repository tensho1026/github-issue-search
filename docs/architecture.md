# IssueScout architecture

## Goals

IssueScout is a stateless recommendation application. The browser talks only
to the IssueScout API; the API owns GitHub credentials, upstream rate-limit
handling, analysis, and response normalization.

```text
React web application
        |
        | HTTP / JSON
        v
Go API (Gin)
        |
        | REST / GraphQL
        v
GitHub APIs
```

The MVP does not persist user or issue data. Storage-facing ports still belong
in the domain boundary so a future Cloudflare D1 adapter can be added without
rewriting handlers or business rules.

## Monorepo boundaries

`apps/web` is an independently buildable React application. It owns browser
routing, accessible presentation, API query hooks, and client-side input
validation. It must not reproduce recommendation rules that belong to the API.

`apps/api` is an independently buildable Go module. Its packages follow this
dependency direction:

```text
transport/handler -> application/usecase -> domain
                              |             |
                              v             v
                       ports/interfaces <- policies
                              ^
                              |
             adapters (GitHub, cache, future D1)
```

Composition happens in `cmd/api` and `internal/router`. Transport packages may
map HTTP values but may not call GitHub directly or calculate scores. Usecases
accept `context.Context`, never Gin context. GitHub transport payloads are
converted to internal models before reaching usecases. Request DTOs, response
DTOs, domain models, and GitHub client models remain distinct even when their
fields initially look similar.

`packages` is reserved for versioned API contracts, generated clients, and
artifacts that have a real cross-application consumer. Shared code is promoted
there only after a second use case exists.

## Performance principles

- Bound every GitHub pagination loop and result set.
- Use cancellation-aware, bounded concurrency for repository inspection.
- Cache only immutable or short-lived upstream reads with explicit limits and
  expiry.
- Deduplicate GitHub requests within one analysis.
- Keep scoring pure and linear in the number of candidate issues.
- Return at most 50 recommendations and render only the initial documented
  result set.
- Track frontend bundle size and split route-level code when feature pages are
  introduced.

Search uses staged enrichment:

```text
GitHub search (up to 50 candidates)
  -> cheap eligibility checks and preliminary score
  -> select the top 20 candidates
  -> bounded repository and issue enrichment (default concurrency: 5)
  -> final scoring and deterministic ranking
```

Profile analysis inspects at most 20 repositories and three supported manifest
files per repository. The limits and concurrency are configuration values that
are validated once at startup.

External calls carry the inbound context and use a ten-second client timeout.
Only transient network failures and HTTP 502/503/504 responses are retried, at
most twice, with jittered exponential backoff. Authentication, validation,
not-found, permission, and rate-limit responses are never retried.

The bounded in-memory cache implements a port so a future adapter can replace
it. Initial TTLs are deliberately different by data volatility:

| Data               |        TTL |
| ------------------ | ---------: |
| GitHub user        | 10 minutes |
| Profile analysis   | 30 minutes |
| Issue search       |  5 minutes |
| Repository details | 15 minutes |

Partial enrichment failures return useful successful items plus typed warnings;
a missing user or a failed primary search remains a request-level error.

## API contract principles

- OpenAPI is the source of truth for endpoints, validation, examples, and error
  codes.
- Success responses use a `data` object plus `meta.requestId` and
  `meta.timestamp`.
- Failure responses use an `error` object and the same metadata.
- Paginated responses expose page, per-page, total, total-pages, and has-next.
- GitHub rate-limit information is normalized into optional response metadata.
- The frontend uses generated or contract-checked types rather than maintaining
  a second handwritten schema.

## Frontend state principles

- TanStack Query owns server state and request cancellation.
- React Router URL parameters own shareable search state.
- React Hook Form owns form state and schema-backed validation.
- Local component state owns transient UI behavior.
- Empty-before-search, no-results, not-found, rate-limited, upstream-error, and
  partial-analysis states are separate user experiences.
- Components call a shared typed API client; they never call `fetch` directly.

## Security principles

- GitHub tokens are server-only and never appear in API responses, browser
  bundles, logs, fixtures, or container layers.
- Upstream responses are normalized; the browser never receives arbitrary
  GitHub transport objects.
- Configuration is read from the environment and examples contain no secrets.
- CORS, timeouts, error mapping, and security headers are enforced at the API
  boundary in issue #2.
- Untrusted issue content is rendered without raw HTML execution.

## Quality principles

- Strict TypeScript and idiomatic, formatted Go are mandatory.
- Each feature includes focused tests; systemic contract and E2E coverage is
  completed by issue #11.
- Root commands are the local source of truth for CI.
- Refactoring follows passing characterization tests and preserves layer
  boundaries.
- Each pull request receives an explicit self-review for architecture,
  correctness, performance, security, test quality, accessibility, and
  operational impact before it is marked ready.
