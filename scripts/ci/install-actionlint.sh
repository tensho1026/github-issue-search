#!/usr/bin/env bash
set -euo pipefail

readonly actionlint_version="1.7.12"
readonly archive="actionlint_${actionlint_version}_linux_amd64.tar.gz"
readonly expected_sha256="8aca8db96f1b94770f1b0d72b6dddcb1ebb8123cb3712530b08cc387b349a3d8"
readonly download_url="https://github.com/rhysd/actionlint/releases/download/v${actionlint_version}/${archive}"
readonly install_directory="${ACTIONLINT_INSTALL_DIRECTORY:-${HOME}/.local/bin}"

temporary_directory="$(mktemp -d)"
trap 'rm -rf "${temporary_directory}"' EXIT

curl \
  --fail \
  --location \
  --proto '=https' \
  --retry 3 \
  --show-error \
  --silent \
  --tlsv1.2 \
  --output "${temporary_directory}/${archive}" \
  "${download_url}"

(
  cd "${temporary_directory}"
  echo "${expected_sha256}  ${archive}" | sha256sum --check --strict
  tar --extract --gzip --file "${archive}" actionlint
)

mkdir -p "${install_directory}"
install -m 0755 \
  "${temporary_directory}/actionlint" \
  "${install_directory}/actionlint"

if [[ -n "${GITHUB_PATH:-}" ]]; then
  echo "${install_directory}" >>"${GITHUB_PATH}"
fi

echo "Installed actionlint ${actionlint_version} to ${install_directory}."
