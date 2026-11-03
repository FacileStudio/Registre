package avatar

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/avatars"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"

	"github.com/go-chi/chi/v5"
)

// picture encodes a blank square of side pixels.
func picture(t *testing.T, side int) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := png.Encode(&out, image.NewRGBA(image.Rect(0, 0, side, side))); err != nil {
		t.Fatalf("encoding the fixture: %v", err)
	}
	return out.Bytes()
}

// get fetches path, sending etag as If-None-Match when it is set.
func get(router http.Handler, path, etag string) *http.Response {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder.Result()
}

// TestTheStableURLAlwaysServesTheCurrentPicture is the instant-update
// contract. Apps keep one URL per human; it must revalidate on every load,
// answer 304 while the picture is unchanged, and serve the new bytes the
// moment it is replaced.
func TestTheStableURLAlwaysServesTheCurrentPicture(t *testing.T) {
	url, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := testdb.Open(url)
	if err != nil {
		t.Fatalf("opening the test database: %v", err)
	}
	if err := testdb.Truncate(db); err != nil {
		t.Fatalf("truncating: %v", err)
	}
	ctx := context.Background()
	user, err := identity.New(db, credentials.New(2)).Create(ctx, identity.NewUser{
		Email: "yann@facile.studio", Password: "correct horse battery staple",
	})
	if err != nil {
		t.Fatalf("creating the user: %v", err)
	}
	service := avatars.New(db, "https://registre")
	router := chi.NewRouter()
	Mount(router, service)
	path := avatars.Path + user.ID

	if _, err := service.Put(ctx, user.ID, picture(t, 32)); err != nil {
		t.Fatalf("storing the first picture: %v", err)
	}
	first := get(router, path, "")
	etag := first.Header.Get("ETag")
	if first.StatusCode != http.StatusOK || etag == "" || first.Header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("first load = %d etag %q cache %q, want 200, an ETag and no-cache",
			first.StatusCode, etag, first.Header.Get("Cache-Control"))
	}
	if again := get(router, path, etag); again.StatusCode != http.StatusNotModified {
		t.Errorf("revalidating an unchanged picture answered %d, want 304", again.StatusCode)
	}

	if _, err := service.Put(ctx, user.ID, picture(t, 64)); err != nil {
		t.Fatalf("replacing the picture: %v", err)
	}
	replaced := get(router, path, etag)
	if replaced.StatusCode != http.StatusOK || replaced.Header.Get("ETag") == etag {
		t.Errorf("after a replacement the old ETag answered %d with ETag %q, want 200 and a new one",
			replaced.StatusCode, replaced.Header.Get("ETag"))
	}

	if err := service.Remove(ctx, user.ID); err != nil {
		t.Fatalf("removing the picture: %v", err)
	}
	if gone := get(router, path, ""); gone.StatusCode != http.StatusNotFound {
		t.Errorf("after removal the URL answered %d, want 404", gone.StatusCode)
	}
}
