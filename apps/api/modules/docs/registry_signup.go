package docs

import "net/http"

// Ceremony is a WebAuthn answer: the bearer from the options and the
// browser's PublicKeyCredential as JSON.
type Ceremony struct {
	Ceremony   string         `json:"ceremony" validate:"required"`
	Credential map[string]any `json:"credential" validate:"required"`
	Name       string         `json:"name,omitempty"`
}

// PasskeyOptions is what navigator.credentials needs, plus the bearer.
type PasskeyOptions struct {
	Ceremony  string         `json:"ceremony"`
	PublicKey map[string]any `json:"publicKey"`
}

// SignupStart is the first sign-up step.
type SignupStart struct {
	Email       string `json:"email" validate:"required"`
	Altcha      string `json:"altcha" validate:"required"`
	Locale      string `json:"locale,omitempty"`
	AuthRequest string `json:"auth_request,omitempty"`
}

// SignupFinish creates the account behind a live link.
type SignupFinish struct {
	Token      string         `json:"token" validate:"required"`
	Name       string         `json:"name" validate:"required"`
	Locale     string         `json:"locale" validate:"required"`
	Accept     bool           `json:"accept_terms" validate:"required"`
	Password   string         `json:"password,omitempty"`
	Ceremony   string         `json:"ceremony,omitempty"`
	Credential map[string]any `json:"credential,omitempty"`
}

// LegalDocument is one legal text.
type LegalDocument struct {
	Document string `json:"document"`
	Version  string `json:"version"`
	Locale   string `json:"locale"`
	Title    string `json:"title"`
	HTML     string `json:"html"`
	Draft    bool   `json:"draft"`
	Required bool   `json:"required"`
}

// tokenBody is a body carrying one link token.
type tokenBody struct {
	Token string `json:"token" validate:"required"`
}

// okBody is the common success body.
func okBody() map[string]string { return map[string]string{"ok": "true"} }

// signupModules documents the routes public sign-up added.
func signupModules() []Module {
	return []Module{signupModule(), passkeyLoginModule(), accountSelfModule(), legalModule(), invitationModule()}
}

func signupModule() Module {
	return Module{Name: "signup", Description: "Public self-service sign-up; every route answers 404 while SIGNUP_ENABLED is off", Routes: []Route{
		{Method: "GET", Path: "/api/signup/challenge", Summary: "A fresh ALTCHA proof-of-work challenge", ResponseBody: map[string]any{"algorithm": "SHA-256", "challenge": "", "maxNumber": 100000, "salt": "", "signature": ""}},
		{Method: "POST", Path: "/api/signup/start", Summary: "Mail a sign-up link; 202 sent whether or not the address has an account", RequestBody: SignupStart{}, ResponseBody: map[string]string{"status": "sent"}, Status: http.StatusAccepted},
		{Method: "POST", Path: "/api/signup/verify", Summary: "What a live link is for, without spending it; 410 when gone", RequestBody: tokenBody{}, ResponseBody: map[string]string{"email": "", "locale": "en"}},
		{Method: "POST", Path: "/api/signup/passkey", Summary: "Begin registering the new account's passkey", RequestBody: tokenBody{}, ResponseBody: PasskeyOptions{}},
		{Method: "POST", Path: "/api/signup/finish", Summary: "Create the account (passkey or NIST password), record consent, sign in", RequestBody: SignupFinish{}, ResponseBody: map[string]string{"redirect": "/"}, Status: http.StatusCreated},
	}}
}

func passkeyLoginModule() Module {
	return Module{Name: "passkey login", Description: "Discoverable passkey sign-in", Routes: []Route{
		{Method: "POST", Path: "/api/login/passkey/begin", Summary: "Options for a discoverable assertion", RequestBody: map[string]any{}, ResponseBody: PasskeyOptions{}},
		{Method: "POST", Path: "/api/login/passkey/finish", Summary: "Verify the assertion and finish the login", RequestBody: Ceremony{}, ResponseBody: LoginResponse{}},
	}}
}

func accountSelfModule() Module {
	return Module{Name: "account self-service", Description: "Profile, consent, passkeys, reauthentication, export and deletion", Routes: []Route{
		{Method: "POST", Path: "/api/account/profile", Summary: "Change name and locale (fr, en, es, de)", Auth: "cookie", RequestBody: map[string]string{"name": "", "locale": "fr"}, ResponseBody: map[string]string{"name": "", "locale": "fr"}},
		{Method: "POST", Path: "/api/account/consent", Summary: "Accept the current CGU and privacy policy", Auth: "cookie", RequestBody: map[string]bool{"accept_terms": true}, ResponseBody: okBody()},
		{Method: "POST", Path: "/api/account/reauth", Summary: "Prove the password (and TOTP code if enrolled) again", Auth: "cookie", RequestBody: map[string]string{"password": "", "code": ""}, ResponseBody: okBody()},
		{Method: "POST", Path: "/api/account/reauth/passkey", Summary: "Begin a passkey reauthentication", Auth: "cookie", RequestBody: map[string]any{}, ResponseBody: PasskeyOptions{}},
		{Method: "POST", Path: "/api/account/reauth/passkey/finish", Summary: "Finish a passkey reauthentication", Auth: "cookie", RequestBody: Ceremony{}, ResponseBody: okBody()},
		{Method: "GET", Path: "/api/account/passkeys", Summary: "My passkeys", Auth: "cookie", ResponseBody: map[string]any{"passkeys": []any{}}},
		{Method: "POST", Path: "/api/account/passkeys/begin", Summary: "Begin adding a passkey (fresh factor required)", Auth: "cookie", RequestBody: map[string]any{}, ResponseBody: PasskeyOptions{}},
		{Method: "POST", Path: "/api/account/passkeys/finish", Summary: "Store the new passkey", Auth: "cookie", RequestBody: Ceremony{}, ResponseBody: map[string]string{"id": "", "name": ""}, Status: http.StatusCreated},
		{Method: "PATCH", Path: "/api/account/passkeys/{id}", Summary: "Rename a passkey", Auth: "cookie", RequestBody: map[string]string{"name": ""}, ResponseBody: okBody()},
		{Method: "DELETE", Path: "/api/account/passkeys/{id}", Summary: "Remove a passkey (fresh factor; 409 for the last factor)", Auth: "cookie", ResponseBody: okBody()},
		{Method: "GET", Path: "/api/account/export", Summary: "My Registre data as a JSON download", Auth: "cookie", ResponseBody: map[string]any{"account": map[string]any{}}},
		{Method: "POST", Path: "/api/account/deletion", Summary: "Schedule my account's deletion in 30 days (fresh factor, typed email)", Auth: "cookie", RequestBody: map[string]string{"confirm_email": ""}, ResponseBody: map[string]string{"deletion_scheduled_for": ""}, Status: http.StatusAccepted},
		{Method: "DELETE", Path: "/api/account/deletion", Summary: "Cancel a pending deletion", Auth: "cookie", ResponseBody: okBody()},
	}}
}

func legalModule() Module {
	return Module{Name: "legal", Description: "Versioned legal documents in fr, en, es, de", Routes: []Route{
		{Method: "GET", Path: "/api/legal", Summary: "Every document with its current version", ResponseBody: map[string]any{"documents": []LegalDocument{}}},
		{Method: "GET", Path: "/api/legal/{document}", Summary: "One document (cgu, cgv, privacy, legal-notice) in ?lang=", ResponseBody: LegalDocument{}},
	}}
}

func invitationModule() Module {
	return Module{Name: "space invitations", Description: "Pending invitations into spaces; membership exists only after acceptance", Routes: []Route{
		{Method: "GET", Path: "/api/spaces/invitations", Summary: "Invitations addressed to me", Auth: "cookie", ResponseBody: map[string]any{"invitations": []any{}}},
		{Method: "POST", Path: "/api/spaces/invitations/{invitationID}/accept", Summary: "Join the space at the offered role", Auth: "cookie", RequestBody: map[string]any{}, ResponseBody: SpaceSummary{}},
		{Method: "POST", Path: "/api/spaces/invitations/{invitationID}/decline", Summary: "Decline an invitation", Auth: "cookie", RequestBody: map[string]any{}, ResponseBody: map[string]bool{"ok": true}},
		{Method: "DELETE", Path: "/api/spaces/{spaceID}/invitations/{invitationID}", Summary: "Revoke a pending invitation (admin or owner)", Auth: "cookie", ResponseBody: map[string]bool{"ok": true}},
	}}
}
