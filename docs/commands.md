# Command reference

Commands run from the repository root. Make provides the short contributor
surface; pnpm scripts expose every CI building block.

## Make targets

| Command             | Purpose                                           |
| ------------------- | ------------------------------------------------- |
| `make help`         | List supported Make targets                       |
| `make install`      | Install the exact pnpm lockfile                   |
| `make dev`          | Supervise the complete native stack               |
| `make dev-api`      | Run only the Go API and load its optional `.env`  |
| `make dev-web`      | Run only Vite                                     |
| `make dev-smoke`    | Run the deterministic healthy-stack smoke journey |
| `make format`       | Apply Prettier and gofmt                          |
| `make format-check` | Verify formatting                                 |
| `make lint`         | Run frontend, backend, and architecture lint      |
| `make typecheck`    | Run strict TypeScript                             |
| `make test`         | Run web, API, and native-supervisor tests         |
| `make build`        | Build frontend and API                            |
| `make check`        | Run the normal local quality gate                 |
| `make clean`        | Remove generated build and coverage output        |

## Root pnpm scripts

| Command                                              | Purpose                                              |
| ---------------------------------------------------- | ---------------------------------------------------- |
| `pnpm run architecture:check`                        | Enforce Go layer dependencies                        |
| `pnpm run build`                                     | Build both applications                              |
| `pnpm run build:api`                                 | Build a local API executable                         |
| `pnpm run build:web`                                 | Build static frontend assets                         |
| `pnpm run bundle:check`                              | Enforce frontend gzip budgets                        |
| `pnpm run check`                                     | Format, lint, typecheck, test, and build             |
| `pnpm run contracts:check`                           | Run the complete OpenAPI and generated-type contract |
| `pnpm run contracts:fixtures`                        | Validate positive and negative envelope fixtures     |
| `pnpm run contracts:lint`                            | Run Redocly semantic lint                            |
| `pnpm run contracts:policy`                          | Enforce statuses, envelopes, and request IDs         |
| `pnpm run coverage:api`                              | Run Go race tests and coverage threshold             |
| `pnpm run coverage:web`                              | Run Vitest coverage and thresholds                   |
| `pnpm run database:migrate`                          | Apply pending forward PostgreSQL migrations          |
| `pnpm run database:status`                           | Show safe migration status and checksums             |
| `pnpm run database:verify`                           | Require an up-to-date checksummed schema             |
| `pnpm run dev`                                       | Supervise native API and web development servers     |
| `pnpm run dev:api`                                   | Run only the API                                     |
| `pnpm run dev:smoke`                                 | Start, verify, and stop the deterministic stack      |
| `pnpm run dev:web`                                   | Run only Vite                                        |
| `pnpm run docs:check`                                | Check links and documentation/source coverage        |
| `pnpm run docs:go`                                   | Print full GoDoc for every API package               |
| `pnpm run docs:go:check`                             | Enforce package and exported declaration comments    |
| `pnpm run e2e`                                       | Build the stack and run Playwright                   |
| `pnpm run format`                                    | Apply repository formatters                          |
| `pnpm run format:check`                              | Verify repository formatting                         |
| `pnpm run fuzz:api`                                  | Run bounded Go fuzz smoke targets                    |
| `pnpm run lint`                                      | Run web, API, and architecture lint                  |
| `pnpm run lint:api`                                  | Run golangci-lint                                    |
| `pnpm run lint:docker-free`                          | Reject container files, workflows, and dependencies  |
| `pnpm run lint:docs`                                 | Lint Markdown and run the documentation contract     |
| `pnpm run lint:web`                                  | Run ESLint with zero warnings                        |
| `pnpm run lint:workflow-policy`                      | Enforce GitHub Actions safety policy                 |
| `pnpm run lint:workflows`                            | Run actionlint and workflow policy                   |
| `pnpm run migrations:check`                          | Enforce SQL and append-only migration policy         |
| `pnpm run performance:api`                           | Enforce Go latency/allocation budgets                |
| `pnpm run quality:strict`                            | Run the complete pre-PR local gate                   |
| `pnpm run release:build -- v0.1.0 artifacts/release` | Build, verify, and scan one archive set              |
| `pnpm run release:compare -- first second`           | Compare two release sets byte-for-byte               |
| `pnpm run release:reproducibility -- v0.1.0`         | Build twice, compare, and native-smoke-test          |
| `pnpm run release:smoke -- artifacts/release v0.1.0` | Smoke-test packaged host artifacts                   |
| `pnpm run test`                                      | Run web, API, and process-lifecycle tests            |
| `pnpm run test:api`                                  | Run all Go tests                                     |
| `pnpm run test:dev`                                  | Test native startup and shutdown behavior            |
| `pnpm run test:web`                                  | Run Vitest once                                      |
| `pnpm run typecheck`                                 | Run strict TypeScript without emit                   |

## Choosing a gate

- While editing, use the smallest feature test and `pnpm run format:check`.
- Before a commit, run affected lint, type, and tests.
- Before a ready PR, run `pnpm run quality:strict` and `pnpm run e2e`.
- For delivery changes, also run
  `pnpm run release:reproducibility -- v0.0.0-local`.

CI invokes these same repository commands. A command that exists only in a
workflow is a maintenance smell; move reusable logic into `scripts/`.
