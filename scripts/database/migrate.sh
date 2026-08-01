#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
command="${1:-}"

case "${command}" in
  up | status | verify) ;;
  *)
    echo "Usage: scripts/database/migrate.sh <up|status|verify>" >&2
    exit 2
    ;;
esac

set -a
if [[ -f "${repository_root}/apps/api/.env" ]]; then
  # shellcheck disable=SC1091
  source "${repository_root}/apps/api/.env"
fi
set +a

exec go -C "${repository_root}/apps/api" run ./cmd/migrate "${command}"
