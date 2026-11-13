package accounts

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// logoutCases are the request shapes MayLogOut must sort.
var logoutCases = []struct {
	method, site, mode, csrf string
	want                     bool
}{
	{http.MethodGet, "same-origin", "navigate", "", true},
	{http.MethodGet, "same-site", "navigate", "", true},
	{http.MethodGet, "same-site", "no-cors", "", false},
	{http.MethodGet, "same-site", "", "", false},
	{http.MethodGet, "cross-site", "navigate", "", false},
	{http.MethodGet, "none", "navigate", "", false},
	{http.MethodGet, "", "", "", false},
	{http.MethodGet, "cross-site", "navigate", "1", false},
	{http.MethodPost, "", "", "1", true},
	{http.MethodPost, "same-origin", "navigate", "", false},
	{http.MethodPost, "cross-site", "navigate", "", false},
}

func TestOnlyASiblingGetOrACSRFPostMayLogOut(t *testing.T) {
	for _, c := range logoutCases {
		req := httptest.NewRequest(c.method, "/logout", nil)
		if c.site != "" {
			req.Header.Set(FetchSiteHeader, c.site)
		}
		if c.mode != "" {
			req.Header.Set(FetchModeHeader, c.mode)
		}
		if c.csrf != "" {
			req.Header.Set(CSRFHeader, c.csrf)
		}
		if got := MayLogOut(req); got != c.want {
			t.Errorf("%s with Sec-Fetch-Site %q, Sec-Fetch-Mode %q and CSRF %q: MayLogOut = %v, want %v", c.method, c.site, c.mode, c.csrf, got, c.want)
		}
	}
}

func TestARefusedLogoutNamesPost(t *testing.T) {
	recorder := httptest.NewRecorder()
	RefuseLogout(recorder)
	if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("answered %d with Allow %q, want 405 and POST", recorder.Code, recorder.Header().Get("Allow"))
	}
}
