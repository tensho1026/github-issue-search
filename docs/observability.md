# Observability and request tracing

IssueScout uses structured process logs and HTTP request correlation. It does
not collect anonymous product analytics or persist request history.

## Structured logs

The API writes one JSON object per line to standard output. Platform log
collection may ingest that stream without an application-specific agent.

Request completion records include:

- timestamp and level;
- normalized route path, never a raw query string;
- method and HTTP status;
- latency in milliseconds;
- request ID;
- nonnegative response bytes;
- `HIT` or `MISS` for cache-aware success responses;
- stable application error code when present.

The request logger uses Gin's normalized route and never includes the query
string. It deliberately omits user agent and client address as unnecessary
identifying metadata. OAuth callback codes, state, provider descriptions,
Cookie headers, CSRF headers, authorization headers, and database parameters
are therefore excluded. Authentication dependencies return fixed safe errors
rather than driver or upstream response details.

Startup records include listener address and build version/commit. Shutdown
records indicate graceful termination. One GitHub completion event records
only the fixed service and operation names, fixed outcome, attempts, latency,
optional request ID, and status. It excludes URLs, usernames, repositories,
issue numbers, tokens, bodies, and query values.

Fixed GitHub operations are `user.get`, `profile.analyze`, `repository.list`,
`repository.search`, `repository.enrich`, `issue.search`, and `issue.detail`.
Fixed outcomes are success, not found, rate limited, unauthorized, cancelled,
deadline exceeded, transport error, and response error.

## Trace a request

Send a caller-owned safe identifier:

```sh
curl --fail-with-body \
  --header 'X-Request-ID: local-diagnosis-001' \
  http://127.0.0.1:8080/api/health
```

Confirm the same value in:

1. the `X-Request-ID` response header;
2. `meta.requestId` in JSON;
3. the API request-completion log.

Use that identifier when reporting an error. Do not paste a token, database
URL, OAuth code, or raw private payload into a request ID.

## Health and readiness

`GET /api/health` proves that configuration was valid, the listener is serving,
middleware is active, and response correlation works. It performs no GitHub or
database operation, so an upstream incident does not incorrectly mark the
process dead.

`GET /api/health/database` separately pings optional authenticated account
storage with the configured query deadline. It returns `DATABASE_UNAVAILABLE`
without driver details when storage is absent or unhealthy. Do not use this
endpoint as the liveness gate for anonymous traffic.

The native supervisor and packaged-artifact smoke test validate health plus the
web root. Promotion health checks use a public HTTPS endpoint supplied by the
deployment environment.

## Metrics semantics

Repository and maintainer values in recommendation responses are product
evidence, not service telemetry. Each bounded metric includes sample size,
window, truncation, availability, and confidence. Never aggregate it into
hidden user tracking.

Platform-derived service metrics should use low-cardinality route templates,
status/error codes, cache status, fixed GitHub operation, and fixed outcome.
Usernames, repositories, issue numbers, request IDs, tokens, URLs, and raw
query values must not become metric labels.

## Incident evidence

Preserve the failed workflow URL, request ID, release checksum, environment,
safe error code, and relevant timestamps. Redact secrets before attaching
logs. Release and promotion artifacts retain immutable manifests so an
operator can distinguish source, build, and deployed version.
