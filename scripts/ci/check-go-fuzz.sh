#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

run_fuzz_target() {
  local package_path="$1"
  local target="$2"
  go -C "${repository_root}/apps/api" test \
    -run '^$' \
    -fuzz "^${target}$" \
    -fuzztime=1s \
    "${package_path}"
}

run_fuzz_target ./internal/domain/user FuzzParseUsername
run_fuzz_target ./internal/domain/issue FuzzParseFilterValue
run_fuzz_target ./internal/domain/issue FuzzNewReference
