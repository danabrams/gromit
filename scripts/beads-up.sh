#!/usr/bin/env bash
set -euo pipefail

HOST="${BEADS_DOLT_HOST:-127.0.0.1}"
PORT="${BEADS_DOLT_PORT:-3307}"
LOG_FILE="${BEADS_DOLT_LOG:-/tmp/gromit-dolt.log}"
DATA_DIR="${BEADS_DOLT_DATA_DIR:-$(pwd)}"
DOLTCFG_DIR="${BEADS_DOLTCFG_DIR:-${DATA_DIR}/.doltcfg}"

usage() {
  cat <<EOF
Usage: $(basename "$0") [start|status|stop|restart]

Environment overrides:
  BEADS_DOLT_HOST   Dolt host (default: 127.0.0.1)
  BEADS_DOLT_PORT   Dolt port (default: 3307)
  BEADS_DOLT_LOG    Dolt log file (default: /tmp/gromit-dolt.log)
  BEADS_DOLT_DATA_DIR Dolt data dir (default: current directory)
  BEADS_DOLTCFG_DIR Dolt config dir (default: <repo>/.doltcfg)
EOF
}

is_running() {
  pgrep -f "dolt .*sql-server.*--host ${HOST}.*--port ${PORT}" >/dev/null 2>&1
}

start_server() {
  if is_running; then
    echo "Dolt already running on ${HOST}:${PORT}"
    return 0
  fi

  nohup dolt \
    --data-dir "${DATA_DIR}" \
    --doltcfg-dir "${DOLTCFG_DIR}" \
    sql-server --host "${HOST}" --port "${PORT}" >"${LOG_FILE}" 2>&1 &
  sleep 1

  if is_running; then
    echo "Dolt started on ${HOST}:${PORT}"
    echo "Log: ${LOG_FILE}"
    return 0
  fi

  echo "Failed to start Dolt. Check log: ${LOG_FILE}" >&2
  return 1
}

status_server() {
  if is_running; then
    echo "Dolt is running on ${HOST}:${PORT}"
  else
    echo "Dolt is not running on ${HOST}:${PORT}"
    return 1
  fi
}

stop_server() {
  if ! is_running; then
    echo "Dolt is not running on ${HOST}:${PORT}"
    return 0
  fi

  pkill -f "dolt .*sql-server.*--host ${HOST}.*--port ${PORT}"
  echo "Dolt stopped on ${HOST}:${PORT}"
}

cmd="${1:-start}"
case "${cmd}" in
  start)
    start_server
    ;;
  status)
    status_server
    ;;
  stop)
    stop_server
    ;;
  restart)
    stop_server || true
    start_server
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    echo "Unknown command: ${cmd}" >&2
    usage
    exit 2
    ;;
esac
