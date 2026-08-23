#!/usr/bin/env sh
#
# Builds the login client and copies it where the api binary embeds it.
# Run this whenever apps/client changes; the output is committed so a fresh
# clone compiles with nothing but a Go toolchain (scripts/check.sh depends on
# that on purpose).
#
#   sh scripts/build-client.sh

set -eu

cd "$(git rev-parse --show-toplevel)"

if ! command -v bun >/dev/null 2>&1; then
  echo "build-client: no bun on PATH" >&2
  exit 1
fi

(cd apps/client && bun install --frozen-lockfile && bun run build)
rm -rf apps/api/internal/web/dist
cp -R apps/client/build apps/api/internal/web/dist
echo "build-client: embedded build refreshed in apps/api/internal/web/dist"
