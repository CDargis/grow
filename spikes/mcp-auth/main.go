// Spike: does Claude's custom-connector OAuth flow work against an
// authorization server that has NO Dynamic Client Registration (RFC 7591)
// endpoint -- i.e. the situation Cognito would put us in?
//
// This is throwaway code. Auth here is deliberately insecure (accepts any
// client_id, no client secret check, in-memory state) -- the only thing
// being tested is whether Claude's connector setup can complete an OAuth
// Authorization Code + PKCE round-trip when oauth-authorization-server
// metadata has no registration_endpoint, and whether it exercises the one
// MCP tool afterward.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var publicBase = os.Getenv("PUBLIC_BASE_URL") // e.g. https://xyz.trycloudflare.com, no trailing slash

type pendingAuth struct {
	redirectURI   string
	codeChallenge string
	clientID      string
	state         string
}

var (
	mu     sync.Mutex
	codes  = map[string]pendingAuth{}
	tokens = map[string]bool{}
)

func randomToken(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// ── MCP server / tool ──────────────────────────────────────────────────────

type pingArgs struct {
	Message string `json:"message" jsonschema:"text to echo back"`
}

func newMCPServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "grow-mcp-spike", Version: "0.1.0"}, nil)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "ping",
		Description: "Echoes back a message. Used to confirm the authenticated MCP connection works end to end.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args pingArgs) (*mcp.CallToolResult, any, error) {
		log.Printf("tools/call ping message=%q", args.Message)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "pong: " + args.Message}},
		}, nil, nil
	})
	return s
}

// ── Auth: 401 gate + well-known metadata + minimal authorization server ────

func requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token := ""
		fmt.Sscanf(auth, "Bearer %s", &token)

		mu.Lock()
		ok := token != "" && tokens[token]
		mu.Unlock()

		if !ok {
			log.Printf("401 %s %s (Authorization=%q)", r.Method, r.URL.Path, auth)
			w.Header().Set("WWW-Authenticate",
				fmt.Sprintf(`Bearer resource_metadata="%s/.well-known/oauth-protected-resource"`, publicBase))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func serveJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func protectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	log.Printf("GET %s", r.URL.Path)
	serveJSON(w, map[string]any{
		"resource":              publicBase + "/mcp",
		"authorization_servers": []string{publicBase},
	})
}

func authorizationServerMetadata(w http.ResponseWriter, r *http.Request) {
	log.Printf("GET %s", r.URL.Path)
	serveJSON(w, map[string]any{
		"issuer":                                publicBase,
		"authorization_endpoint":                publicBase + "/authorize",
		"token_endpoint":                        publicBase + "/token",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
		// Deliberately no "registration_endpoint" -- this is the whole point
		// of the spike: simulate an authorization server with no DCR.
	})
}

func authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	log.Printf("%s /authorize client_id=%q redirect_uri=%q state=%q code_challenge=%q method=%q",
		r.Method, q.Get("client_id"), q.Get("redirect_uri"), q.Get("state"), q.Get("code_challenge"), q.Get("code_challenge_method"))

	if r.Method == http.MethodGet {
		fmt.Fprintf(w, `<!doctype html><html><body style="font-family:sans-serif;max-width:420px;margin:60px auto">
<h2>grow-mcp-spike</h2>
<p>Approve access for client <code>%s</code>?</p>
<form method="POST">
<input type="hidden" name="redirect_uri" value="%s">
<input type="hidden" name="state" value="%s">
<input type="hidden" name="client_id" value="%s">
<input type="hidden" name="code_challenge" value="%s">
<button type="submit" style="padding:10px 20px;font-size:16px">Log in as Chris &amp; Authorize</button>
</form>
</body></html>`, q.Get("client_id"), q.Get("redirect_uri"), q.Get("state"), q.Get("client_id"), q.Get("code_challenge"))
		return
	}

	r.ParseForm()
	code := randomToken(16)
	mu.Lock()
	codes[code] = pendingAuth{
		redirectURI:   r.FormValue("redirect_uri"),
		codeChallenge: r.FormValue("code_challenge"),
		clientID:      r.FormValue("client_id"),
		state:         r.FormValue("state"),
	}
	mu.Unlock()

	dest := fmt.Sprintf("%s?code=%s&state=%s", r.FormValue("redirect_uri"), code, r.FormValue("state"))
	log.Printf("redirecting to %s", dest)
	http.Redirect(w, r, dest, http.StatusFound)
}

func tokenEndpoint(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	code := r.FormValue("code")
	verifier := r.FormValue("code_verifier")
	log.Printf("POST /token grant_type=%q code=%q client_id=%q has_verifier=%v",
		r.FormValue("grant_type"), code, r.FormValue("client_id"), verifier != "")

	mu.Lock()
	pending, ok := codes[code]
	mu.Unlock()
	if !ok {
		log.Printf("token exchange failed: unknown code")
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
		return
	}

	if pending.codeChallenge != "" {
		sum := sha256.Sum256([]byte(verifier))
		computed := base64.RawURLEncoding.EncodeToString(sum[:])
		if computed != pending.codeChallenge {
			log.Printf("PKCE verification FAILED: computed=%s expected=%s", computed, pending.codeChallenge)
			http.Error(w, `{"error":"invalid_grant","error_description":"PKCE verification failed"}`, http.StatusBadRequest)
			return
		}
		log.Printf("PKCE verification OK")
	}

	access := randomToken(24)
	mu.Lock()
	tokens[access] = true
	delete(codes, code)
	mu.Unlock()

	log.Printf("issued access token for client_id=%q", pending.clientID)
	serveJSON(w, map[string]any{
		"access_token": access,
		"token_type":   "Bearer",
		"expires_in":   3600,
	})
}

func main() {
	if publicBase == "" {
		log.Fatal("set PUBLIC_BASE_URL to the public https URL this is reachable at (e.g. the cloudflared tunnel URL)")
	}

	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return newMCPServer()
	}, &mcp.StreamableHTTPOptions{JSONResponse: true, Stateless: true, DisableLocalhostProtection: true})

	mux := http.NewServeMux()
	mux.Handle("/mcp", requireAuth(mcpHandler))
	mux.HandleFunc("/.well-known/oauth-protected-resource", protectedResourceMetadata)
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", protectedResourceMetadata)
	mux.HandleFunc("/.well-known/oauth-authorization-server", authorizationServerMetadata)
	mux.HandleFunc("/authorize", authorize)
	mux.HandleFunc("/token", tokenEndpoint)

	log.Printf("public base: %s", publicBase)
	log.Printf("listening on :8090")
	log.Fatal(http.ListenAndServe(":8090", mux))
}
