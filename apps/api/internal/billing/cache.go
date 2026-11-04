package billing

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// CatalogTTL is how long registre reuses Ardoise's catalog, the same 60 s
// Ardoise caches it for. Every owner opening a space page would otherwise be
// one more upstream call.
const CatalogTTL = 60 * time.Second

// catalogCache holds the last catalog Ardoise answered successfully.
type catalogCache struct {
	mu      sync.Mutex
	value   Catalog
	fetched time.Time
}

// fresh answers the cached catalog when it is younger than CatalogTTL.
func (c *catalogCache) fresh(now time.Time) (Catalog, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value, !c.fetched.IsZero() && now.Sub(c.fetched) < CatalogTTL
}

// store keeps a catalog fetched at now.
func (c *catalogCache) store(value Catalog, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value, c.fetched = value, now
}

// Catalog answers the published plans, the only ones that can be bought,
// from cache when Ardoise answered within CatalogTTL. A failure is never
// cached, so the next call tries again.
func (c *Client) Catalog(ctx context.Context) (Catalog, error) {
	if cached, ok := c.catalog.fresh(time.Now()); ok {
		return cached, nil
	}
	var out Catalog
	if err := c.call(ctx, http.MethodGet, "/api/catalog", nil, &out); err != nil {
		return Catalog{}, err
	}
	c.catalog.store(out, time.Now())
	return out, nil
}
