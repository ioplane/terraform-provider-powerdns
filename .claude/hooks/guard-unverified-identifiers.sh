#!/usr/bin/env bash
#
# guard-unverified-identifiers.sh — warn before a file:line citation without a
# revision reaches a file.
#
# docs/standards/verified-identifiers.md forbids writing an exact identifier
# from memory. It was broken once in a way that mattered: `ws-auth.cc:3361` was
# written with no revision, which is correct on master and wrong on the tag
# this project pins, where the same registration is line 3349.
#
# A line number without a revision is not wrong so much as unfalsifiable — a
# reader cannot tell which of the two it meant. This warns rather than blocks,
# because the fix is to add a revision, not to delete the citation.
set -uo pipefail

payload="$(cat)"

content="$(printf '%s' "$payload" | jq -r '
  (.tool_input.content // "") + "\n" + (.tool_input.new_string // "")
')"
[[ -z "${content//[[:space:]]/}" ]] && exit 0

# A PowerDNS source citation: a .cc or .hh file with a line number.
citations="$(printf '%s' "$content" | grep -oE '[a-zA-Z0-9_./-]+\.(cc|hh):[0-9]+' || true)"
[[ -z "$citations" ]] && exit 0

# Satisfied when a revision is named nearby: a tag, a commit, or the word.
if printf '%s' "$content" | grep -qiE 'at (tag|commit|revision)|auth-5\.|rec-5\.|dnsdist-2\.|master [0-9a-f]{7,}'; then
    exit 0
fi

cat >&2 <<MESSAGE
Warning: a source citation without a revision.

$(printf '%s' "$citations" | sort -u | sed 's/^/  /')

Line numbers move. \`ws-auth.cc:3361\` is right on master and wrong at tag
auth-5.1.3, where the same registration is line 3349 — a reader cannot tell
which was meant. Name the revision:

  ws-auth.cc:3349 at tag auth-5.1.3
  ws-auth.cc:3361 on master a74d89a8

Verify it rather than recalling it:
  git -C /opt/projects/repositories/pdns-upstream show <ref>:<path> | grep -n ...
MESSAGE
exit 0
