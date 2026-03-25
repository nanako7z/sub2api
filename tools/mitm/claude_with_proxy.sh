#!/usr/bin/env bash
set -euo pipefail

PORT="${MITM_PORT:-8080}"
CA_CERT="${NODE_EXTRA_CA_CERTS:-$HOME/.mitmproxy/mitmproxy-ca-cert.pem}"

if [[ $# -eq 0 ]]; then
  set -- claude
fi

export HTTPS_PROXY="http://127.0.0.1:${PORT}"
export HTTP_PROXY="http://127.0.0.1:${PORT}"
export ALL_PROXY="http://127.0.0.1:${PORT}"
export NODE_EXTRA_CA_CERTS="$CA_CERT"

exec "$@"
