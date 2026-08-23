// Package migrations embeds the ordered SQL that owns registre's schema.
//
// It is a package rather than a bare directory so the tests can reach the same
// migrations the binary runs, instead of each test package building its own
// schema from the GORM struct tags.
package migrations

import "embed"

// FS holds the migration files at its root, which is the layout
// tronc/migrate expects — no fs.Sub needed.
//
//go:embed *.sql
var FS embed.FS
