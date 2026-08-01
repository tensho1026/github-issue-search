# Getting started

This guide takes a clean checkout to a healthy browser and API without Docker
or another container runtime.

## Prerequisites

- Git
- Node.js 22.22 or newer; Node 24 LTS is used by CI
- Corepack and pnpm 10.12.4
- Go matching `apps/api/go.mod`
- GNU tar (`tar` on Linux or `gtar` on macOS) only for release archives
- `curl`, `jq`, and Python 3 only for packaged release smoke tests

Enable the pinned package manager and install the lockfile:

```sh
corepack enable
corepack prepare --activate
make install
```

## Fast deterministic journey

No credential is needed to prove the local toolchain:

```sh
make dev-smoke
```

This starts the Go API and Vite web server as native child processes, waits for
both, verifies API health, `X-Request-ID` correlation, and a deterministic
profile-analysis request, then shuts both processes down. The GitHub mock can
run only with `APP_ENV=test`; startup rejects it in development, staging, and
production.

## Real development

Create local files and place any GitHub credential only in the ignored API
file:

```sh
cp apps/api/.env.example apps/api/.env
cp apps/web/.env.example apps/web/.env
```

Set `GITHUB_TOKEN` in `apps/api/.env`. A fine-grained read-only token that can
read public repository metadata is sufficient. Issue search uses GitHub
GraphQL, so it requires a token; public REST profile behavior can still start
without one.

Leave `DATABASE_URL` empty for the complete anonymous product. Optional account
work requires a rotated TLS PostgreSQL credential and the native migration
workflow in [Authenticated PostgreSQL persistence](database.md). Never reuse a
connection string that appeared in chat or another non-secret channel.

Start the complete stack with one command:

```sh
make dev
```

The supervisor loads the optional application `.env` files without printing
them, starts API and web process groups, and waits for:

- API `http://127.0.0.1:8080/api/health`;
- web `http://127.0.0.1:5173/`;
- an echoed request ID in both the health response header and envelope.

Press Ctrl-C once. The supervisor sends graceful termination to both process
trees, waits up to ten seconds, and force-stops only a child that exceeds the
deadline. Child cleanup is tested during startup and after readiness.

## Separate processes

For debugger attachment or focused frontend work:

```sh
make dev-api
make dev-web
```

These commands do not supervise one another. Prefer `make dev` for normal
full-stack work.

## Verify the checkout

```sh
make format-check
make lint
make typecheck
make test
make build
make check
```

Before a ready PR, run `pnpm run quality:strict` and `pnpm run e2e`. The
complete command catalog explains narrower checks.

## Build reproducible release archives

The following builds two independent archive sets from the current revision,
checks every archive and checksum, scans their extracted contents, compares
every byte, smoke-tests the host API and web artifacts, and removes temporary
builds:

```sh
pnpm run release:reproducibility v0.1.0
```

To retain one verified set:

```sh
pnpm run release:build v0.1.0 artifacts/release
pnpm run release:smoke artifacts/release v0.1.0
```

## Next steps

- [Configuration reference](configuration.md)
- [Commands](commands.md)
- [API guide](api.md)
- [Troubleshooting](troubleshooting.md)
