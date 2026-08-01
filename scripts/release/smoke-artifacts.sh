#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 2 ]]; then
  echo "usage: smoke-artifacts.sh <artifact-directory> <version>" >&2
  exit 64
fi

readonly artifact_directory="$1"
readonly version="$2"
temporary_directory="$(mktemp -d)"
readonly temporary_directory
api_pid=""
web_pid=""

# The EXIT trap invokes this function indirectly.
# shellcheck disable=SC2317,SC2329
cleanup() {
  if [[ -n "${api_pid}" ]]; then
    kill "${api_pid}" >/dev/null 2>&1 || true
    wait "${api_pid}" >/dev/null 2>&1 || true
  fi
  if [[ -n "${web_pid}" ]]; then
    kill "${web_pid}" >/dev/null 2>&1 || true
    wait "${web_pid}" >/dev/null 2>&1 || true
  fi
  rm -rf "${temporary_directory}"
}
trap cleanup EXIT

case "$(uname -s)" in
  Darwin) readonly host_operating_system="darwin" ;;
  Linux) readonly host_operating_system="linux" ;;
  *)
    echo "artifact smoke test supports Darwin and Linux hosts" >&2
    exit 69
    ;;
esac

case "$(uname -m)" in
  aarch64 | arm64) readonly host_architecture="arm64" ;;
  x86_64) readonly host_architecture="amd64" ;;
  *)
    echo "artifact smoke test supports amd64 and arm64 hosts" >&2
    exit 69
    ;;
esac

readonly api_artifact_name="issuescout-api-${version}-${host_operating_system}-${host_architecture}"
tar -xzf \
  "${artifact_directory}/${api_artifact_name}.tar.gz" \
  -C "${temporary_directory}"
tar -xzf \
  "${artifact_directory}/issuescout-web-${version}.tar.gz" \
  -C "${temporary_directory}"

api_directory="${temporary_directory}/${api_artifact_name}"
web_directory="${temporary_directory}/issuescout-web-${version}"
chmod +x "${api_directory}/issuescout-api"

ALLOWED_ORIGINS="http://127.0.0.1:14173" \
  PORT="18080" \
  "${api_directory}/issuescout-api" \
  >"${temporary_directory}/api.log" 2>&1 &
api_pid="$!"

python3 -m http.server \
  14173 \
  --bind 127.0.0.1 \
  --directory "${web_directory}" \
  >"${temporary_directory}/web.log" 2>&1 &
web_pid="$!"

for _ in {1..30}; do
  api_headers="${temporary_directory}/api-headers"
  api_response="$(
    curl --fail --show-error --silent \
      --dump-header "${api_headers}" \
      --header "X-Request-ID: artifact-smoke-readiness" \
      "http://127.0.0.1:18080/api/health" 2>/dev/null || true
  )"
  documentation_headers="${temporary_directory}/documentation-headers"
  documentation_response="$(
    curl --fail --show-error --silent \
      --dump-header "${documentation_headers}" \
      "http://127.0.0.1:18080/docs/" 2>/dev/null || true
  )"
  openapi_headers="${temporary_directory}/openapi-headers"
  openapi_response="$(
    curl --fail --show-error --silent \
      --dump-header "${openapi_headers}" \
      "http://127.0.0.1:18080/openapi.yaml" 2>/dev/null || true
  )"
  swagger_stylesheet="$(
    curl --fail --show-error --silent \
      --header "Accept-Encoding: identity" \
      "http://127.0.0.1:18080/docs/swagger-ui.css" 2>/dev/null || true
  )"
  web_response="$(
    curl --fail --show-error --silent \
      "http://127.0.0.1:14173/" 2>/dev/null || true
  )"
  if [[ "${api_response}" == *'"status":"ok"'* ]] &&
    [[ "${api_response}" == *'"requestId":"artifact-smoke-readiness"'* ]] &&
    tr -d '\r' <"${api_headers}" |
      grep -Eiq '^x-request-id: artifact-smoke-readiness$' &&
    [[ "${documentation_response}" == *"<title>IssueScout API reference</title>"* ]] &&
    [[ "${documentation_response}" == *'src="/docs/issuescout-swagger.js"'* ]] &&
    tr -d '\r' <"${documentation_headers}" |
      grep -Eiq "^content-security-policy: .*script-src 'self'" &&
    [[ "${openapi_response}" == *"openapi: 3.1.0"* ]] &&
    [[ "${openapi_response}" == *"title: IssueScout API"* ]] &&
    tr -d '\r' <"${openapi_headers}" |
      grep -Eiq '^content-type: application/yaml; charset=utf-8$' &&
    tr -d '\r' <"${openapi_headers}" |
      grep -Eiq '^cache-control: public, max-age=300, must-revalidate$' &&
    tr -d '\r' <"${openapi_headers}" |
      grep -Eiq '^etag: \"[a-f0-9]{64}\"$' &&
    [[ "${swagger_stylesheet}" == *".swagger-ui"* ]] &&
    [[ "${web_response}" == *"<title>IssueScout</title>"* ]]; then
    kill "${api_pid}"
    set +e
    wait "${api_pid}"
    api_status="$?"
    set -e
    api_pid=""
    if [[ "${api_status}" -ne 0 ]] ||
      ! grep -q '"msg":"shutting down IssueScout API"' \
        "${temporary_directory}/api.log"; then
      cat "${temporary_directory}/api.log" >&2
      echo "release API did not shut down gracefully" >&2
      exit 1
    fi
    echo "Release API, embedded documentation, and web passed readiness, correlation, and graceful shutdown smoke tests."
    exit 0
  fi
  sleep 1
done

cat "${temporary_directory}/api.log" >&2
cat "${temporary_directory}/web.log" >&2
echo "release artifacts did not become healthy" >&2
exit 1
