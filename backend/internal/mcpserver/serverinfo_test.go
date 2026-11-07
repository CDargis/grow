package mcpserver

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cdargis/grow/internal/oils"
)

func TestServerInfo(t *testing.T) {
	ctx := context.Background()
	d := Deps{
		Oils:        oils.NewService(memLots{}, memBatches{}, noPlants{}, okArchiver{}),
		BuildCommit: "6963320abc",
	}
	st, ct := mcp.NewInMemoryTransports()
	if _, err := d.newServer("u1").Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "server_info", Arguments: map[string]any{}})
	if err != nil || res.IsError {
		t.Fatalf("server_info: err=%v result=%+v", err, res)
	}
	var info serverInfo
	b, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(b, &info); err != nil {
		t.Fatal(err)
	}
	if info.Commit != "6963320abc" {
		t.Errorf("commit = %q", info.Commit)
	}
	for _, want := range []string{"server_info", "create_batch", "update_batch", "delete_batch", "list_plants"} {
		if !slices.Contains(info.Tools, want) {
			t.Errorf("tools %v missing %s", info.Tools, want)
		}
	}
}
