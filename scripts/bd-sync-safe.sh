#!/usr/bin/env bash
set -euo pipefail

log() {
  printf '[bd-sync-safe] %s\n' "$*"
}

die() {
  printf '[bd-sync-safe] ERROR: %s\n' "$*" >&2
  exit 1
}

resolve_repo_root() {
  local cwd common_dir
  cwd="$(pwd)"
  common_dir="$(git -C "$cwd" rev-parse --git-common-dir 2>/dev/null || true)"
  if [[ -z "$common_dir" ]]; then
    die "not inside a git repository"
  fi
  if [[ "$common_dir" != /* ]]; then
    common_dir="$(cd "$cwd" && cd "$common_dir" && pwd)"
  fi
  if [[ "$(basename "$common_dir")" != ".git" ]]; then
    die "unexpected git common dir: $common_dir"
  fi
  dirname "$common_dir"
}

acquire_lock() {
  local lockdir="$1"
  local waited=0
  local max_wait=60
  while ! mkdir "$lockdir" 2>/dev/null; do
    sleep 1
    waited=$((waited + 1))
    if (( waited >= max_wait )); then
      die "timed out waiting for sync lock: $lockdir"
    fi
  done
}

recover_dolt() {
  log "attempting Dolt recovery"
  bd dolt stop --allow-stale >/dev/null 2>&1 || true
  rm -f "$BEADS_DIR/dolt-server.lock" \
        "$BEADS_DIR/dolt-server.pid" \
        "$BEADS_DIR/dolt-sql-server.pid"
  bd dolt start --allow-stale >/dev/null
}

checkpoint_server_working_set() {
  local port="13577"
  if [[ -f "$BEADS_DIR/dolt-server.port" ]]; then
    port="$(tr -d '[:space:]' < "$BEADS_DIR/dolt-server.port")"
  fi

  if ! command -v dolt >/dev/null 2>&1; then
    return 0
  fi

  dolt --no-tls --host 127.0.0.1 --port "$port" --use-db gromit sql -q "call dolt_add('.');" >/dev/null 2>&1 || true
  dolt --no-tls --host 127.0.0.1 --port "$port" --use-db gromit sql -q "call dolt_commit('-Am','bd-sync-safe checkpoint');" >/dev/null 2>&1 || true
}

safe_pull() {
  local out_file
  out_file="$(mktemp)"
  if bd dolt pull --allow-stale >"$out_file" 2>&1; then
    cat "$out_file"
    rm -f "$out_file"
    return 0
  fi

  cat "$out_file" >&2
  if grep -Eq "cannot merge with uncommitted changes|locked by another dolt process|could not find current branch commit" "$out_file"; then
    rm -f "$out_file"
    recover_dolt
    checkpoint_server_working_set
    bd dolt pull --allow-stale
    return 0
  fi

  rm -f "$out_file"
  return 1
}

main() {
  local repo_root lockdir
  repo_root="$(resolve_repo_root)"
  export BEADS_DIR="${BEADS_DIR:-$repo_root/.beads}"
  lockdir="$BEADS_DIR/dolt-sync.lock"

  acquire_lock "$lockdir"
  trap 'rmdir "$lockdir" >/dev/null 2>&1 || true' EXIT

  log "repo_root=$repo_root"
  log "BEADS_DIR=$BEADS_DIR"

  safe_pull
  bd dolt push --allow-stale
  log "sync complete"
}

main "$@"
