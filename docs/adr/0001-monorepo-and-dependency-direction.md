# ADR 0001: Monorepo and dependency direction

- Status: Accepted
- Date: 2026-07-30

## Context

The web, API, shared HTTP contract, automation, and documentation change
together but use different language toolchains. Uncontrolled sharing would
couple browser state to backend rules and adapters.

## Decision

Keep one pnpm/Go monorepo with independently buildable `apps/web` and
`apps/api`, cross-application contracts under `packages`, and executable
automation under `scripts`. Backend dependencies point transport to use cases
to domain/ports; concrete adapters point inward to ports. Composition alone
selects adapters.

## Consequences

One PR can atomically change contract and consumers. Root gates can enforce the
whole product. The architecture checker rejects forbidden Go imports. Shared
code is promoted only for a real second consumer, so the repository accepts
some intentional DTO mapping instead of hidden cross-layer coupling.
