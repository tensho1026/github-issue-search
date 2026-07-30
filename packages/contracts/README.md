# IssueScout API contracts

`openapi.yaml` is the source of truth for the public IssueScout HTTP API. Handler changes must update this document in the same pull request.

The issue discovery contract documents the strict JSON body, bounded
pagination, cache status header, exclusion diagnostics, and partial GitHub
search warning returned by `POST /api/issues/search`.

`fixtures/` contains deterministic success, empty, and error envelopes shared
by backend response decoding and frontend browser-boundary tests.
`fixtures/manifest.json` maps every document to its OpenAPI component schema.
Fixture JSON must contain no credentials or user-specific production data.

Run semantic validation, explicit status/envelope/request-ID policy, positive
and negative JSON Schema fixture validation, generated frontend type drift,
and Gin route drift from the repository root:

```sh
pnpm run contracts:check
```

The drift check compares every Gin route registered under `/api` with every
OpenAPI path. A difference fails closed in CI. `ajv` validates each fixture
against the referenced OpenAPI 3.1 schema and proves undocumented fields or
missing envelope metadata are rejected. Frontend types are generated from the
same contract and must never be edited manually.
