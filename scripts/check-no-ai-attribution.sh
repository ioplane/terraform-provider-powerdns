#!/usr/bin/env bash
# Enforces AGENTS.md golden rule 6: no AI attribution in commit messages.
# Takes the commit-message file as its argument.
set -euo pipefail

readonly MSG_FILE="${1:?usage: check-no-ai-attribution.sh <commit-msg-file>}"
readonly PATTERN='(claude|chatgpt|copilot|gpt-[0-9]|anthropic|openai|generated with|co-authored-by:[[:space:]]*(claude|ai\b))'

if grep -qiE "${PATTERN}" "${MSG_FILE}"; then
  echo "commit message mentions AI attribution; see AGENTS.md golden rule 6" >&2
  grep -inE "${PATTERN}" "${MSG_FILE}" >&2
  exit 1
fi
