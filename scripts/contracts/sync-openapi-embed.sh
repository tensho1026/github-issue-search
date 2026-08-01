#!/usr/bin/env bash
set -euo pipefail

script_directory="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly script_directory
repository_root="$(cd "${script_directory}/../.." && pwd)"
readonly repository_root
readonly source_path="${repository_root}/packages/contracts/openapi.yaml"
readonly embedded_path="${repository_root}/apps/api/internal/documentation/openapi.yaml"

if [[ "${1:-}" == "--check" ]]; then
  if ! cmp --silent "${source_path}" "${embedded_path}"; then
    echo "embedded OpenAPI is stale; run: pnpm run contracts:embed" >&2
    exit 1
  fi
  echo "Embedded OpenAPI matches the single contract source."
  exit 0
fi

if [[ "$#" -ne 0 ]]; then
  echo "usage: sync-openapi-embed.sh [--check]" >&2
  exit 64
fi

cp "${source_path}" "${embedded_path}"
echo "Synchronized packages/contracts/openapi.yaml into the API binary."
