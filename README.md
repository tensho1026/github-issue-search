# IssueScout

IssueScout helps developers find open-source issues they can realistically
complete. It compares a GitHub user's public technology profile with issue
requirements, estimated effort, and repository health.

The MVP is intentionally stateless: the API reads public GitHub data on demand
and may cache it in memory, but it does not require a database or GitHub OAuth.

## Repository layout

```text
.
├── apps/
│   ├── api/              # Go + Gin HTTP API
│   └── web/              # React + TypeScript web application
├── docs/                 # Architecture and engineering decisions
├── packages/             # Shared contracts and generated artifacts (later issues)
├── go.work               # Go workspace
├── package.json          # Cross-application commands
└── pnpm-workspace.yaml   # JavaScript workspace
```

See [the architecture guide](docs/architecture.md) for dependency rules and
future extension seams.

## Prerequisites

- Node.js 22.12 or newer (Node 24 is recommended)
- pnpm 10
- Go 1.25
- A GitHub personal access token is optional for the foundation and will be
  documented with the profile/search APIs

The repository pins the pnpm release in `package.json`. Corepack can provision
that version:

```sh
corepack enable
corepack prepare --activate
```

## Setup

```sh
make install
cp apps/api/.env.example apps/api/.env
cp apps/web/.env.example apps/web/.env
```

The Makefile loads `apps/api/.env` for the local API process. Vite loads the
web environment file. Start the applications in separate terminals:

```sh
make dev-api
make dev-web
```

- Web: `http://127.0.0.1:5173`
- API: `http://127.0.0.1:8080`

## Quality commands

```sh
make format-check
make lint
make typecheck
make test
make build
```

`make check` runs the complete local gate in the same order expected by CI.
Generated output is ignored and can be removed with `make clean`.

Performance-sensitive code is developed against explicit limits: at most 20
repositories per profile, 50 search candidates, 20 detailed candidates, three
manifest reads per repository, and five concurrent GitHub requests by default.
See the architecture guide for the staged analysis and caching model.

## Current status

The repository is being delivered through the scoped GitHub issues in the
[MVP implementation backlog](https://github.com/tensho1026/github-issue-search/issues).
The foundation intentionally contains only the application entry points; API
features, recommendation logic, production UI, tests, and delivery workflows
are added on their own issue-linked branches.
