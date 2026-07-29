#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 1 ]]; then
  echo "usage: smoke-endpoint.sh <https-health-url>" >&2
  exit 64
fi

readonly health_url="$1"

for _ in {1..24}; do
  if response="$(curl \
    --connect-timeout 5 \
    --fail \
    --max-time 10 \
    --show-error \
    --silent \
    "${health_url}")"; then
    if [[ "${response}" == *"ok"* ]]; then
      echo "Promoted release passed its environment health check."
      exit 0
    fi
  fi
  sleep 5
done

echo "Promoted release did not pass its environment health check." >&2
exit 1

