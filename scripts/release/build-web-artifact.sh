#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 2 ]]; then
  echo "usage: build-web-artifact.sh <version> <output-directory>" >&2
  exit 64
fi

readonly version="$1"
readonly output_directory="$2"

if [[ ! "${version}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([+-][0-9A-Za-z.-]+)?$ ]]; then
  echo "version must use semantic version syntax prefixed by v" >&2
  exit 64
fi

if command -v gtar >/dev/null 2>&1; then
  readonly tar_command="gtar"
elif tar --version 2>/dev/null | grep -q "GNU tar"; then
  readonly tar_command="tar"
else
  echo "GNU tar is required for deterministic release archives." >&2
  exit 69
fi

readonly revision="${GITHUB_SHA:-$(git rev-parse HEAD)}"
source_epoch="$(git show --no-patch --format=%ct "${revision}")"
readonly source_epoch
source_date="$(git show --no-patch --format=%cI "${revision}")"
readonly source_date
readonly source_repository="${GITHUB_REPOSITORY:-tensho1026/github-issue-search}"
readonly artifact_name="issuescout-web-${version}"
temporary_directory="$(mktemp -d)"
readonly temporary_directory
readonly artifact_directory="${temporary_directory}/${artifact_name}"

cleanup() {
  rm -rf "${temporary_directory}"
}
trap cleanup EXIT

pnpm run build:web

mkdir -p "${artifact_directory}" "${output_directory}"
output_directory_absolute="$(cd "${output_directory}" && pwd)"
readonly output_directory_absolute
cp -R apps/web/dist/. "${artifact_directory}/"
cp LICENSE "${artifact_directory}/LICENSE"
jq -n \
  --arg commit "${revision}" \
  --arg created "${source_date}" \
  --arg kind "web" \
  --arg repository "${source_repository}" \
  --arg version "${version}" \
  '{
    schemaVersion: 1,
    kind: $kind,
    version: $version,
    commit: $commit,
    created: $created,
    repository: $repository,
    target: {
      format: "static-web-bundle"
    }
  }' >"${artifact_directory}/release-manifest.json"

(
  cd "${temporary_directory}"
  "${tar_command}" \
    --format=posix \
    --group=0 \
    --mode="u+rwX,go+rX,go-w" \
    --mtime="@${source_epoch}" \
    --numeric-owner \
    --owner=0 \
    --sort=name \
    --pax-option=delete=atime,delete=ctime \
    --create \
    "${artifact_name}" |
    gzip -n >"${output_directory_absolute}/${artifact_name}.tar.gz"
)

echo "Built deterministic web archive in ${output_directory}."
