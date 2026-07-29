# Contributing to IssueScout

IssueScout uses small, issue-linked pull requests and automated quality gates. Do not place credentials, private GitHub content, or personal data in commits, fixtures, logs, or pull request descriptions.

## Before implementation

1. Open or select an English issue containing `Summary`, `Scope`, engineering requirements, acceptance criteria, and a test plan.
2. Branch from the latest `main` using `agent/issue-<number>-<short-name>` for Codex work or `feature/issue-<number>-<short-name>` for human work.
3. Confirm that the intended change belongs to one issue. Split unrelated behavior before coding.

## Development

Install the locked dependencies and create local environment files:

```sh
make install
cp apps/api/.env.example apps/api/.env
cp apps/web/.env.example apps/web/.env
```

Keep commits reviewable and use Conventional Commit subjects:

```text
feat(profile): aggregate repository languages
fix(api): preserve request cancellation
test(search): cover bounded pagination
ci(quality): enforce workflow policy
```

The allowed types are `build`, `chore`, `ci`, `docs`, `feat`, `fix`, `perf`, `refactor`, `revert`, and `test`. Keep the subject between 4 and 72 characters after the prefix.

## Architecture

- Browser code calls the shared API client; it must not call GitHub or duplicate backend scoring rules.
- HTTP handlers validate and map transport data; they must not perform upstream I/O or business analysis.
- Usecases depend on domain policies and ports, never concrete clients or caches.
- Domain packages remain independent of transport, configuration, frameworks, and adapters.
- External payloads are normalized in adapters before entering usecases.
- Every external loop, result set, response body, cache, and concurrency group has an explicit bound.

Run `pnpm run architecture:check` whenever a package moves between layers.

## Validation

The normal local gate is:

```sh
pnpm run check
```

Before opening a ready pull request, run the stricter checks relevant to the change:

```sh
pnpm run coverage:api
pnpm run coverage:web
pnpm run bundle:check
pnpm run contracts:lint
pnpm run lint:docs
pnpm run lint:workflows
pnpm run e2e
```

The quality budgets live in `config/quality-budgets.json`. Lowering a threshold requires explicit rationale and must not be used to hide a regression.

## Pull requests

Use the repository template. A ready pull request must:

- include `Closes #<issue>` or another supported GitHub closing keyword;
- list exact validation evidence;
- describe performance and security impact;
- complete every self-review checkbox;
- contain only conventional, focused commits;
- update OpenAPI and developer documentation when behavior changes.

The stable branch-protection statuses are `CI required` and `Security required`. Individual path-aware jobs may be skipped, but each aggregate status fails if any applicable job fails or is cancelled.

## Reviews and merges

Review architecture, correctness, bounds, cancellation, security, tests, accessibility, operations, and documentation. Resolve every actionable thread. Merge only when the branch is current, the required status succeeds, and the diff still matches the linked issue.
