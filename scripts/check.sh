#!/usr/bin/env sh
#
# The repository quality gate. Reports, never rewrites (except --format).
#
#   sh scripts/check.sh             gofmt + vet + test, then filet
#   sh scripts/check.sh --format    rewrite Go sources in place
#
# Deliberately depends on nothing but a `go` on PATH. It is NOT invoked through
# mise: `mise run` resolves every tool in the merged config before running any
# task body, so an unrelated broken tool in the user's global config would take
# this gate down with it.

set -eu

GO_MODULES="apps/api"

mode="all"
case "${1:-}" in
--format) mode="format" ;;
"") ;;
*)
  echo "usage: $0 [--format]" >&2
  exit 2
  ;;
esac

cd "$(git rev-parse --show-toplevel)"

# Resolve the toolchain from GOROOT when it is set. mise exports GOROOT for the
# version this repo pins but leaves an unrelated `go` earlier on PATH, and a go
# binary driving a different GOROOT fails with
# `compile: version "X" does not match go tool version "Y"`.
if [ -z "${GO:-}" ]; then
  if [ -n "${GOROOT:-}" ] && [ -x "$GOROOT/bin/go" ]; then GO="$GOROOT/bin/go"; else GO=go; fi
fi
if [ -z "${GOFMT:-}" ]; then
  if [ -n "${GOROOT:-}" ] && [ -x "$GOROOT/bin/gofmt" ]; then GOFMT="$GOROOT/bin/gofmt"; else GOFMT=gofmt; fi
fi

if ! command -v "$GO" >/dev/null 2>&1 && [ ! -x "$GO" ]; then
  echo "check: no usable go ('$GO')" >&2
  exit 1
fi

if [ "$mode" = "format" ]; then
  for dir in $GO_MODULES; do
    (cd "$dir" && "$GO" fmt ./...)
  done
  exit 0
fi

# The key rotation and token issuance tests need a real Postgres and skip
# loudly without one. See docs/development.md for the one-line docker command;
# this gate does not start a database for you, because on a shared machine that
# is a surprise rather than a convenience.
if [ -z "${TEST_DATABASE_URL:-}" ]; then
  echo "check: TEST_DATABASE_URL is unset, the database-backed tests will skip" >&2
fi

status=0

for dir in $GO_MODULES; do
  echo "==> $dir"
  (
    cd "$dir" || exit 1
    s=0

    unformatted="$("$GOFMT" -l . | grep -v '^vendor/' || true)"
    if [ -n "$unformatted" ]; then
      echo "gofmt: the following files are not formatted (run 'sh scripts/check.sh --format'):"
      echo "$unformatted"
      s=1
    fi

    "$GO" vet ./... || s=1
    "$GO" test ./... || s=1

    stamped="$(sh ../../scripts/version.sh ../../CHANGELOG.md)"
    "$GO" test -count=1 -run 'TestVersion' -ldflags="-X github.com/FacileStudio/Registre/apps/api.version=$stamped" . || s=1

    # Postgres is the only database in this suite, tests included. A test on
    # SQLite builds a different schema from the GORM struct tags and then
    # passes, proving nothing about the DDL that ships — and it cannot run what
    # does ship: the partial unique index that enforces one active signing key.
    forbidden='gorm\.io/driver/(sqlite|mysql|sqlserver|clickhouse)|mattn/go-sqlite3|modernc\.org/sqlite'
    drivers="$(grep -nE "$forbidden" go.mod || true)"
    if [ -n "$drivers" ]; then
      echo "non-Postgres database driver in go.mod; use github.com/FacileStudio/tronc/testdb:"
      echo "$drivers"
      s=1
    fi

    # argon2id allocates 64 MiB per call, which makes every call site a
    # memory ceiling of its own: before internal/credentials held a single
    # process-wide limit, a hundred concurrent bad credentials against the
    # public token endpoint meant 6.4 GB resident. A second importer of
    # porte/local is a second uncapped path.
    hashers="$(grep -rln 'FacileStudio/porte/local' --include='*.go' . 2>/dev/null || true)"
    uncapped="$(echo "$hashers" | grep -v '^./internal/credentials/' | grep -v '^$' || true)"
    if [ -n "$uncapped" ]; then
      echo "porte/local imported outside internal/credentials; argon2 stays behind the concurrency ceiling:"
      echo "$uncapped"
      s=1
    fi

    # The schema is owned by migrations/, not by the models. AutoMigrate is
    # unordered, has no down path and silently no-ops on a rename. The pattern
    # is the call and not the bare word: the models' package doc explains why
    # they do not own the schema, and naming the thing it refuses to do must
    # not be what trips the guard.
    found="$(grep -rln 'AutoMigrate(' --include='*.go' . 2>/dev/null || true)"
    strays="$(echo "$found" | grep -v '_test\.go$' | grep -v '^vendor/' || true)"
    if [ -n "$strays" ]; then
      echo "AutoMigrate is back in non-test code; schema changes belong in migrations/:"
      echo "$strays"
      s=1
    fi
    exit "$s"
  ) || status=1
done

echo "==> filet"
if command -v filet >/dev/null 2>&1; then
  filet check . || status=1
else
  # Skipped rather than fatal, like the database tests above: this gate is meant
  # to run on a machine with nothing but go. CI pins the version and is the
  # authority — see .github/workflows/filet.yml.
  echo "check: no 'filet' on PATH, skipping the style gate" >&2
  echo "check: GOBIN=\$HOME/go/bin go install github.com/FacileStudio/filet@v0.19.3" >&2
fi

if [ "$status" -ne 0 ]; then
  echo "check failed"
fi
exit "$status"
