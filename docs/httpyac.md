# HTTPYAC executable API collection

The tracked files under `http/` are an operator-friendly companion to the
OpenAPI contract. They cover every HTTP operation, important validation and
security failures, cache repeats, Swagger delivery, and optional account
workflows. They are also parsed and executed in CI.

## Collection map

| File                        | Capability and boundaries                                       |
| --------------------------- | --------------------------------------------------------------- |
| `system-health.http`        | Process health, database readiness, untrusted origin            |
| `profiles.http`             | Public profile, bounded analysis, cache repeat, invalid login   |
| `repository-discovery.http` | Defaults, filters, fork/Japanese README paths, invalid filters  |
| `issue-search.http`         | Defaults, pagination, effort filters, cache, malformed input    |
| `issue-detail.http`         | Explainable recommendation, skills, cache, invalid reference    |
| `authentication.http`       | Optional OAuth/session lifecycle, CORS and credential failures  |
| `account-workspace.http`    | Bookmarks, saved searches, preferences, export, deletion safety |
| `api-documentation.http`    | Self-contained Swagger, source OpenAPI, unknown asset           |

Every request region has a human title and a globally unique `# @name`.
`[negative]` titles identify intentional failure probes.

## Install and run

HTTPYAC is pinned in the root lockfile, so no global installation is needed:

```sh
pnpm install --frozen-lockfile
make dev
pnpm run http:run -- http/system-health.http --all --env local
```

Run one named request while developing:

```sh
pnpm run http:run -- \
  http/issue-search.http \
  --name searchIssuesDefault \
  --env local \
  --output response
```

Run all anonymous capabilities against the local API:

```sh
pnpm run http:run -- \
  http/system-health.http \
  http/profiles.http \
  http/repository-discovery.http \
  http/issue-search.http \
  http/issue-detail.http \
  http/api-documentation.http \
  --all \
  --env local \
  --bail
```

Live GitHub search and detail requests require the server-only `GITHUB_TOKEN`
in the ignored `apps/api/.env`. HTTPYAC never receives that token.

## Environments and secrets

`http/http-client.env.json` is tracked and contains only safe examples:

- `local` points at the native loopback stack;
- `deployed` uses non-routable replacement hostnames;
- `ci` contains inert values that are overridden by the validation harness.

Do not replace tracked placeholders with a real Cookie, OAuth code, state,
CSRF value, account UUID, or deployment URL containing credentials. Put fresh
local-only overrides in the ignored file:

```json
{
  "local": {
    "sessionToken": "fresh-session-cookie-value",
    "csrfToken": "fresh-session-response-value",
    "bookmarkID": "owned-bookmark-uuid",
    "bookmarkVersion": 1,
    "savedSearchID": "owned-saved-search-uuid",
    "savedSearchVersion": 1,
    "preferencesVersion": 0
  }
}
```

The filename must be `http/http-client.private.env.json`. Confirm it is ignored
before adding any value:

```sh
git check-ignore http/http-client.private.env.json
```

OAuth callback codes, state values, and flow cookies are single-use and
short-lived. Copy them only for a focused local diagnostic, then remove the
private file. Never paste the file into an issue, pull request, terminal log,
or chat.

## Optional authenticated workflows

Start with `getAuthSession`. When authentication is disabled it safely proves
`configured=false` without touching PostgreSQL. After a local browser login,
copy the current session Cookie and the session response's `csrfToken` into
the private environment, then execute individual account requests by name.

Account requests can create, update, or delete account-owned rows. Do not run
`account-workspace.http --all` against an account whose data matters. The
account-deletion body is deliberately invalid in the tracked environments;
replace `accountDeleteConfirmation` with `DELETE` only after export and only
for a disposable account.

OAuth start and callback requests opt out of redirect following so that an
operator can inspect the exact fixed `Location` and `Set-Cookie` headers.

## Automated contract

Run the same executable collection policy used by CI:

```sh
pnpm run http:check
```

The checker:

1. maps request methods and path templates to every OpenAPI operation;
2. requires unique titles/names and shared `apiBaseUrl` configuration;
3. requires negative probes for core validation and authorization boundaries;
4. rejects credential shapes and literal Cookie values;
5. proves anonymous routes contain no Cookie, authorization, or CSRF header;
6. validates the safe `local`, `deployed`, and `ci` profiles;
7. executes every request through the pinned HTTPYAC parser against an
   ephemeral loopback-only catch-all server.

The loopback execution validates collection syntax and interpolation without
calling GitHub, PostgreSQL, OAuth, or a developer process. `pnpm run
contracts:check` includes this gate, so a new OpenAPI operation without an
HTTPYAC request fails both local strict quality and CI.

When an endpoint changes, update the OpenAPI source, handler/tests, and the
owning `.http` file in the same pull request. Add a negative request when the
change introduces a new validator, authentication boundary, or destructive
operation.
