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

// logoutModule documents the logout shortcuts: GET redirects to /, POST
// answers JSON. Both end the account session the cookie names and send
// back-channel logout to every app that session reached.
func logoutModule() Module {
	routes := []Route{}
	for _, path := range []string{"/logout", "/api/logout", "/api/account/logout"} {
		routes = append(routes,
			Route{Method: "GET", Path: path, Summary: "End the account session, sign out of its apps and go to /", Auth: "cookie", Status: 302, ResponseBody: "Found"})
		if path != "/api/account/logout" {
			routes = append(routes,
				Route{Method: "POST", Path: path, Summary: "End the account session and sign out of its apps", Auth: "cookie", RequestBody: map[string]any{}, ResponseBody: map[string]string{"ok": "true"}})
		}
	}
	return Module{Name: "logout", Description: "Logout shortcuts", Routes: routes}
}
