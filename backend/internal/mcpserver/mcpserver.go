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
	Region           string
	UserPoolID       string
	UserPoolClientID string
}

func (d Deps) issuer() string {
	return fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", d.Region, d.UserPoolID)
}

// ── Well-known metadata (unauthenticated) ───────────────────────────────────

// ProtectedResourceMetadata points MCP clients at Cognito as the
// authorization server. Cognito is the real OAuth provider -- this server
// never implements /authorize or /token itself, it only tells clients where
// to find them.
func (d Deps) ProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"resource":              d.PublicBaseURL + "/api/mcp",
		"authorization_servers": []string{d.issuer()},
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

	type recentActivityArgs struct {
		Days int `json:"days,omitempty" jsonschema:"how many days back to look, default 7"`
	}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_recent_activity",
		Description: "Get recent log entries across all of the user's plants, most recent first.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args recentActivityArgs) (*mcp.CallToolResult, any, error) {
		days := args.Days
		if days <= 0 {
			days = 7
		}
		var all []model.Log
		now := time.Now().UTC()
		for i := 0; i < days; i++ {
			date := now.AddDate(0, 0, -i).Format("2006-01-02")
			logs, err := d.Logs.ListForDate(ctx, userID, date)
			if err != nil {
				return nil, nil, err
			}
			all = append(all, logs...)
		}
		return nil, struct {
			Logs []model.Log `json:"logs"`
		}{Logs: all}, nil
	})

	return s
}
