#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PORT="${MITM_PORT:-8080}"
PID_FILE="$ROOT/state/mitmdump-${PORT}.pid"
UPSTREAM_FILE="$ROOT/state/mitmdump-${PORT}.upstream"
SESSION_FILE="$ROOT/state/mitmdump-${PORT}.session"
SESSION_NAME="mitmdump-${PORT}"

if command -v tmux >/dev/null 2>&1 && tmux has-session -t "$SESSION_NAME" 2>/dev/null; then
  tmux kill-session -t "$SESSION_NAME"
  echo "Stopped mitmdump tmux session ${SESSION_NAME} on port ${PORT}"
else
  echo "No running tmux session for port ${PORT}"
fi

rm -f "$PID_FILE" "$UPSTREAM_FILE" "$SESSION_FILE"
