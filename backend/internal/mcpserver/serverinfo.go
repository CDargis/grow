package mcpserver

import (
	"context"
	"fmt"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// serverInfo answers "which code is this connector talking to?". A Claude
// client can hold an older tool list than the deployed server (the app
// caches it per chat/connection), which looks exactly like a missing
// feature. Tools is what the server offers right now, so a mismatch with
// the client's own tool list points at the client's cache, not the deploy.
type serverInfo struct {
	Commit string   `json:"commit"`
	Tools  []string `json:"tools"`
}

func (d Deps) addServerInfoTool(s *mcp.Server) {
	commit := d.BuildCommit
	if commit == "" {
		commit = "unknown"
	}
	mcp.AddTool(s, &mcp.Tool{
		Name: "server_info",
		Description: "Report the git commit this grow server was deployed from and the tools it offers right now. " +
			"If a tool or field seems missing, call this: if the server lists something your tool list lacks, " +
			"your tool list is stale -- tell Chris to start a new chat or reconnect the connector.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		names, err := listOwnTools(ctx, s)
		if err != nil {
			return nil, nil, err
		}
		return nil, serverInfo{Commit: commit, Tools: names}, nil
	})
}

// listOwnTools asks s for its tools over an in-memory connection -- the SDK
// has no direct accessor, and this reports exactly what a real client gets.
func listOwnTools(ctx context.Context, s *mcp.Server) ([]string, error) {
	st, ct := mcp.NewInMemoryTransports()
	ss, err := s.Connect(ctx, st, nil)
	if err != nil {
		return nil, fmt.Errorf("server_info: connect: %w", err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "server_info", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("server_info: client: %w", err)
	}
	defer cs.Close()

	var names []string
	for t, err := range cs.Tools(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("server_info: list tools: %w", err)
		}
		names = append(names, t.Name)
	}
	sort.Strings(names)
	return names, nil
}
