#!/bin/sh

set -eu

root=$(git rev-parse --show-toplevel)
root=$(CDPATH='' cd "$root" && pwd -P)
git_dir=$(git rev-parse --absolute-git-dir)
git_dir=$(CDPATH='' cd "$git_dir" && pwd -P)
common_dir=$(git rev-parse --path-format=absolute --git-common-dir)
common_dir=$(CDPATH='' cd "$common_dir" && pwd -P)

if [ "$git_dir" = "$common_dir" ]; then
    printf '\n'
    exit 0
fi

name=$(
    basename "$root" |
        LC_ALL=C tr '[:upper:]' '[:lower:]' |
        LC_ALL=C sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//' |
        cut -c1-48 |
        LC_ALL=C sed -E 's/-+$//'
)
[ -n "$name" ] || name=worktree
hash_output=$(printf '%s' "$root" | sha256sum) || {
    echo "dev-suffix: sha256sum failed" >&2
    exit 1
}
full_digest=${hash_output%%[[:space:]]*}
case "$full_digest" in
    '' | *[!0-9a-f]*)
        echo "dev-suffix: sha256sum returned an invalid digest" >&2
        exit 1
        ;;
esac
if [ "${#full_digest}" -ne 64 ]; then
    echo "dev-suffix: sha256sum returned an invalid digest" >&2
    exit 1
fi
digest=$(printf '%.12s' "$full_digest")

printf -- '-%s-%s\n' "$name" "$digest"
