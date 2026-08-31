#!/bin/sh

set -eu

fail()
{
    printf '%s\n' "release check: $*" >&2
    exit 1
}

if [ "$#" -gt 1 ]; then
    fail "usage: $0 [vX.Y]"
fi

requested_version=${1-}
if [ -n "$requested_version" ] && \
    ! printf '%s\n' "$requested_version" | grep -Eq '^v[0-9]+\.[0-9]+$'; then
    fail "invalid version: $requested_version"
fi

script_directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repository_root=$(dirname -- "$script_directory")
temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/hard-release-check.XXXXXX")
trap 'rm -rf "$temporary_directory"' 0 1 2 3 15

development_binary=$temporary_directory/hard-development
release_binary=$temporary_directory/hard-release

(
    cd "$repository_root/hard"
    "${GO:-go}" build -trimpath -o "$development_binary" .
)

development_version=$("$development_binary" version)
if [ -z "$requested_version" ]; then
    if ! printf '%s\n' "$development_version" | \
        grep -Eq '^v[0-9]+\.[0-9]+-development$'; then
        fail "invalid development version: $development_version"
    fi
    requested_version=${development_version%-development}
fi

expected_development_version=$requested_version-development
if [ "$development_version" != "$expected_development_version" ]; then
    fail "development binary reports $development_version, expected $expected_development_version"
fi

(
    cd "$repository_root/hard"
    "${GO:-go}" build \
        -trimpath \
        -ldflags='-X main.versionPrerelease=' \
        -o "$release_binary" \
        .
)

release_version=$("$release_binary" version)
if [ "$release_version" != "$requested_version" ]; then
    fail "release binary reports $release_version, expected $requested_version"
fi

environment_report=$("$release_binary" --no-color environment)
if ! printf '%s\n' "$environment_report" | \
    grep -Fqx "  Version             $requested_version"; then
    fail "environment report does not contain version $requested_version"
fi

if find "$temporary_directory" -name VERSION -print -quit | grep -q .; then
    fail "release build created a runtime VERSION file"
fi

printf '%s\n' "Release contract verified: $requested_version"
