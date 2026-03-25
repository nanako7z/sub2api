#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PORT="${MITM_PORT:-8080}"
ADDON="$ROOT/addon.py"
STATE_DIR="$ROOT/state"
LOG_DIR="$ROOT/logs"
PID_FILE="$STATE_DIR/mitmdump-${PORT}.pid"
UPSTREAM_FILE="$STATE_DIR/mitmdump-${PORT}.upstream"
SESSION_FILE="$STATE_DIR/mitmdump-${PORT}.session"
LOG_FILE="$LOG_DIR/mitmdump-${PORT}.log"
SESSION_NAME="mitmdump-${PORT}"

mkdir -p "$STATE_DIR" "$LOG_DIR"

if ! command -v mitmdump >/dev/null 2>&1; then
  echo "mitmdump not found. Install dependencies from tools/mitm/requirements.txt" >&2
  exit 1
fi

if ! command -v tmux >/dev/null 2>&1; then
  echo "tmux not found" >&2
  exit 1
fi

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

read_scutil_proxy() {
  python3 - <<'PY'
import re
import subprocess
import sys

try:
    out = subprocess.check_output(["scutil", "--proxy"], text=True)
except Exception:
    sys.exit(0)

data = {}
for line in out.splitlines():
    m = re.match(r"\s*([A-Za-z0-9_]+)\s*:\s*(.+?)\s*$", line)
    if m:
        data[m.group(1)] = m.group(2)

def enabled(prefix):
    return data.get(prefix + "Enable") == "1"

def build(prefix, scheme):
    host = data.get(prefix + "Proxy")
    port = data.get(prefix + "Port")
    if host and port:
        return f"{scheme}://{host}:{port}"
    return ""

proxy = ""
if enabled("HTTPS"):
    proxy = build("HTTPS", "http")
elif enabled("HTTP"):
    proxy = build("HTTP", "http")
elif enabled("SOCKS"):
    proxy = build("SOCKS", "socks5")

print(proxy, end="")
PY
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

  candidate="$(read_scutil_proxy)"
  if [[ -n "$candidate" ]] && ! is_self_proxy "$candidate"; then
    printf '%s' "$candidate"
  fi
}

if [[ -f "$PID_FILE" ]]; then
  pid="$(cat "$PID_FILE" 2>/dev/null || true)"
  if [[ -n "${pid}" ]] && kill -0 "$pid" 2>/dev/null; then
    echo "mitmdump already running on port ${PORT} (pid ${pid})"
    exit 0
  fi
  rm -f "$PID_FILE"
fi

if tmux has-session -t "$SESSION_NAME" 2>/dev/null; then
  pane_pid="$(tmux list-panes -t "$SESSION_NAME" -F '#{pane_pid}' 2>/dev/null | head -n1)"
  echo "mitmdump already running in tmux session ${SESSION_NAME}${pane_pid:+ (pane_pid ${pane_pid})}"
  exit 0
fi

UPSTREAM_PROXY_VALUE="$(detect_upstream_proxy)"
MITM_ARGS=(-q -s "$ADDON" --listen-host 127.0.0.1 --listen-port "$PORT")

if [[ -n "$UPSTREAM_PROXY_VALUE" ]]; then
  MITM_ARGS+=(--mode "upstream:${UPSTREAM_PROXY_VALUE}")
fi

echo "Starting mitmdump on 127.0.0.1:${PORT}"
quoted_args=()
for arg in "${MITM_ARGS[@]}"; do
  quoted_args+=("$(printf '%q' "$arg")")
done
tmux new-session -d -s "$SESSION_NAME" "/bin/sh -lc 'exec mitmdump ${quoted_args[*]} >> $(printf '%q' "$LOG_FILE") 2>&1'"
sleep 1
if ! tmux has-session -t "$SESSION_NAME" 2>/dev/null; then
  echo "mitmdump failed to start in tmux; see $LOG_FILE" >&2
  exit 1
fi
pid="$(tmux list-panes -t "$SESSION_NAME" -F '#{pane_pid}' | head -n1)"
printf '%s' "$pid" >"$PID_FILE"
printf '%s' "$UPSTREAM_PROXY_VALUE" >"$UPSTREAM_FILE"
printf '%s' "$SESSION_NAME" >"$SESSION_FILE"

echo "mitmdump started"
echo "pid=${pid}"
echo "port=${PORT}"
echo "log=${LOG_FILE}"
echo "pid_file=${PID_FILE}"
echo "tmux_session=${SESSION_NAME}"
if [[ -n "$UPSTREAM_PROXY_VALUE" ]]; then
  echo "upstream_proxy=${UPSTREAM_PROXY_VALUE}"
else
  echo "upstream_proxy=direct"
fi
