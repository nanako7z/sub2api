#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PORT="${MITM_PORT:-8080}"
ADDON="$ROOT/addon.py"

is_self_proxy() {
  local candidate="${1:-}"
  if [[ -z "$candidate" ]]; then
    return 1
  fi
  case "$candidate" in
    http://127.0.0.1:${PORT}|http://localhost:${PORT}|socks5://127.0.0.1:${PORT}|socks5://localhost:${PORT})
      return 0
      ;;
  esac
  return 1
}

detect_upstream_proxy() {
  local candidate=""
  for candidate in \
    "${UPSTREAM_PROXY:-}" \
    "${HTTPS_PROXY:-}" \
    "${https_proxy:-}" \
    "${HTTP_PROXY:-}" \
    "${http_proxy:-}" \
    "${ALL_PROXY:-}" \
    "${all_proxy:-}"
  do
    if [[ -n "$candidate" ]] && ! is_self_proxy "$candidate"; then
      printf '%s' "$candidate"
      return 0
    fi
  done
}

if ! command -v mitmdump >/dev/null 2>&1; then
  echo "mitmdump not found. Install dependencies from tools/mitm/requirements.txt" >&2
  exit 1
fi

if [[ $# -eq 0 ]]; then
  echo "Usage: $0 <command...>" >&2
  echo "Example: $0 claude" >&2
  exit 1
fi

MITM_ARGS=(-q -s "$ADDON" --listen-host 127.0.0.1 --listen-port "$PORT")
UPSTREAM_PROXY_VALUE="$(detect_upstream_proxy)"
if [[ -n "$UPSTREAM_PROXY_VALUE" ]]; then
  MITM_ARGS+=(--mode "upstream:${UPSTREAM_PROXY_VALUE}")
fi

echo "Starting mitmdump on 127.0.0.1:${PORT}"
if [[ -n "$UPSTREAM_PROXY_VALUE" ]]; then
  echo "Using upstream proxy: ${UPSTREAM_PROXY_VALUE}"
else
  echo "Using upstream proxy: direct"
fi
mitmdump "${MITM_ARGS[@]}" &
MITM_PID=$!
trap 'kill $MITM_PID 2>/dev/null || true' EXIT

sleep 1

export HTTPS_PROXY="http://127.0.0.1:${PORT}"
export HTTP_PROXY="http://127.0.0.1:${PORT}"
export ALL_PROXY="http://127.0.0.1:${PORT}"
export NODE_EXTRA_CA_CERTS="${NODE_EXTRA_CA_CERTS:-$HOME/.mitmproxy/mitmproxy-ca-cert.pem}"

exec "$@"
