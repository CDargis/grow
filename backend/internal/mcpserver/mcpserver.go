// Package mcpserver exposes a read-only Model Context Protocol server over
// the existing grow-api data (plants, logs), for use as a Claude custom
// connector. Registered on its own route, excluded from the API Gateway JWT
// authorizer that guards the rest of the API -- see the package comment on
// the auth gate below for why.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cdargis/grow/internal/model"
	"github.com/cdargis/grow/internal/store"
)

type Deps struct {
	Plants           *store.PlantStore
	Logs             *store.LogStore
	PublicBaseURL    string // e.g. https://grow.chrisdargis.com, no trailing slash
	HostedUIBase     string // e.g. https://grow-chrisdargis.auth.us-east-1.amazoncognito.com, no trailing slash
	Region           string
	UserPoolID       string
	UserPoolClientID string
}

func (d Deps) issuer() string {
	return fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", d.Region, d.UserPoolID)
}

// ── Well-known metadata (unauthenticated) ───────────────────────────────────

// ProtectedResourceMetadata points MCP clients at ourselves as the
// authorization server (see AuthorizationServerMetadata below for why it's
// us and not Cognito directly).
func (d Deps) ProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"resource":                 d.PublicBaseURL + "/api/mcp",
		"authorization_servers":    []string{d.PublicBaseURL},
		"scopes_supported":         []string{"openid", "email", "profile"},
		"bearer_methods_supported": []string{"header"},
	})
}

// AuthorizationServerMetadata serves RFC 8414 authorization-server metadata
// at OUR domain root, with the endpoints pointing at Cognito's real hosted-UI
// authorize/token endpoints. We do this instead of listing Cognito's issuer
// in authorization_servers because Cognito's issuer URL has a path component
// (/{poolId}) and Cognito only serves its discovery document at the OIDC
// suffix form ({issuer}/.well-known/openid-configuration) -- the RFC 8414
// path-insertion forms and the oauth-authorization-server forms all return
// 400 (verified empirically). Claude's connector discovery never finds it and
// fails with "Authorization with the MCP server failed" before any request
// reaches the login page, Cognito, or us. Hosting the metadata ourselves at
// the domain root reproduces the exact topology validated end-to-end in
// spikes/mcp-auth (metadata at {origin}/.well-known/oauth-authorization-server,
// no path in the issuer), which a real Claude connector completed successfully.
// Cognito remains the only real OAuth implementation -- this is a static
// pointer document, not a proxy: /authorize and /token still go directly to
// Cognito, and token validation still checks Cognito's issuer.
func (d Deps) AuthorizationServerMetadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"issuer":                                d.PublicBaseURL,
		"authorization_endpoint":                d.HostedUIBase + "/oauth2/authorize",
		"token_endpoint":                        d.HostedUIBase + "/oauth2/token",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post", "client_secret_basic", "none"},
		"scopes_supported":                      []string{"openid", "email", "profile"},
	})
}

// ── Auth gate ────────────────────────────────────────────────────────────────
//
// This route is deliberately excluded from API Gateway's JWT authorizer.
// That authorizer returns a generic 401 with no WWW-Authenticate header, but
// MCP clients discover where to authenticate via a 401 carrying
// `WWW-Authenticate: Bearer resource_metadata="..."`. Handling auth here
// gives full control over that response shape -- confirmed necessary and
// sufficient against a real Claude custom connector in spikes/mcp-auth.

type ctxKey int

const userIDKey ctxKey = 0

type jwksCache struct {
	mu      sync.Mutex
	set     jwk.Set
	fetched time.Time
}

func (c *jwksCache) get(ctx context.Context, jwksURL string) (jwk.Set, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.set != nil && time.Since(c.fetched) < time.Hour {
		return c.set, nil
	}
	set, err := jwk.Fetch(ctx, jwksURL)
	if err != nil {
		return nil, err
	}
	c.set = set
	c.fetched = time.Now()
	return set, nil
}

func bearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return ""
	}
	return strings.TrimPrefix(auth, prefix)
}

func (d Deps) validateAccessToken(ctx context.Context, cache *jwksCache, tokenStr string) (string, error) {
	set, err := cache.get(ctx, d.issuer()+"/.well-known/jwks.json")
	if err != nil {
		return "", fmt.Errorf("fetch jwks: %w", err)
	}
	tok, err := jwt.Parse([]byte(tokenStr), jwt.WithKeySet(set))
	if err != nil {
		return "", fmt.Errorf("parse/verify: %w", err)
	}
	if iss, _ := tok.Issuer(); iss != d.issuer() {
		return "", fmt.Errorf("unexpected issuer %q", iss)
	}
	var tokenUse string
	if err := tok.Get("token_use", &tokenUse); err != nil || tokenUse != "access" {
		return "", fmt.Errorf("not an access token")
	}
	var clientID string
	if err := tok.Get("client_id", &clientID); err != nil || clientID != d.UserPoolClientID {
		return "", fmt.Errorf("unexpected client_id")
	}
	sub, ok := tok.Subject()
	if !ok || sub == "" {
		return "", fmt.Errorf("missing sub claim")
	}
	return sub, nil
}

func (d Deps) unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate",
		fmt.Sprintf(`Bearer resource_metadata="%s/.well-known/oauth-protected-resource"`, d.PublicBaseURL))
	w.WriteHeader(http.StatusUnauthorized)
}

// Handler returns the /api/mcp HTTP handler: validates the bearer token,
// then serves the MCP Streamable HTTP transport with tools scoped to the
// authenticated user.
func (d Deps) Handler() http.Handler {
	cache := &jwksCache{}

	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		userID, _ := r.Context().Value(userIDKey).(string)
		return d.newServer(userID)
	}, &mcp.StreamableHTTPOptions{JSONResponse: true, Stateless: true, DisableLocalhostProtection: true})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			d.unauthorized(w)
			return
		}
		userID, err := d.validateAccessToken(r.Context(), cache, token)
		if err != nil {
			log.Printf("mcp: token rejected: %v", err)
			d.unauthorized(w)
			return
		}
		mcpHandler.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, userID)))
	})
}

// ── Tools ────────────────────────────────────────────────────────────────────

func (d Deps) newServer(userID string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "grow", Version: "1.0.0"}, nil)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_plants",
		Description: "List all of the user's plants (active and past grows), with id, name, strain, and phase.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		plants, err := d.Plants.List(ctx, userID)
		if err != nil {
			return nil, nil, err
		}
		return nil, struct {
			Plants []model.Plant `json:"plants"`
		}{Plants: plants}, nil
	})

	type getPlantArgs struct {
		PlantID string `json:"plantId" jsonschema:"the plant's id, from list_plants"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_plant",
		Description: "Get full details for one plant by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getPlantArgs) (*mcp.CallToolResult, any, error) {
		plant, err := d.Plants.Get(ctx, args.PlantID)
		if err != nil {
			return nil, nil, err
		}
		if plant.UserID != userID {
			return nil, nil, fmt.Errorf("plant not found")
		}
		return nil, plant, nil
	})

	type listLogsArgs struct {
		PlantID string `json:"plantId" jsonschema:"the plant's id, from list_plants"`
		Limit   int    `json:"limit,omitempty" jsonschema:"max number of most-recent logs to return, default 20"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_logs_for_plant",
		Description: "List a plant's log entries (watering/feeding, training, trimming, height, notes, photos, phase changes, etc.), newest first.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listLogsArgs) (*mcp.CallToolResult, any, error) {
		plant, err := d.Plants.Get(ctx, args.PlantID)
		if err != nil {
			return nil, nil, err
		}
		if plant.UserID != userID {
			return nil, nil, fmt.Errorf("plant not found")
		}
		logs, err := d.Logs.ListForPlant(ctx, args.PlantID)
		if err != nil {
			return nil, nil, err
		}
		limit := args.Limit
		if limit <= 0 {
			limit = 20
		}
		if len(logs) > limit {
			logs = logs[:limit]
		}
		return nil, struct {
			Logs []model.Log `json:"logs"`
		}{Logs: logs}, nil
	})

	const maxDateRangeDays = 31

	type logsByDateRangeArgs struct {
		StartDate string `json:"startDate" jsonschema:"first date to include, inclusive, YYYY-MM-DD"`
		EndDate   string `json:"endDate" jsonschema:"last date to include, inclusive, YYYY-MM-DD"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_logs_by_date_range",
		Description: fmt.Sprintf("Get log entries across all of the user's plants within an inclusive date range (max %d days), most recent first. Use a narrow range to limit how much comes back.", maxDateRangeDays),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args logsByDateRangeArgs) (*mcp.CallToolResult, any, error) {
		start, err := time.Parse("2006-01-02", args.StartDate)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid startDate, expected YYYY-MM-DD: %w", err)
		}
		end, err := time.Parse("2006-01-02", args.EndDate)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid endDate, expected YYYY-MM-DD: %w", err)
		}
		if end.Before(start) {
			return nil, nil, fmt.Errorf("endDate must not be before startDate")
		}
		numDays := int(end.Sub(start).Hours()/24) + 1
		if numDays > maxDateRangeDays {
			return nil, nil, fmt.Errorf("range is %d days, max is %d -- ask for a narrower range", numDays, maxDateRangeDays)
		}

		var all []model.Log
		for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
			logs, err := d.Logs.ListForDate(ctx, userID, day.Format("2006-01-02"))
			if err != nil {
				return nil, nil, err
			}
			all = append(all, logs...)
		}
		sort.Slice(all, func(i, j int) bool {
			if all[i].Date != all[j].Date {
				return all[i].Date > all[j].Date
			}
			return all[i].LoggedAt > all[j].LoggedAt
		})
		return nil, struct {
			Logs []model.Log `json:"logs"`
		}{Logs: all}, nil
	})

	return s
}
