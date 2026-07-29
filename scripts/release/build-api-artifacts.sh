#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 2 ]]; then
  echo "usage: build-api-artifacts.sh <version> <output-directory>" >&2
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
temporary_directory="$(mktemp -d)"
readonly temporary_directory
readonly targets=(
  "darwin amd64"
  "darwin arm64"
  "linux amd64"
  "linux arm64"
  "windows amd64"
)

cleanup() {
  rm -rf "${temporary_directory}"
}
trap cleanup EXIT

mkdir -p "${output_directory}"
output_directory_absolute="$(cd "${output_directory}" && pwd)"
readonly output_directory_absolute

for target in "${targets[@]}"; do
  read -r goos goarch <<<"${target}"
  artifact_name="issuescout-api-${version}-${goos}-${goarch}"
  artifact_directory="${temporary_directory}/${artifact_name}"
  binary_name="issuescout-api"
  if [[ "${goos}" == "windows" ]]; then
    binary_name="${binary_name}.exe"
  fi

  mkdir -p "${artifact_directory}"
  CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" \
    go -C apps/api build \
    -buildvcs=false \
    -ldflags="-s -w -buildid= -X main.buildVersion=${version} -X main.buildCommit=${revision}" \
    -trimpath \
    -o "${artifact_directory}/${binary_name}" \
    ./cmd/api

  cp LICENSE "${artifact_directory}/LICENSE"
  jq -n \
    --arg architecture "${goarch}" \
    --arg commit "${revision}" \
    --arg created "${source_date}" \
    --arg kind "api" \
    --arg operatingSystem "${goos}" \
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
        operatingSystem: $operatingSystem,
        architecture: $architecture
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
done

echo "Built ${#targets[@]} deterministic API archives in ${output_directory}."
