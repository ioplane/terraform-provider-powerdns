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

readonly REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

fail() { printf 'check-hooks: %s\n' "$*" >&2; exit 1; }

for hook in commit-msg pre-commit; do
    [[ -f ".git/hooks/${hook}" ]] || fail "${hook} hook is not installed — run: task hooks"
done

readonly TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Must be rejected.
printf 'feat(x): a change\n\nCo-Authored-By: Claude <noreply@anthropic.com>\n' > "$TMP/bad"
if scripts/check-no-ai-attribution.sh "$TMP/bad" >/dev/null 2>&1; then
    fail "the AI-attribution check accepted a message carrying an AI trailer"
fi

# Must be accepted. A checker that rejects everything is equally broken, and
# only testing the rejection would not notice.
printf 'feat(x): a change\n\nAn ordinary body naming the reason.\n' > "$TMP/good"
if ! scripts/check-no-ai-attribution.sh "$TMP/good" >/dev/null 2>&1; then
    fail "the AI-attribution check rejected an ordinary message"
fi

printf 'check-hooks: commit-msg and pre-commit installed; the attribution ban rejects and accepts correctly\n'
