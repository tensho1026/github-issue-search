#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 1 ]]; then
  echo "usage: write-checksums.sh <artifact-directory>" >&2
  exit 64
fi

readonly artifact_directory="$1"
readonly checksum_file="${artifact_directory}/SHA256SUMS"

if [[ ! -d "${artifact_directory}" ]]; then
  echo "artifact directory does not exist: ${artifact_directory}" >&2
  exit 66
fi

: >"${checksum_file}"
while IFS= read -r archive; do
  archive_name="${archive#./}"
  if command -v sha256sum >/dev/null 2>&1; then
    digest="$(cd "${artifact_directory}" && sha256sum "${archive_name}")"
  else
    digest="$(cd "${artifact_directory}" && shasum -a 256 "${archive_name}")"
  fi
  printf '%s\n' "${digest}" >>"${checksum_file}"
done < <(
  cd "${artifact_directory}"
  find . -maxdepth 1 -type f -name '*.tar.gz' -print | LC_ALL=C sort
)

if [[ ! -s "${checksum_file}" ]]; then
  echo "no release archives were found" >&2
  exit 66
fi

echo "Wrote checksums to ${checksum_file}."

