#!/usr/bin/env bash
#
# guard-main-branch.sh — refuse a commit or push on main.
#
# AGENTS.md has said "main is never committed to directly" since phase 0.
# Phases 0 to 4 were committed straight to main anyway, fourteen times, because
# a rule that lives only in a document is a rule that gets forgotten at the
# moment it applies.
#
# This is that rule as a mechanism. It reads the tool call Claude Code is about
# to make and blocks it when the branch is main.
#
# Exit 0 allows; exit 2 blocks and returns the message on stderr to the model.
set -uo pipefail

payload="$(cat)"

command="$(printf '%s' "$payload" | jq -r '.tool_input.command // empty')"
[[ -z "$command" ]] && exit 0

# Only git commit and git push are interesting. Anything else on main is fine.
case "$command" in
    *"git commit"*|*"git push"*) ;;
    *) exit 0 ;;
esac

cwd="$(printf '%s' "$payload" | jq -r '.cwd // empty')"
[[ -n "$cwd" && -d "$cwd" ]] && cd "$cwd" || exit 0

git rev-parse --git-dir >/dev/null 2>&1 || exit 0

branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null)"
[[ "$branch" != "main" ]] && exit 0

# A merge landing on main through gh is the sanctioned path, not a direct
# commit. So is a fast-forward pull.
case "$command" in
    *"gh pr merge"*|*"git pull"*|*"--ff-only"*) exit 0 ;;
esac

cat >&2 <<'MESSAGE'
Blocked: this is main, and AGENTS.md says main is never committed to directly.

One worktree per sprint, one pull request per sprint:

  scripts/worktree.sh new sprint/<phase>-<name>
  cd ../.worktrees/sprint/<phase>-<name>
  # work, task all, task verify
  gh pr create --fill
  gh pr merge --squash --delete-branch
  scripts/worktree.sh rm sprint/<phase>-<name>

If this really has to land on main — a merge through gh, or a fast-forward
pull — those are already allowed and this would not have fired.
MESSAGE
exit 2
