# Repository discovery

Repository discovery finds public open-source repositories without cloning or
executing their code. It uses one bounded GitHub REST search, one bounded
GraphQL enrichment, explicit evidence states, deterministic rules, and an
in-memory cache. The anonymous route never reads or writes a database.

## Bounded request flow

```mermaid
sequenceDiagram
    participant Browser
    participant API
    participant Cache as TTL/LRU cache
    participant GitHub

    Browser->>API: POST /api/repositories/search
    API->>API: Strict JSON and bounded criteria
    API->>Cache: Canonical criteria key
    alt cache hit
        Cache-->>API: Deep-copied analyzed window
    else cache miss
        API->>GitHub: One REST search query (at most 50)
        GitHub-->>API: Normalized public candidates
        API->>API: Cheap filtering and stable shortlist
        API->>GitHub: One batched GraphQL enrichment (at most 20)
        GitHub-->>API: Bounded README and contribution files
        API->>API: Analyze, filter, rank
        API->>Cache: Store deep copy for five minutes
    end
    API-->>Browser: Page, evidence, warnings, rate limit
```

A cache miss makes at most two upstream requests. The first query obtains at
most 50 candidates. The second query enriches 10 repositories by default and
at most 20 when explicitly configured, in one batch; it is not a
per-repository fan-out. Pagination is applied after analysis and is excluded
from the canonical cache key.

## Filters and defaults

| Input              | Default     | Bound or accepted values            |
| ------------------ | ----------- | ----------------------------------- |
| Languages          | empty       | Up to 10 safe values                |
| Technologies       | empty       | Up to 10 safe values                |
| SPDX licenses      | empty       | Supported SPDX allowlist            |
| OSS categories     | empty       | Nine documented categories          |
| Minimum stars      | 10          | 0–10,000,000                        |
| Minimum forks      | 0           | 0–10,000,000                        |
| Open issues        | 0–unbounded | Each explicit bound is 0–10,000,000 |
| Updated within     | 365 days    | 1–3,650 days                        |
| Maximum difficulty | unset       | 1–5                                 |
| Minimum readiness  | 0           | Inclusive score from 0–100          |
| Japanese README    | unset       | `true` or `false`                   |
| Fork policy        | `exclude`   | `exclude`, `include`, `only`        |
| Exclude archived   | `true`      | `true` or `false`                   |
| Page / page size   | 1 / 20      | Page 1–50; page size 1–50           |

The supported OSS categories are `application`, `data`, `documentation`,
`education`, `framework`, `infrastructure`, `library`, `security`, and
`tooling`. Filter values reject control characters, quotes, backslashes,
excessive length, and excessive collections before GitHub I/O. SPDX
identifiers and categories use allowlists.

## Classification and evidence

Category classification follows one deterministic priority: security, data,
infrastructure, documentation, education, framework, library, tooling, then
application. Topics and bounded README text are matched as complete normalized
technology terms rather than arbitrary substrings.

Japanese README evidence reports:

- `detected` after at least 20 Japanese-script runes and a 5% share of letters;
- `high` confidence after 100 runes, a 20% share, and unsampled content;
- `medium` confidence for the remaining positive matches and clear negative
  matches;
- `unavailable` when GitHub did not expose README content.

README analysis is capped at 64 KiB. The API reports the inspected byte count
and whether the content was sampled.

Difficulty is an explainable 1–5 preliminary contribution estimate based on
repository size and contribution support signals. Readiness starts from a
fixed point budget and records every positive or negative reason. It maps to
`needs_work`, `promising`, or `ready`; it is not a guarantee that maintainers
will accept a contribution.

## Partial data and privacy

If the required search fails, the route returns a mapped request error. If the
optional enrichment batch fails, eligible repositories remain useful and
README-derived evidence becomes `unavailable` with a typed warning.
Cancellation and deadlines still stop the complete operation.

Only public normalized evidence enters responses or the bounded in-memory
cache. Tokens, raw upstream payloads, README text, and anonymous search history
are never returned or persisted.

Executable examples, including invalid requests, are in
[`http/repository-discovery.http`](../http/repository-discovery.http).
