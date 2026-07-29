#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
artifact_directory="${GO_COVERAGE_DIRECTORY:-${repository_root}/artifacts/go}"
coverage_file="${artifact_directory}/coverage.out"
mkdir -p "${artifact_directory}"

go -C "${repository_root}/apps/api" test \
  -race \
  -covermode=atomic \
  -coverprofile="${coverage_file}" \
  ./...

coverage="$(
  go -C "${repository_root}/apps/api" tool cover -func="${coverage_file}" |
    awk '/^total:/ {gsub(/%/, "", $3); print $3}'
)"
required="$(
  node -e \
    'const budget = require(process.argv[1]); process.stdout.write(String(budget.coverage.apiStatements));' \
    "${repository_root}/config/quality-budgets.json"
)"

echo "API statement coverage: ${coverage}% (required ${required}%)"
awk \
  -v actual="${coverage}" \
  -v minimum="${required}" \
  'BEGIN { if (actual + 0 < minimum + 0) exit 1 }'
