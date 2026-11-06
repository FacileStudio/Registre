package accounts

import (
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// TestASessionIsFreshOnlyRightAfterAFactor: a brand-new session that proved
// nothing (a reissued one) is not fresh.
func TestASessionIsFreshOnlyRightAfterAFactor(t *testing.T) {
	svc := New(nil)
	now := time.Now().UTC()
	old := now.Add(-time.Hour)
	recent := now.Add(-time.Minute)
	cases := map[string]struct {
		session schemas.Session
		fresh   bool
	}{
		"new, nothing proven": {schemas.Session{CreatedAt: now}, false},
		"proven just now":     {schemas.Session{CreatedAt: old, FactorProvenAt: &recent}, true},
		"proven long ago":     {schemas.Session{CreatedAt: old, FactorProvenAt: &old}, false},
	}
	for name, c := range cases {
		if got := svc.Fresh(c.session); got != c.fresh {
			t.Errorf("%s: Fresh = %v, want %v", name, got, c.fresh)
		}
	}
}
