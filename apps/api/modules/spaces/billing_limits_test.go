package spaces

import (
	"net/http"
	"testing"
)

// TestTheCatalogIsCachedAndThePlanRoutesRateLimited keeps owners opening
// space pages from becoming one Ardoise call each, and a script looping on
// the plan routes from reaching Ardoise at all.
func TestTheCatalogIsCachedAndThePlanRoutesRateLimited(t *testing.T) {
	w := newWorld(t)
	for i := 0; i < 3; i++ {
		w.call(w.on, http.MethodGet, "/billing", w.yann, nil)
	}
	w.fake.set(func() {
		if w.fake.catalogs != 1 {
			t.Errorf("three owner views fetched the catalog %d times, want 1", w.fake.catalogs)
		}
	})
	refused := 0
	for i := 0; i < PlanBurst+5; i++ {
		if status, _ := w.call(w.on, http.MethodGet, "/billing", w.yann, nil); status == http.StatusTooManyRequests {
			refused++
		}
	}
	if refused == 0 {
		t.Errorf("%d plan reads in a burst were all admitted", PlanBurst+5)
	}
	if status, _ := w.call(w.on, http.MethodGet, "/billing", w.bob, nil); status != http.StatusOK {
		t.Errorf("another person was throttled with the first: %d", status)
	}
}
