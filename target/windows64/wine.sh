#!/bin/sh
set -eu

# Wine creates its prefix, but not missing intermediate directories.
if [ -n "${WINEPREFIX:-}" ]; then
    mkdir -p -- "$(dirname -- "$WINEPREFIX")"
fi

exec /usr/lib/wine/wine64 "$@"
