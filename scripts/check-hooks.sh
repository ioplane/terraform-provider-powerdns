#!/usr/bin/env bash
#
# check-hooks.sh — assert the commit-msg hook is installed and enforcing.
#
# Golden rule 6 in AGENTS.md forbids AI attribution in commits. That ban is
# only real if the hook that carries it is installed and rejects a message
# that violates it. A configuration file committed to the repository proves
# neither: `.git/hooks` is not tracked, so a fresh clone starts unprotected.
#
# This checks both halves — installed, and observed to fail on input that must
# fail. A gate never seen to reject is not known to work.
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
readonly REPO_ROOT
cd "$REPO_ROOT"

fail() {
  printf 'check-hooks: %s\n' "$*" >&2
  exit 1
}

# --git-common-dir, not ".git": in a worktree .git is a file pointing at the
# main checkout, and the hooks live with the main checkout. Looking in ".git"
# reports every worktree as unprotected.
GIT_DIR="$(git rev-parse --git-common-dir)"
readonly GIT_DIR

for hook in commit-msg pre-commit; do
  [[ -f "${GIT_DIR}/hooks/${hook}" ]] ||
    fail "${hook} hook is not installed — run: task hooks"
done

TMP="$(mktemp -d)"
readonly TMP
trap 'rm -rf "$TMP"' EXIT

# Must be rejected.
printf 'feat(x): a change\n\nCo-Authored-By: Claude <noreply@anthropic.com>\n' >"$TMP/bad"
if scripts/check-no-ai-attribution.sh "$TMP/bad" >/dev/null 2>&1; then
  fail "the AI-attribution check accepted a message carrying an AI trailer"
fi

# Must be accepted. A checker that rejects everything is equally broken, and
# only testing the rejection would not notice.
printf 'feat(x): a change\n\nAn ordinary body naming the reason.\n' >"$TMP/good"
if ! scripts/check-no-ai-attribution.sh "$TMP/good" >/dev/null 2>&1; then
  fail "the AI-attribution check rejected an ordinary message"
fi

# Must also be accepted: a path to the tool directory this repository carries.
# The rule bans a claim of authorship, not the letters, and a repository has to
# be able to describe its own contents.
printf 'feat(x): a change\n\nAdds .claude/hooks/guard-main-branch.sh.\n' >"$TMP/path"
if ! scripts/check-no-ai-attribution.sh "$TMP/path" >/dev/null 2>&1; then
  fail "the AI-attribution check rejected a path under the tool directory"
fi

# And a claim in prose must still be caught, whatever shape it takes.
printf 'feat(x): a change\n\nThis change was generated with an AI.\n' >"$TMP/claim"
if scripts/check-no-ai-attribution.sh "$TMP/claim" >/dev/null 2>&1; then
  fail "the AI-attribution check accepted a claim of AI authorship in prose"
fi

printf 'check-hooks: commit-msg and pre-commit installed; the attribution ban rejects and accepts correctly\n'
