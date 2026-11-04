package accounts

import (
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// TestASessionIsFreshOnlyRightAfterAFactor.
func TestASessionIsFreshOnlyRightAfterAFactor(t *testing.T) {
	svc := New(nil)
	now := time.Now().UTC()
	old := now.Add(-time.Hour)
	recent := now.Add(-time.Minute)
	cases := map[string]struct {
		session schemas.Session
		fresh   bool
	}{
		"just signed in":         {schemas.Session{CreatedAt: recent}, true},
		"old, never reproven":    {schemas.Session{CreatedAt: old}, false},
		"old, reproven":          {schemas.Session{CreatedAt: old, ReauthenticatedAt: &recent}, true},
		"old, reproven long ago": {schemas.Session{CreatedAt: old, ReauthenticatedAt: &old}, false},
	}
	for name, c := range cases {
		if got := svc.Fresh(c.session); got != c.fresh {
			t.Errorf("%s: Fresh = %v, want %v", name, got, c.fresh)
		}
	}
}
