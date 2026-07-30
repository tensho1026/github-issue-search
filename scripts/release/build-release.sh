#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 2 ]]; then
  echo "usage: build-release.sh <version> <output-directory>" >&2
  exit 64
fi

readonly version="$1"
readonly output_directory="$2"

bash scripts/release/build-api-artifacts.sh "${version}" "${output_directory}"
bash scripts/release/build-web-artifact.sh "${version}" "${output_directory}"
bash scripts/release/write-checksums.sh "${output_directory}"
node scripts/release/verify-artifacts.mjs "${output_directory}" "${version}"
node scripts/release/check-artifact-secrets.mjs "${output_directory}"
