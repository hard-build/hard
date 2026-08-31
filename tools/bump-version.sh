#!/bin/sh

set -eu

fail() {
    printf '%s\n' "$1" >&2
    exit 1
}

valid_component() {
    case "$1" in
        '' | *[!0-9]* | 0[0-9]*) return 1 ;;
        *) return 0 ;;
    esac
}

component_greater() {
    left=$1
    right=$2

    if [ "${#left}" -gt "${#right}" ]; then
        return 0
    fi
    if [ "${#left}" -lt "${#right}" ] || [ "$left" = "$right" ]; then
        return 1
    fi

    greatest=$(printf '%s\n%s\n' "$left" "$right" | LC_ALL=C sort | tail -n 1)
    [ "$greatest" = "$left" ]
}

new_version=${1-}
case "$new_version" in
    *.*)
        new_major=${new_version%%.*}
        new_minor=${new_version#*.}
        ;;
    *)
        fail 'usage: make bump VERSION=X.Y'
        ;;
esac

valid_component "$new_major" &&
    valid_component "$new_minor" &&
    case "$new_minor" in
        *.*) false ;;
        *) true ;;
    esac || fail "invalid version: $new_version (expected canonical X.Y)"

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
root=$(CDPATH= cd -- "$script_dir/.." && pwd)
version_file=$root/hard/version.go

current_version=$(sed -n \
    's/^var versionNumber = "\([0-9][0-9]*\.[0-9][0-9]*\)"$/\1/p' \
    "$version_file")
case "$current_version" in
    '' | *"
"*) fail "cannot determine one versionNumber from $version_file" ;;
esac

current_major=${current_version%%.*}
current_minor=${current_version#*.}
valid_component "$current_major" && valid_component "$current_minor" ||
    fail "current version is not canonical: $current_version"

if [ "$new_major" = "$current_major" ]; then
    component_greater "$new_minor" "$current_minor" ||
        fail "version must increase: $current_version -> $new_version"
else
    component_greater "$new_major" "$current_major" ||
        fail "version must increase: $current_version -> $new_version"
fi

if git -C "$root" show-ref --verify --quiet "refs/tags/v$new_version"; then
    fail "release tag already exists: v$new_version"
else
    status=$?
    [ "$status" -eq 1 ] || fail "cannot inspect release tag: v$new_version"
fi

old_pattern=$(printf '%s\n' "$current_version" | sed 's/\./\\./g')
temporary=$(mktemp "${TMPDIR:-/tmp}/hard-version.XXXXXX")
trap 'rm -f "$temporary"' EXIT HUP INT TERM
sed "s/^var versionNumber = \"$old_pattern\"$/var versionNumber = \"$new_version\"/" \
    "$version_file" > "$temporary"
cat "$temporary" > "$version_file"

printf 'Bumped hard development version from %s to %s\n' \
    "$current_version" "$new_version"
