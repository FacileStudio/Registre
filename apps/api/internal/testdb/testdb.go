// Package testdb is registre's configuration of the suite's test-database
// helper.
//
// The logic — one Postgres schema per test binary, the search_path carried in
// the connection string rather than a SET, truncate-and-reseed — lives in
// tronc/testdb, which seven suite apps already share. tronc exists because
// twelve services each carried their own copy of code like this under
// apps/api/internal/ and the copies drifted; a thirteenth copy here would be
// that story starting again. What is registre's, and all this package
// supplies, is the schema prefix, how to migrate, and the command that turns
// the tests on.
package testdb

import (
	"context"
	"hash/fnv"

	"github.com/FacileStudio/Registre/apps/api/migrations"

	"github.com/FacileStudio/tronc/migrate"
	shared "github.com/FacileStudio/tronc/testdb"
	"gorm.io/gorm"
)

// Prefix names the per-binary schemas.
const Prefix = "registre_test"

// Bootstrap is the command that gives a developer a database to point at. It
// is spelled out in full because a gate that rots does so silently: whoever
// sees the skip banner has to be able to turn the tests on without reading
// another file.
const Bootstrap = `docker run -d --rm --name registre-db \
  -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=registre_test \
  -p 127.0.0.1:5433:5432 postgres:16-alpine
export TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5433/registre_test?sslmode=disable'`

// URL returns the configured connection string, or false when the tests should
// skip.
func URL() (string, bool) { return shared.URL() }

// Announce writes the skip banner to stderr. t.Skip only shows under -v, and a
// gate nobody notices is off is worse than no gate at all.
func Announce() { shared.Announce(Bootstrap) }

// SkipReason is the message a skipped test prints.
func SkipReason() string { return shared.SkipReason(Bootstrap) }

// Open returns a migrated connection scoped to this test binary's own schema.
func Open(url string) (*gorm.DB, error) { return shared.Open(url, config()) }

// Truncate empties every table in the current schema, sparing the migration
// ledger, so a re-seeded fixture always lands on the same rows.
func Truncate(db *gorm.DB) error { return shared.Truncate(db, config()) }

func config() shared.Config {
	return shared.Config{Prefix: Prefix, Migrate: migrateSchema}
}

// migrateSchema brings a freshly created schema up to date with the same
// migrations the binary runs.
//
// The advisory lock id is derived from the schema name because goose's own
// lock is database-scoped, not schema-scoped: without it every test package
// sharing one test database queues on the same default id.
func migrateSchema(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return migrate.Run(context.Background(), migrate.Config{
		DB: sqlDB, FS: migrations.FS, LockID: lockID(shared.SchemaName(Prefix)),
	})
}

func lockID(schema string) int64 {
	digest := fnv.New64a()
	_, _ = digest.Write([]byte(schema))
	return int64(digest.Sum64())
}
