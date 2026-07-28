#!/usr/bin/env bash
#
# commitlint.sh — lint a commit message inside the dev container.
#
# pre-commit passes the path to the message file. It is read here, on the host,
# and piped into the container: commitlint's own --edit resolves the path from
# git, and in a worktree that is an absolute path into the main checkout's
# .git directory, which the container has never seen.
set -euo pipefail

readonly MSG_FILE="${1:?usage: commitlint.sh <commit-msg-file>}"

# The suffix picks this checkout's container; see Taskfile.yml.
suffix=""
case "$PWD" in
    */.worktrees/*) suffix="-$(basename "$PWD")" ;;
esac

DEV_SUFFIX="$suffix" podman-compose \
    -f deployments/compose/compose.dev.yml exec -T dev \
    npx --no-install commitlint --config .commitlintrc.yaml < "$MSG_FILE"
