#!/usr/bin/env bash
#
# Enforces AGENTS.md golden rule 6: no AI attribution in commit messages.
# Takes the commit-message file as its argument.
#
# The rule bans a *claim of authorship*, not the letters. It fired on
# `.claude/hooks/guard-main-branch.sh` — a path in this repository, named by
# the tool that reads it, in a message describing what the file does. Refusing
# that would mean the repository could never describe its own contents.
#
# So the check now strips paths under a tool directory before matching. What
# remains is prose, and prose claiming a machine wrote the change is what the
# rule is about.
set -euo pipefail

readonly MSG_FILE="${1:?usage: check-no-ai-attribution.sh <commit-msg-file>}"

# An assertion of authorship, in any of the shapes it usually takes.
readonly PATTERN='(co-authored-by:[[:space:]]*(claude|chatgpt|copilot|ai\b)|generated (with|by)[[:space:]]+(claude|chatgpt|copilot|an? ai)|written by[[:space:]]+(claude|chatgpt|copilot|an? ai)|assisted by[[:space:]]+(claude|chatgpt|copilot|an? ai)|\bchatgpt\b|\bcopilot\b|\bgpt-[0-9]|\banthropic\b|\bopenai\b)'

# A bare "claude" is ambiguous: it names the tool directory this repository
# carries. Strip those paths, then look for it as a word on its own.
PROSE="$(mktemp)"
readonly PROSE
trap 'rm -f "${PROSE}"' EXIT
sed -E 's#[^[:space:]]*\.claude/[^[:space:]]*##g' "${MSG_FILE}" >"${PROSE}"

if grep -qiE "${PATTERN}" "${PROSE}" || grep -qiE '(^|[^./[:alnum:]])claude([^./[:alnum:]]|$)' "${PROSE}"; then
  echo "commit message claims AI authorship; see AGENTS.md golden rule 6" >&2
  grep -inE "${PATTERN}|(^|[^./[:alnum:]])claude([^./[:alnum:]]|$)" "${PROSE}" >&2
  exit 1
fi
