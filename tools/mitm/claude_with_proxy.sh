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
export https_proxy="$HTTPS_PROXY"
export http_proxy="$HTTP_PROXY"
export all_proxy="$ALL_PROXY"
unset NO_PROXY
unset no_proxy
export NODE_EXTRA_CA_CERTS="$CA_CERT"

exec "$@"
