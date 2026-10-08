package httpapi

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

const claudeCallback = "https://claude.ai/api/mcp/auth_callback"

// oauthClient simulates an MCP host going through the OAuth flow.
type oauthClient struct {
	api          *testAPI
	clientID     string
	clientSecret string
	verifier     string
}

type tokenResult struct {
	Status           int
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int64  `json:"expires_in"`
	Scope            string `json:"scope"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func (a *testAPI) raw(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)
	return rec
}

func (a *testAPI) postForm(path string, form url.Values, basicUser, basicPass string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if basicUser != "" {
		req.SetBasicAuth(basicUser, basicPass)
	}
	return a.raw(req)
}

func registerClient(t *testing.T, api *testAPI, body map[string]any) (map[string]any, int) {
	t.Helper()
	var resp map[string]any
	r := api.as("").do(http.MethodPost, "/oauth/register", body)
	r.decode(&resp)
	return resp, r.Status
}

func newOAuthClient(t *testing.T, api *testAPI, authMethod string) *oauthClient {
	t.Helper()
	resp, status := registerClient(t, api, map[string]any{
		"client_name":                "Claude",
		"redirect_uris":              []string{claudeCallback},
		"token_endpoint_auth_method": authMethod,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
	})
	if status != http.StatusCreated {
		t.Fatalf("register: %d %v", status, resp)
	}
	c := &oauthClient{api: api, clientID: resp["client_id"].(string), verifier: strings.Repeat("v", 50)}
	if secret, ok := resp["client_secret"].(string); ok {
		c.clientSecret = secret
	}
	return c
}

func (c *oauthClient) challenge() string {
	sum := sha256.Sum256([]byte(c.verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (c *oauthClient) authorizeURL(overrides map[string]string) string {
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {c.clientID},
		"redirect_uri":          {claudeCallback},
		"code_challenge":        {c.challenge()},
		"code_challenge_method": {"S256"},
		"state":                 {"xyz"},
		"scope":                 {"finance"},
		"resource":              {testIssuer + "/mcp"},
	}
	for k, v := range overrides {
		if v == "" {
			q.Del(k)
		} else {
			q.Set(k, v)
		}
	}
	return "/oauth/authorize?" + q.Encode()
}

var requestIDPattern = regexp.MustCompile(`name="request_id" value="([0-9a-f-]{36})"`)

// startAuthorization opens the authorize page and returns the request id.
func (c *oauthClient) startAuthorization(t *testing.T) string {
	t.Helper()
	rec := c.api.raw(httptest.NewRequest(http.MethodGet, c.authorizeURL(nil), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("authorize page: %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("authorize page must not be frameable")
	}
	m := requestIDPattern.FindStringSubmatch(rec.Body.String())
	if m == nil {
		t.Fatalf("no request_id in page:\n%s", rec.Body)
	}
	return m[1]
}

func (c *oauthClient) submit(requestID string, fields map[string]string) *httptest.ResponseRecorder {
	form := url.Values{"request_id": {requestID}}
	for k, v := range fields {
		form.Set(k, v)
	}
	return c.api.postForm("/oauth/authorize", form, "", "")
}

// authorize runs the whole browser flow and returns the authorization code.
func (c *oauthClient) authorize(t *testing.T, email string) string {
	t.Helper()
	requestID := c.startAuthorization(t)
	if rec := c.submit(requestID, map[string]string{"action": "send_code", "email": email}); rec.Code != http.StatusOK {
		t.Fatalf("send code: %d %s", rec.Code, rec.Body)
	}
	code := c.api.mail.LastCode(t)
	rec := c.submit(requestID, map[string]string{"action": "verify", "code": code})
	if rec.Code != http.StatusFound {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(loc.String(), claudeCallback+"?") || loc.Query().Get("state") != "xyz" || loc.Query().Get("iss") != testIssuer {
		t.Fatalf("unexpected redirect: %s", loc)
	}
	return loc.Query().Get("code")
}

func (c *oauthClient) token(form url.Values) tokenResult {
	form.Set("client_id", c.clientID)
	if c.clientSecret != "" {
		form.Set("client_secret", c.clientSecret)
	}
	rec := c.api.postForm("/oauth/token", form, "", "")
	var res tokenResult
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	res.Status = rec.Code
	return res
}

func (c *oauthClient) exchange(code string) tokenResult {
	return c.token(url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {claudeCallback}, "code_verifier": {c.verifier}})
}

func (c *oauthClient) refresh(token string) tokenResult {
	return c.token(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {token}})
}

func TestOAuthMetadata(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t).as("")

	// The bare root is not advertised: a connector saved without /mcp must
	// fail at discovery.
	api.do(http.MethodGet, "/.well-known/oauth-protected-resource", nil).expect(http.StatusNotFound)

	var meta struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
		ScopesSupported      []string `json:"scopes_supported"`
	}
	api.do(http.MethodGet, "/.well-known/oauth-protected-resource/mcp", nil).expect(http.StatusOK).decode(&meta)
	if meta.Resource != testIssuer+"/mcp" || meta.AuthorizationServers[0] != testIssuer || meta.ScopesSupported[0] != "finance" {
		t.Fatalf("unexpected metadata %+v", meta)
	}

	var as map[string]any
	api.do(http.MethodGet, "/.well-known/oauth-authorization-server", nil).expect(http.StatusOK).decode(&as)
	for key, want := range map[string]string{
		"issuer":                 testIssuer,
		"authorization_endpoint": testIssuer + "/oauth/authorize",
		"token_endpoint":         testIssuer + "/oauth/token",
		"registration_endpoint":  testIssuer + "/oauth/register",
		"revocation_endpoint":    testIssuer + "/oauth/revoke",
	} {
		if as[key] != want {
			t.Errorf("%s = %v, want %s", key, as[key], want)
		}
	}
	if methods := as["code_challenge_methods_supported"].([]any); len(methods) != 1 || methods[0] != "S256" {
		t.Errorf("unexpected PKCE methods %v", methods)
	}
}

func TestOAuthClientRegistration(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)

	public, status := registerClient(t, api, map[string]any{"client_name": "Claude", "redirect_uris": []string{claudeCallback}, "token_endpoint_auth_method": "none"})
	if status != http.StatusCreated || !strings.HasPrefix(public["client_id"].(string), "fw_") || public["client_secret"] != nil {
		t.Fatalf("public client: %d %v", status, public)
	}

	confidential, status := registerClient(t, api, map[string]any{"redirect_uris": []string{"https://chatgpt.com/connector_platform_oauth_redirect"}})
	if status != http.StatusCreated || confidential["token_endpoint_auth_method"] != "client_secret_basic" || confidential["client_secret"] == "" {
		t.Fatalf("default auth method should be client_secret_basic with a secret: %v", confidential)
	}

	invalid := []map[string]any{
		{},
		{"redirect_uris": []string{"http://evil.example/cb"}},
		{"redirect_uris": []string{claudeCallback}, "token_endpoint_auth_method": "private_key_jwt"},
		{"redirect_uris": []string{claudeCallback}, "grant_types": []string{"password"}},
		{"redirect_uris": []string{claudeCallback}, "response_types": []string{"token"}},
	}
	for _, body := range invalid {
		if resp, status := registerClient(t, api, body); status != http.StatusBadRequest || resp["error"] == nil {
			t.Errorf("register %v: expected 400 with error, got %d %v", body, status, resp)
		}
	}
	rec := api.raw(httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader("not json")))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed body: expected 400, got %d", rec.Code)
	}
}

func TestOAuthAuthorizationCodeFlow(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")

	code := client.authorize(t, "owner@example.com")

	msg := api.mail.Messages()[0]
	if !strings.Contains(msg.Subject, "Claude") || strings.Contains(msg.Text, "http") {
		t.Fatalf("expected a code-only email mentioning the client, got %q:\n%s", msg.Subject, msg.Text)
	}

	tokens := client.exchange(code)
	if tokens.Status != http.StatusOK || tokens.AccessToken == "" || tokens.RefreshToken == "" || tokens.TokenType != "Bearer" || tokens.ExpiresIn != 3600 || tokens.Scope != "finance" {
		t.Fatalf("unexpected token response: %+v", tokens)
	}

	// The access token works against the API as the authorizing user.
	var me struct{ Email string }
	api.as(tokens.AccessToken).do(http.MethodGet, "/api/v1/auth/me", nil).expect(http.StatusOK).decode(&me)
	if me.Email != "owner@example.com" {
		t.Fatalf("token belongs to %q", me.Email)
	}

	// Authorization codes are single use.
	if again := client.exchange(code); again.Status != http.StatusBadRequest || again.Error != "invalid_grant" {
		t.Fatalf("code reuse: %+v", again)
	}
}

func TestOAuthRefreshTokenRotation(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")
	first := client.exchange(client.authorize(t, "owner@example.com"))

	second := client.refresh(first.RefreshToken)
	if second.Status != http.StatusOK || second.RefreshToken == first.RefreshToken || second.AccessToken == first.AccessToken {
		t.Fatalf("refresh should rotate tokens: %+v", second)
	}
	api.as(second.AccessToken).do(http.MethodGet, "/api/v1/auth/me", nil).expect(http.StatusOK)

	// Reusing a rotated refresh token revokes the whole family.
	if reuse := client.refresh(first.RefreshToken); reuse.Status != http.StatusBadRequest || reuse.Error != "invalid_grant" {
		t.Fatalf("reuse: %+v", reuse)
	}
	api.as(second.AccessToken).do(http.MethodGet, "/api/v1/auth/me", nil).expectError(http.StatusUnauthorized)
	if after := client.refresh(second.RefreshToken); after.Status != http.StatusBadRequest {
		t.Fatalf("family should be revoked: %+v", after)
	}
}

func TestOAuthTokenErrors(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")
	other := newOAuthClient(t, api, "none")

	code := client.authorize(t, "owner@example.com")
	wrongVerifier := *client
	wrongVerifier.verifier = strings.Repeat("w", 50)
	if res := wrongVerifier.exchange(code); res.Error != "invalid_grant" {
		t.Fatalf("wrong verifier: %+v", res)
	}

	code = client.authorize(t, "owner@example.com")
	if res := other.exchange(code); res.Error != "invalid_grant" {
		t.Fatalf("code for another client: %+v", res)
	}

	code = client.authorize(t, "owner@example.com")
	if res := client.token(url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {"https://other.example/cb"}, "code_verifier": {client.verifier}}); res.Error != "invalid_grant" {
		t.Fatalf("redirect mismatch: %+v", res)
	}

	code = client.authorize(t, "owner@example.com")
	api.oauth.SetClock(func() time.Time { return time.Now().Add(10 * time.Minute) })
	if res := client.exchange(code); res.Error != "invalid_grant" {
		t.Fatalf("expired code: %+v", res)
	}
	api.oauth.SetClock(time.Now)

	for name, form := range map[string]url.Values{
		"missing verifier":  {"grant_type": {"authorization_code"}, "code": {"x"}},
		"unsupported grant": {"grant_type": {"password"}},
		"missing refresh":   {"grant_type": {"refresh_token"}},
		"unknown refresh":   {"grant_type": {"refresh_token"}, "refresh_token": {"nope"}},
		"unknown code":      {"grant_type": {"authorization_code"}, "code": {"nope"}, "code_verifier": {client.verifier}},
	} {
		if res := client.token(form); res.Status != http.StatusBadRequest || res.Error == "" {
			t.Errorf("%s: expected 400 with error, got %+v", name, res)
		}
	}

	rec := api.postForm("/oauth/token", url.Values{"grant_type": {"refresh_token"}, "client_id": {"fw_unknown"}}, "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unknown client: expected 401, got %d", rec.Code)
	}
}

func TestOAuthConfidentialClient(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "client_secret_basic")
	if client.clientSecret == "" {
		t.Fatal("expected a client secret")
	}
	code := client.authorize(t, "owner@example.com")

	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {claudeCallback}, "code_verifier": {client.verifier}}
	if rec := api.postForm("/oauth/token", form, client.clientID, "wrong-secret"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong secret: expected 401, got %d", rec.Code)
	}
	rec := api.postForm("/oauth/token", form, client.clientID, client.clientSecret)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("basic auth exchange: %d %s", rec.Code, rec.Body)
	}
}

func TestOAuthAuthorizeValidation(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")

	// Problems with the client or redirect URI are shown on the page.
	for name, overrides := range map[string]map[string]string{
		"unknown client":      {"client_id": "fw_unknown"},
		"unregistered target": {"redirect_uri": "https://evil.example/cb"},
	} {
		rec := api.raw(httptest.NewRequest(http.MethodGet, client.authorizeURL(overrides), nil))
		if rec.Code != http.StatusBadRequest || rec.Header().Get("Location") != "" {
			t.Errorf("%s: expected an error page, got %d (Location %q)", name, rec.Code, rec.Header().Get("Location"))
		}
	}

	// Other problems are reported back to the client.
	for name, tc := range map[string]struct {
		overrides map[string]string
		error     string
	}{
		"token response type": {map[string]string{"response_type": "token"}, "unsupported_response_type"},
		"missing PKCE":        {map[string]string{"code_challenge": ""}, "invalid_request"},
		"plain PKCE":          {map[string]string{"code_challenge_method": "plain"}, "invalid_request"},
		"foreign resource":    {map[string]string{"resource": "https://other.example/mcp"}, "invalid_target"},
	} {
		rec := api.raw(httptest.NewRequest(http.MethodGet, client.authorizeURL(tc.overrides), nil))
		loc, _ := url.Parse(rec.Header().Get("Location"))
		if rec.Code != http.StatusFound || loc.Query().Get("error") != tc.error || loc.Query().Get("state") != "xyz" {
			t.Errorf("%s: expected redirect with error %s, got %d %s", name, tc.error, rec.Code, loc)
		}
	}

	// redirect_uri may be omitted when the client registered exactly one.
	rec := api.raw(httptest.NewRequest(http.MethodGet, client.authorizeURL(map[string]string{"redirect_uri": ""}), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("omitted redirect_uri: %d", rec.Code)
	}
}

func TestOAuthAuthorizePageFlowErrors(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")
	requestID := client.startAuthorization(t)

	if rec := client.submit(requestID, map[string]string{"action": "send_code", "email": "nope"}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid email: %d", rec.Code)
	}
	if rec := client.submit(requestID, map[string]string{"action": "verify", "code": "123456"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("verify before sending code: %d", rec.Code)
	}

	// Unknown addresses look the same as known ones, but no email is sent.
	if rec := client.submit(requestID, map[string]string{"action": "send_code", "email": "stranger@example.com"}); rec.Code != http.StatusOK {
		t.Fatalf("unknown email: %d", rec.Code)
	}
	if len(api.mail.Messages()) != 0 {
		t.Fatal("no email should be sent to unknown addresses")
	}
	if rec := client.submit(requestID, map[string]string{"action": "verify", "code": "123456"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong code: %d", rec.Code)
	}

	if rec := client.submit(requestID, map[string]string{"action": "change_email"}); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `name="email"`) {
		t.Fatalf("change email: %d", rec.Code)
	}

	rec := client.submit(requestID, map[string]string{"action": "deny"})
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if rec.Code != http.StatusFound || loc.Query().Get("error") != "access_denied" {
		t.Fatalf("deny: %d %s", rec.Code, loc)
	}
	// The request is gone after denial.
	if rec := client.submit(requestID, map[string]string{"action": "change_email"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("request should be deleted: %d", rec.Code)
	}
	if rec := client.submit("not-a-uuid", map[string]string{"action": "deny"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid request id: %d", rec.Code)
	}
}

func TestOAuthAuthorizePageIsLocalized(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")

	req := httptest.NewRequest(http.MethodGet, client.authorizeURL(nil), nil)
	req.Header.Set("Accept-Language", "es-MX,es;q=0.9")
	rec := api.raw(req)
	if !strings.Contains(rec.Body.String(), "Conectar Claude") || !strings.Contains(rec.Body.String(), `lang="es"`) {
		t.Fatalf("expected Spanish page:\n%s", rec.Body)
	}
}

func TestOAuthRevocation(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")
	tokens := client.exchange(client.authorize(t, "owner@example.com"))

	rec := api.postForm("/oauth/revoke", url.Values{"token": {tokens.RefreshToken}, "client_id": {client.clientID}}, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke: %d", rec.Code)
	}
	api.as(tokens.AccessToken).do(http.MethodGet, "/api/v1/auth/me", nil).expectError(http.StatusUnauthorized)
	if res := client.refresh(tokens.RefreshToken); res.Error != "invalid_grant" {
		t.Fatalf("refresh after revoke: %+v", res)
	}

	// Revoking by access token works too, and unknown tokens still return 200.
	tokens = client.exchange(client.authorize(t, "owner@example.com"))
	api.postForm("/oauth/revoke", url.Values{"token": {tokens.AccessToken}, "client_id": {client.clientID}}, "", "")
	api.as(tokens.AccessToken).do(http.MethodGet, "/api/v1/auth/me", nil).expectError(http.StatusUnauthorized)
	if rec := api.postForm("/oauth/revoke", url.Values{"token": {"unknown"}, "client_id": {client.clientID}}, "", ""); rec.Code != http.StatusOK {
		t.Fatalf("unknown token: %d", rec.Code)
	}
}

func TestOAuthPurgeExpired(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")
	client.startAuthorization(t)
	client.exchange(client.authorize(t, "owner@example.com"))
	api.oauth.SetClock(func() time.Time { return time.Now().Add(100 * 24 * time.Hour) })
	if err := api.oauth.PurgeExpired(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"oauth_authorization_requests", "oauth_refresh_tokens", "oauth_grants"} {
		var remaining int
		if err := api.pool.QueryRow(t.Context(), "SELECT count(*) FROM "+table).Scan(&remaining); err != nil {
			t.Fatal(err)
		}
		if remaining != 0 {
			t.Fatalf("expected expired %s to be purged, %d left", table, remaining)
		}
	}
}

type connectionBody struct {
	ID          string    `json:"id"`
	ClientName  string    `json:"client_name"`
	ConnectedAt time.Time `json:"connected_at"`
	LastUsedAt  time.Time `json:"last_used_at"`
	Workspace   *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"workspace"`
}

func (a *testAPI) listConnections() []connectionBody {
	a.t.Helper()
	var list struct{ Items []connectionBody }
	a.do(http.MethodGet, "/api/v1/auth/connections", nil).expect(http.StatusOK).decode(&list)
	return list.Items
}

// mcpStatus returns the status of an MCP request made with the token.
func (a *testAPI) mcpStatus(token string) int {
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+token)
	return a.raw(req).Code
}

func TestOAuthListConnections(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	if got := api.listConnections(); len(got) != 0 {
		t.Fatalf("expected no connections, got %+v", got)
	}

	client := newOAuthClient(t, api, "none")
	first := client.exchange(client.authorize(t, "owner@example.com"))
	// Refreshing keeps a single connection.
	second := client.refresh(first.RefreshToken)
	if second.Status != http.StatusOK {
		t.Fatalf("refresh: %+v", second)
	}
	connections := api.listConnections()
	if len(connections) != 1 {
		t.Fatalf("expected 1 connection, got %+v", connections)
	}
	c := connections[0]
	if c.ClientName != "Claude" || c.Workspace == nil || c.Workspace.Name != "Home" || c.ConnectedAt.IsZero() || c.LastUsedAt.Before(c.ConnectedAt) {
		t.Fatalf("unexpected connection %+v", c)
	}

	// Another user sees only their own connections.
	bob := api.newUser("bob@example.com", "Bob's")
	if got := bob.listConnections(); len(got) != 0 {
		t.Fatalf("bob should not see the owner's connections: %+v", got)
	}

	// A connection the app revoked itself is no longer listed.
	api.postForm("/oauth/revoke", url.Values{"token": {second.AccessToken}, "client_id": {client.clientID}}, "", "")
	if got := api.listConnections(); len(got) != 0 {
		t.Fatalf("revoked connection should not be listed: %+v", got)
	}
}

func TestOAuthDisconnect(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")
	revoked := client.exchange(client.authorize(t, "owner@example.com"))
	if status := api.mcpStatus(revoked.AccessToken); status != http.StatusOK {
		t.Fatalf("access token should reach /mcp before disconnecting, got %d", status)
	}
	connections := api.listConnections()
	if len(connections) != 1 {
		t.Fatalf("expected 1 connection, got %+v", connections)
	}
	id := connections[0].ID
	// A second authorization is a separate connection.
	kept := client.exchange(client.authorize(t, "owner@example.com"))

	// Other users can't disconnect it.
	bob := api.newUser("bob@example.com", "Bob's")
	bob.do(http.MethodDelete, "/api/v1/auth/connections/"+id, nil).expectError(http.StatusNotFound)
	bob.do(http.MethodDelete, "/api/v1/auth/connections/not-a-uuid", nil).expectError(http.StatusUnprocessableEntity)
	api.as("").do(http.MethodDelete, "/api/v1/auth/connections/"+id, nil).expectError(http.StatusUnauthorized)

	api.do(http.MethodDelete, "/api/v1/auth/connections/"+id, nil).expect(http.StatusNoContent)
	api.do(http.MethodDelete, "/api/v1/auth/connections/"+id, nil).expectError(http.StatusNotFound)
	api.do(http.MethodDelete, "/api/v1/auth/connections/"+missingID, nil).expectError(http.StatusNotFound)
	if got := api.listConnections(); len(got) != 1 || got[0].ID == id {
		t.Fatalf("expected only the other connection, got %+v", got)
	}

	// The disconnected app can neither refresh nor call the API or /mcp.
	if res := client.refresh(revoked.RefreshToken); res.Status != http.StatusBadRequest || res.Error != "invalid_grant" {
		t.Fatalf("refresh after disconnect: %+v", res)
	}
	if status := api.mcpStatus(revoked.AccessToken); status != http.StatusUnauthorized {
		t.Fatalf("/mcp after disconnect: expected 401, got %d", status)
	}
	api.as(revoked.AccessToken).do(http.MethodGet, "/api/v1/auth/me", nil).expectError(http.StatusUnauthorized)

	// The other connection and the user's browser session keep working.
	if status := api.mcpStatus(kept.AccessToken); status != http.StatusOK {
		t.Fatalf("the other connection should reach /mcp, got %d", status)
	}
	if res := client.refresh(kept.RefreshToken); res.Status != http.StatusOK {
		t.Fatalf("the other connection should keep refreshing: %+v", res)
	}
	api.do(http.MethodGet, "/api/v1/auth/me", nil).expect(http.StatusOK)
}
