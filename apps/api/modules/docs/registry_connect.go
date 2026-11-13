package docs

// connectModule documents the consent API and the connected apps list.
func connectModule() Module {
	pending := map[string]any{
		"client_id": "", "client_name": "", "registration": "metadata_document", "redirect_host": "",
		"loopback_only": false, "resource": "", "app_name": "", "scopes": []string{},
	}
	return Module{Name: "connect", Description: "Consent for third-party MCP clients and the connected apps list (session cookie, X-Facile-CSRF on writes)", Routes: []Route{
		{Method: "GET", Path: "/api/connect", Summary: "The third-party flow ?auth_request= waiting on this session's consent", Auth: "cookie", ResponseBody: pending},
		{Method: "POST", Path: "/api/connect", Summary: "Allow or refuse it; answers where the browser goes next", Auth: "cookie", RequestBody: map[string]any{"auth_request": "", "allow": true}, ResponseBody: map[string]string{"redirect": ""}},
		{Method: "GET", Path: "/api/account/apps", Summary: "The person's connected third-party apps", Auth: "cookie", ResponseBody: []map[string]any{{"id": "", "client_name": "", "resource": "", "scopes": []string{}}}},
		{Method: "DELETE", Path: "/api/account/apps/{grantID}", Summary: "Disconnect one: the grant and the client's refresh tokens end now, its access tokens within ten minutes", Auth: "cookie", Status: 204, ResponseBody: ""},
	}}
}

// csrfRefused is the answer to a logout POST without X-Facile-CSRF.
var csrfRefused = Error{Status: 403, Code: "forbidden", Description: "No X-Facile-CSRF header"}

// getRefused is the answer to an API logout GET that another site started.
var getRefused = Error{Status: 405, Code: "method_not_allowed", Description: "Not a same-origin or same-site navigation; sign out with POST and X-Facile-CSRF"}

// logoutModule documents the logout shortcuts. Each ends the account session
// the cookie names and sends back-channel logout to every app it reached, but
// a GET does so only for a same-origin or same-site navigation, so a
// link on another website cannot sign anyone out.
func logoutModule() Module {
	const sibling = "End the account session and sign out of its apps for a navigation (Sec-Fetch-Mode navigate) whose Sec-Fetch-Site is same-origin or same-site"
	return Module{Name: "logout", Description: "Logout shortcuts", Routes: []Route{
		{Method: "GET", Path: "/logout", Summary: sibling + ", then go to /", Description: "Any other GET ends nothing and answers 200 with the \"Sign out of Facile?\" page, whose button POSTs with X-Facile-CSRF.", Auth: "cookie", Status: 302, ResponseBody: "Found"},
		{Method: "POST", Path: "/logout", Summary: "End the account session and sign out of its apps (needs X-Facile-CSRF)", Auth: "cookie", RequestBody: map[string]any{}, ResponseBody: map[string]string{"ok": "true"}, Errors: []Error{csrfRefused}},
		{Method: "GET", Path: "/api/logout", Summary: sibling + ", then go to /", Auth: "cookie", Status: 302, ResponseBody: "Found", Errors: []Error{getRefused}},
		{Method: "POST", Path: "/api/logout", Summary: "End the account session and sign out of its apps (needs X-Facile-CSRF)", Auth: "cookie", RequestBody: map[string]any{}, ResponseBody: map[string]string{"ok": "true"}, Errors: []Error{csrfRefused}},
		{Method: "GET", Path: "/api/account/logout", Summary: sibling, Auth: "cookie", ResponseBody: map[string]string{"ok": "true"}, Errors: []Error{getRefused}},
	}}
}
