package accounts

import "net/http"

// FetchSiteHeader is the Fetch Metadata header a browser sets on every
// request it sends, naming how the page that started it relates to this
// origin.
const FetchSiteHeader = "Sec-Fetch-Site"

// FetchModeHeader is the Fetch Metadata header naming what kind of request
// the browser is making: "navigate" for a top-level page load or a link.
const FetchModeHeader = "Sec-Fetch-Mode"

// MayLogOut reports whether r is a sign-out the person asked for, rather than
// one another site started on their behalf.
//
// A POST must carry CSRFHeader, which no cross-origin page can set without a
// preflight this API never answers. A GET counts only when the browser says
// it came from this origin or a sibling subdomain (same-origin or same-site):
// every suite app is a facile.studio subdomain and signs out by sending the
// browser to <issuer>/logout, so that stays one hop. It must also be a
// navigation: an image or a script on a sibling host that serves someone
// else's content is same-site too, and is not the person asking. Everything
// else ends
// nothing: a cross-site link or image, a typed address ("none"), and a request
// without the header at all, which is an old browser or a script. Logging out
// fans out to every app through back-channel logout, so a link on any other
// website would otherwise sign the person out of the whole suite.
func MayLogOut(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return r.Header.Get(CSRFHeader) != ""
	}
	if r.Header.Get(FetchModeHeader) != "navigate" {
		return false
	}
	switch r.Header.Get(FetchSiteHeader) {
	case "same-origin", "same-site":
		return true
	}
	return false
}

// RefuseLogout answers an API GET logout that MayLogOut refused: 405, naming
// the request that works.
func RefuseLogout(w http.ResponseWriter) {
	w.Header().Set("Allow", http.MethodPost)
	write(w, map[string]string{"error": "sign out with POST and the " + CSRFHeader + " header"}, http.StatusMethodNotAllowed)
}
