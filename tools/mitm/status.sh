#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PORT="${MITM_PORT:-8080}"
LOG_FILE="$ROOT/logs/mitmdump-${PORT}.log"
UPSTREAM_FILE="$ROOT/state/mitmdump-${PORT}.upstream"
SESSION_NAME="mitmdump-${PORT}"

if command -v tmux >/dev/null 2>&1 && tmux has-session -t "$SESSION_NAME" 2>/dev/null; then
  pid="$(tmux list-panes -t "$SESSION_NAME" -F '#{pane_pid}' 2>/dev/null | head -n1)"
  echo "running pid=${pid} port=${PORT}"
  echo "log=${LOG_FILE}"
  echo "tmux_session=${SESSION_NAME}"
  if [[ -f "$UPSTREAM_FILE" ]]; then
    upstream="$(cat "$UPSTREAM_FILE" 2>/dev/null || true)"
    if [[ -n "$upstream" ]]; then
      echo "upstream_proxy=${upstream}"
    else
      echo "upstream_proxy=direct"
    fi
  fi
  exit 0
fi

echo "stopped port=${PORT}"
