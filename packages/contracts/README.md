# IssueScout API contracts

`openapi.yaml` is the source of truth for the public IssueScout HTTP API. Handler changes must update this document in the same pull request.

Run both semantic validation and route-drift detection from the repository root:

```sh
pnpm run contracts:check
```

The drift check compares every Gin route registered under `/api` with every OpenAPI path. A difference fails closed in CI. Generated frontend types will be added when the shared API client is introduced; generated files must never be edited manually.
