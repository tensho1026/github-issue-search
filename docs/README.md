# IssueScout engineering handbook

This directory is the operational and development source of truth for
IssueScout. Start with the product intent, run the deterministic mock journey,
then follow the topic guide that matches your change.

## First-hour learning path

1. Read [Product specification and glossary](product.md).
2. Complete [Getting started](getting-started.md).
3. Learn the [system architecture](architecture.md) and
   [API contract](api.md).
4. Open the [interactive API reference](api-reference.md) to browse or try the
   versioned contract.
5. Use the [HTTPYAC collection](httpyac.md) for repeatable endpoint probes.
6. Read [Optional GitHub authentication](authentication.md) before changing
   sessions or account ownership.
7. Read [Authenticated account workspace](account-workspace.md) before
   changing bookmarks, saved searches, preferences, export, or deletion.
8. Read [GoDoc and internal API contracts](godoc.md) before changing Go package
   boundaries or exported declarations.
9. Use [Commands](commands.md) and [Testing](testing.md) while changing code.
10. Use [Troubleshooting](troubleshooting.md) when a local or CI check fails.
11. Read [Extension playbooks](extending.md) before adding a feature,
    upstream adapter, persistence, or a scoring rule.

## Knowledge map

| Concern                                  | Primary guide                                   | Supporting material                          |
| ---------------------------------------- | ----------------------------------------------- | -------------------------------------------- |
| Product intent, vocabulary, journeys     | [Product](product.md)                           | [Frontend](frontend.md)                      |
| Local setup and native process lifecycle | [Getting started](getting-started.md)           | [Configuration](configuration.md)            |
| Monorepo and dependency boundaries       | [Architecture](architecture.md)                 | [ADRs](adr/README.md)                        |
| HTTP endpoints and errors                | [API](api.md)                                   | [Interactive reference](api-reference.md)    |
| Executable HTTP requests                 | [HTTPYAC](httpyac.md)                           | [API](api.md)                                |
| Profile and OSS evidence                 | [Profile analysis](profile-analysis.md)         | [API](api.md)                                |
| Repository discovery and readiness       | [Repository discovery](repository-discovery.md) | [API](api.md)                                |
| Issue analysis and ranking               | [Rule analysis](issue-analysis.md)              | [Recommendations](issue-recommendations.md)  |
| Security and anonymous-data rules        | [Security](security.md)                         | [Configuration](configuration.md)            |
| Optional OAuth and server sessions       | [Authentication](authentication.md)             | [Database](database.md)                      |
| Account-owned optional features          | [Account workspace](account-workspace.md)       | [Authentication](authentication.md)          |
| Authenticated PostgreSQL persistence     | [Database](database.md)                         | [Architecture](architecture.md)              |
| Go packages and internal contracts       | [GoDoc](godoc.md)                               | [Architecture](architecture.md)              |
| Automated verification                   | [Testing](testing.md)                           | [CI](ci.md)                                  |
| Release and promotion                    | [Delivery](delivery.md)                         | [ADR 0005](adr/0005-docker-free-delivery.md) |
| Logs and request correlation             | [Observability](observability.md)               | [Troubleshooting](troubleshooting.md)        |
| Safe product evolution                   | [Extension playbooks](extending.md)             | [Contributing](../CONTRIBUTING.md)           |

## Documentation contract

`pnpm run lint:docs` validates Markdown and then checks local links, Mermaid
syntax, root commands, Make targets, environment variables, API paths, and
error codes. `pnpm run docs:check` runs only the executable documentation
contract. `pnpm run http:check` additionally maps and safely executes all
HTTPYAC requests. A behavioral change is incomplete until its guide and
executable source agree.
