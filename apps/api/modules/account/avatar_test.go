package account

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
)

// upload sends raw image bytes the way the account page does.
func (f fixture) upload(t *testing.T, raw []byte, csrf bool) (*http.Response, map[string]any) {
	t.Helper()
	req := f.cookie(httptest.NewRequest(http.MethodPut, "/api/account/avatar", bytes.NewReader(raw)))
	req.Header.Set("Content-Type", "image/png")
	if csrf {
		req.Header.Set(accounts.CSRFHeader, "1")
	}
	return f.do(t, req)
}

// TestAPictureIsUploadedShownAndRemoved runs the page's whole round trip: a
// cross-site upload is refused, a real one lands under the issuer and shows on
// the account view, a non-image is refused, and removing it clears the view.
func TestAPictureIsUploadedShownAndRemoved(t *testing.T) {
	f := newFixture(t)
	var picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 64, 48))); err != nil {
		t.Fatalf("encoding the fixture: %v", err)
	}

	if response, _ := f.upload(t, picture.Bytes(), false); response.StatusCode != http.StatusForbidden {
		t.Fatalf("an upload without the CSRF header answered %d, want 403", response.StatusCode)
	}
	response, body := f.upload(t, picture.Bytes(), true)
	url, _ := body["picture"].(string)
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(url, "https://registre/avatars/") {
		t.Fatalf("upload answered %d %v, want 200 and a URL under the issuer", response.StatusCode, body)
	}
	if _, me := f.send(t, http.MethodGet, "/api/account/", nil); me["picture"] != url {
		t.Errorf("the account view shows picture %v, want %q", me["picture"], url)
	}
	if response, _ := f.upload(t, []byte("not a picture"), true); response.StatusCode != http.StatusBadRequest {
		t.Errorf("a non-image upload answered %d, want 400", response.StatusCode)
	}
	if response, _ := f.send(t, http.MethodDelete, "/api/account/avatar", nil); response.StatusCode != http.StatusOK {
		t.Fatalf("removing the picture answered %d", response.StatusCode)
	}
	if _, me := f.send(t, http.MethodGet, "/api/account/", nil); me["picture"] != "" {
		t.Errorf("after removal the account view still shows %v", me["picture"])
	}
}
