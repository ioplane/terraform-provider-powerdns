#!/usr/bin/env bash
# Worktree helper. main is never committed to directly (AGENTS.md); this makes
# the isolated-branch workflow one command instead of four.
set -euo pipefail

readonly REPO_ROOT="$(git rev-parse --show-toplevel)"
readonly WORKTREE_DIR="${REPO_ROOT}/../.worktrees"
readonly BASE_REMOTE="origin"

usage() {
  cat <<'USAGE'
Usage:
  scripts/worktree.sh new <branch>   create a worktree cut from fork/main
  scripts/worktree.sh rm  <branch>   remove a worktree and delete the branch
  scripts/worktree.sh ls             list worktrees

Branch names follow docs/standards/naming-conventions.md §4, e.g.
  feat/dnssec/cryptokey-resource
  fix/zone/ipv6-masters
  sprint/S3-test-harness
USAGE
}

cmd_new() {
  local branch="$1"
  local path="${WORKTREE_DIR}/${branch}"

  git fetch "${BASE_REMOTE}" main
  mkdir -p "$(dirname "${path}")"
  git worktree add -b "${branch}" "${path}" "${BASE_REMOTE}/main"

  echo
  echo "worktree: ${path}"
  echo "cd ${path} && task up && task shell"
}

cmd_rm() {
  local branch="$1"
  local path="${WORKTREE_DIR}/${branch}"

  git worktree remove "${path}"
  git branch -D "${branch}" 2>/dev/null || true
  echo "removed ${branch}"
}

case "${1:-}" in
  new) [ $# -eq 2 ] || { usage; exit 2; }; cmd_new "$2" ;;
  rm)  [ $# -eq 2 ] || { usage; exit 2; }; cmd_rm  "$2" ;;
  ls)  git worktree list ;;
  *)   usage; exit 2 ;;
esac
