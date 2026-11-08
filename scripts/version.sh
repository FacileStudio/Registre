#!/usr/bin/env sh
set -eu
sed -n 's/^## \[\([0-9][^]]*\)\].*/v\1/p' "${1:-CHANGELOG.md}" | head -n 1
