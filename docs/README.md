# IssueScout engineering handbook

This directory is the operational and development source of truth for
IssueScout. Start with the product intent, run the deterministic mock journey,
then follow the topic guide that matches your change.

## First-hour learning path

1. Read [Product specification and glossary](product.md).
2. Complete [Getting started](getting-started.md).
3. Learn the [system architecture](architecture.md) and
   [API contract](api.md).
4. Use [Commands](commands.md) and [Testing](testing.md) while changing code.
5. Use [Troubleshooting](troubleshooting.md) when a local or CI check fails.
6. Read [Extension playbooks](extending.md) before adding a feature,
   upstream adapter, persistence, or a scoring rule.

## Knowledge map

| Concern                                  | Primary guide                           | Supporting material                           |
| ---------------------------------------- | --------------------------------------- | --------------------------------------------- |
| Product intent, vocabulary, journeys     | [Product](product.md)                   | [Frontend](frontend.md)                       |
| Local setup and native process lifecycle | [Getting started](getting-started.md)   | [Configuration](configuration.md)             |
| Monorepo and dependency boundaries       | [Architecture](architecture.md)         | [ADRs](adr/README.md)                         |
| HTTP endpoints and errors                | [API](api.md)                           | [OpenAPI](../packages/contracts/openapi.yaml) |
| Profile and OSS evidence                 | [Profile analysis](profile-analysis.md) | [API](api.md)                                 |
| Issue analysis and ranking               | [Rule analysis](issue-analysis.md)      | [Recommendations](issue-recommendations.md)   |
| Security and anonymous-data rules        | [Security](security.md)                 | [Configuration](configuration.md)             |
| Automated verification                   | [Testing](testing.md)                   | [CI](ci.md)                                   |
| Release and promotion                    | [Delivery](delivery.md)                 | [ADR 0005](adr/0005-docker-free-delivery.md)  |
| Logs and request correlation             | [Observability](observability.md)       | [Troubleshooting](troubleshooting.md)         |
| Safe product evolution                   | [Extension playbooks](extending.md)     | [Contributing](../CONTRIBUTING.md)            |

## Documentation contract

`pnpm run lint:docs` validates Markdown and then checks local links, root
commands, Make targets, environment variables, API paths, and error codes.
`pnpm run docs:check` runs only the executable documentation contract. A
behavioral change is incomplete until its guide and executable source agree.
