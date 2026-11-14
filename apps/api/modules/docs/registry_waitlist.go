package docs

import "net/http"

// WaitlistJoin is what a landing page posts to join a product's waitlist.
type WaitlistJoin struct {
	Email   string `json:"email" validate:"required"`
	Product string `json:"product" validate:"required"`
	Locale  string `json:"locale" validate:"required"`
	Consent string `json:"consent" validate:"required"`
	Altcha  string `json:"altcha" validate:"required"`
}

func waitlistModule() Module {
	return Module{Name: "waitlist", Description: "Product waitlists for the Scribe and Pastille landings; CORS limited to those origins without credentials, every route 404 without ALTCHA_HMAC_KEY", Routes: []Route{
		{Method: "GET", Path: "/api/waitlist/challenge", Summary: "A fresh ALTCHA proof-of-work challenge", ResponseBody: map[string]any{"algorithm": "SHA-256", "challenge": "", "maxNumber": 100000, "salt": "", "signature": ""}},
		{Method: "POST", Path: "/api/waitlist", Summary: "Join a waitlist (product scribe or pastille, locale fr, en, es or de); 202 for a new and a known address alike, 400 invalid_email, invalid_product, invalid_locale, consent_required or invalid_proof, 403 from another origin, 429 with retry_after", RequestBody: WaitlistJoin{}, ResponseBody: map[string]bool{"ok": true}, Status: http.StatusAccepted},
		{Method: "POST", Path: "/api/waitlist/unsubscribe", Summary: "Delete the entry a token names, from ?token= (List-Unsubscribe one-click) or the body; 200 again for an entry already gone, 400 invalid_token", RequestBody: tokenBody{}, ResponseBody: map[string]bool{"ok": true}},
	}}
}
