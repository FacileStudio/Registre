package database

import (
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
)

func TestOpenAppliesThePoolCaps(t *testing.T) {
	url, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := Open(url, Pool{MaxOpen: 7, MaxIdle: 3, MaxLifetime: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if got := sqlDB.Stats().MaxOpenConnections; got != 7 {
		t.Fatalf("MaxOpenConnections = %d, want 7", got)
	}
}
