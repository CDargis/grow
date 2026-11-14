package mcpserver

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/cdargis/grow/internal/dose"
	"github.com/cdargis/grow/internal/model"
)

type memVersions map[string]model.DoseVersion

func (m memVersions) List(_ context.Context, u string) ([]model.DoseVersion, error) {
	var out []model.DoseVersion
	for _, v := range m {
		if v.UserID == u {
			out = append(out, v)
		}
	}
	return out, nil
}
func (m memVersions) Put(_ context.Context, v model.DoseVersion) error {
	m[v.VersionID] = v
	return nil
}
func (m memVersions) Delete(_ context.Context, id string) error { delete(m, id); return nil }

type memShelf map[string]model.ShelfItem

func (m memShelf) List(_ context.Context, u string) ([]model.ShelfItem, error) {
	var out []model.ShelfItem
	for _, it := range m {
		if it.UserID == u {
			out = append(out, it)
		}
	}
	return out, nil
}
func (m memShelf) Put(_ context.Context, it model.ShelfItem) error {
	m[it.ShelfItemID] = it
	return nil
}
func (m memShelf) Delete(_ context.Context, id string) error { delete(m, id); return nil }

// The dose flow through MCP: an isolate oil and a gummy on the shelf, a
// first plan, a bump that must say its mode, and the card read back.
func TestDoseFlow(t *testing.T) {
	cs := connect(t)

	var lot struct {
		Lot model.Lot `json:"lot"`
	}
	if msg := call(t, cs, "create_lot", map[string]any{
		"supplier": "Red Mesa", "product": "CBD isolate", "supplierLotNumber": "R260611MS08",
		"profile": map[string]any{"cannabinoids": map[string]any{"cbd": 1000}},
	}, &lot); msg != "" {
		t.Fatal(msg)
	}
	// 10 g isolate in 100 g oil at density 1 → 100 mg/mL.
	if msg := call(t, cs, "create_batch", map[string]any{
		"type": "ISO", "date": "2026-06-11", "totalGrams": 100, "densityGPerMl": 1,
		"inputs": []any{map[string]any{"kind": "lot", "lotId": lot.Lot.LotID, "grams": 10}},
	}, nil); msg != "" {
		t.Fatal(msg)
	}

	for _, item := range []map[string]any{
		{"name": "Isolate oil", "source": map[string]any{"kind": "batch", "batchId": "ISO-260611"}},
		{"name": "Night", "source": map[string]any{"kind": "product", "productName": "Sunset Lake Vibe gummy",
			"unit": "piece", "unitName": "gummy", "mgPerUnit": map[string]any{"cbd": 30}}},
	} {
		if msg := call(t, cs, "set_shelf_item", item, nil); msg != "" {
			t.Fatal(msg)
		}
	}

	times := []any{
		map[string]any{"name": "Morning", "lines": []any{map[string]any{"shelf": "Isolate oil", "mg": 100, "compound": "cbd"}}},
		map[string]any{"name": "Night", "lines": []any{map[string]any{"shelf": "Night", "mg": 15, "compound": "cbd"}}},
	}
	var v dose.VersionView
	if msg := call(t, cs, "create_dose_version", map[string]any{"times": times}, &v); msg != "" {
		t.Fatal(msg)
	}
	if v.Status != model.DoseCurrent || math.Abs(v.Times[0].Lines[0].Amount-1) > 1e-9 || v.Times[1].Lines[0].Amount != 0.5 {
		t.Fatalf("first plan = %+v", v)
	}

	// mode is required by the schema, and an unknown mode tells Claude to ask.
	if msg := call(t, cs, "update_dose_version", map[string]any{"times": times}, nil); !strings.Contains(msg, "mode") {
		t.Errorf("update without mode = %q, want a schema error about mode", msg)
	}
	if msg := call(t, cs, "update_dose_version", map[string]any{"mode": "change", "times": times}, nil); !strings.Contains(msg, "ask Chris") {
		t.Errorf("update with unknown mode = %q, want a prompt to ask Chris", msg)
	}

	var card dose.View
	if msg := call(t, cs, "get_dose_plan", map[string]any{}, &card); msg != "" {
		t.Fatal(msg)
	}
	if card.Current == nil || math.Abs(card.Current.TotalCBD-115) > 1e-9 {
		t.Errorf("card = %+v", card.Current)
	}
}
