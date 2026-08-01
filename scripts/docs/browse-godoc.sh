#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
api_root="${repository_root}/apps/api"
requested_package="${1:-}"

cd "${api_root}"

if [[ -n "${requested_package}" ]]; then
  go doc -all "${requested_package}"
  exit 0
fi

while IFS= read -r package_path; do
  go doc -all "${package_path}"
done < <(go list ./...)
