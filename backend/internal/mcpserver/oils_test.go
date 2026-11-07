package mcpserver

import (
	"context"
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cdargis/grow/internal/model"
	"github.com/cdargis/grow/internal/oils"
)

type memLots map[string]model.Lot

func (m memLots) List(_ context.Context, u string) ([]model.Lot, error) {
	var out []model.Lot
	for _, l := range m {
		if l.UserID == u {
			out = append(out, l)
		}
	}
	return out, nil
}
func (m memLots) Get(_ context.Context, id string) (*model.Lot, error) {
	if l, ok := m[id]; ok {
		return &l, nil
	}
	return nil, nil
}
func (m memLots) Put(_ context.Context, l model.Lot) error  { m[l.LotID] = l; return nil }
func (m memLots) Delete(_ context.Context, id string) error { delete(m, id); return nil }

type memBatches map[string]model.Batch

func (m memBatches) List(_ context.Context, u string) ([]model.Batch, error) {
	var out []model.Batch
	for _, b := range m {
		if b.UserID == u {
			out = append(out, b)
		}
	}
	return out, nil
}
func (m memBatches) Get(_ context.Context, id string) (*model.Batch, error) {
	if b, ok := m[id]; ok {
		return &b, nil
	}
	return nil, nil
}
func (m memBatches) Put(_ context.Context, b model.Batch) error { m[b.BatchID] = b; return nil }
func (m memBatches) Delete(_ context.Context, id string) error  { delete(m, id); return nil }

type noPlants struct{}

func (noPlants) Get(context.Context, string) (*model.Plant, error) { return nil, nil }

type okArchiver struct{}

func (okArchiver) Archive(_ context.Context, _, prefix string) (string, error) {
	return prefix + "/x.pdf", nil
}

func (okArchiver) Remove(context.Context, string) error { return nil }

func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	d := Deps{Oils: oils.NewService(memLots{}, memBatches{}, noPlants{}, okArchiver{})}
	srv := d.newServer("u1") // panics here if any tool's schema can't be inferred

	st, ct := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

// call returns the tool's structured result decoded into out, or the error
// text when the tool reported an error.
func call(t *testing.T, cs *mcp.ClientSession, name string, args any, out any) string {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		var sb strings.Builder
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				sb.WriteString(tc.Text)
			}
		}
		return sb.String()
	}
	if out != nil {
		b, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("%s: decode: %v", name, err)
		}
	}
	return ""
}

func TestOilToolsListed(t *testing.T) {
	cs := connect(t)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, tool := range res.Tools {
		have[tool.Name] = true
	}
	for _, name := range []string{"list_lots", "create_lot", "update_lot", "list_batches",
		"get_batch", "create_batch", "update_batch", "dose", "list_plants"} {
		if !have[name] {
			t.Errorf("tool %s not listed", name)
		}
	}
}

// The kitchen flow: record a Red Mesa isolate lot, make an ISO batch from it
// in LouAna, read back the numbers, ask for a dose.
func TestKitchenFlow(t *testing.T) {
	cs := connect(t)

	var lot struct {
		Lot model.Lot `json:"lot"`
	}
	if msg := call(t, cs, "create_lot", map[string]any{
		"supplier": "Red Mesa", "product": "CBD isolate", "supplierLotNumber": "2410",
		"coaUrl":  "https://lab.example/2410.pdf",
		"profile": map[string]any{"cannabinoids": map[string]any{"cbd": 992, "d9thc": 0}},
	}, &lot); msg != "" {
		t.Fatalf("create_lot error: %s", msg)
	}
	if lot.Lot.COAS3Key == "" {
		t.Error("COA not archived")
	}

	var batch oils.BatchResult
	if msg := call(t, cs, "create_batch", map[string]any{
		"type": "ISO", "date": "2026-10-24",
		"inputs":        []any{map[string]any{"kind": "lot", "lotId": lot.Lot.LotID, "grams": 3}},
		"carrier":       map[string]any{"name": "LouAna liquid coconut oil", "grams": 27},
		"densityGPerMl": 0.94, "totalGrams": 30,
	}, &batch); msg != "" {
		t.Fatalf("create_batch error: %s", msg)
	}
	if batch.Batch.Code != "ISO-261024" || batch.Profile == nil {
		t.Fatalf("batch = %+v", batch)
	}
	if got := batch.Profile.TotalCBD.MgPerG; math.Abs(got-99.2) > 1e-9 {
		t.Errorf("total CBD = %v, want 99.2", got)
	}

	// Same code again → tool error that tells Claude how to fix it.
	msg := call(t, cs, "create_batch", map[string]any{
		"type": "ISO", "date": "2026-10-24", "totalGrams": 30,
		"inputs": []any{map[string]any{"kind": "lot", "lotId": lot.Lot.LotID, "grams": 3}},
	}, nil)
	if !strings.Contains(msg, "batchDiscriminator") {
		t.Errorf("duplicate code error = %q, want a hint about batchDiscriminator", msg)
	}

	var dose oils.DoseResult
	if msg := call(t, cs, "dose", map[string]any{"batch": "ISO-261024", "compound": "total_cbd", "targetMg": 50}, &dose); msg != "" {
		t.Fatalf("dose error: %s", msg)
	}
	if math.Abs(dose.Ml-50/99.2/0.94) > 1e-9 {
		t.Errorf("dose mL = %v", dose.Ml)
	}

	var upd oils.BatchResult
	if msg := call(t, cs, "update_batch", map[string]any{
		"batch": "ISO-261024", "changes": map[string]any{"notes": "smooth"},
	}, &upd); msg != "" {
		t.Fatalf("update_batch error: %s", msg)
	}
	if upd.Batch.Notes != "smooth" {
		t.Errorf("notes = %q", upd.Batch.Notes)
	}
}

func TestDeleteBatchTool(t *testing.T) {
	cs := connect(t)
	var lot struct {
		Lot model.Lot `json:"lot"`
	}
	call(t, cs, "create_lot", map[string]any{
		"supplier": "Red Mesa", "product": "CBD isolate", "supplierLotNumber": "2410",
		"profile": map[string]any{"cannabinoids": map[string]any{"cbd": 992}},
	}, &lot)
	call(t, cs, "create_batch", map[string]any{
		"type": "ISO", "date": "2026-10-24", "totalGrams": 30,
		"inputs": []any{map[string]any{"kind": "lot", "lotId": lot.Lot.LotID, "grams": 3}},
	}, nil)

	if msg := call(t, cs, "delete_lot", map[string]any{"lotId": lot.Lot.LotID}, nil); !strings.Contains(msg, "ISO-261024") {
		t.Errorf("delete_lot in use: %q, want refusal naming ISO-261024", msg)
	}
	if msg := call(t, cs, "delete_batch", map[string]any{"batch": "ISO-261024"}, nil); msg != "" {
		t.Fatalf("delete_batch: %s", msg)
	}
	if msg := call(t, cs, "get_batch", map[string]any{"batch": "ISO-261024"}, nil); !strings.Contains(msg, "not found") {
		t.Errorf("get_batch after delete: %q, want not found", msg)
	}
	if msg := call(t, cs, "delete_lot", map[string]any{"lotId": lot.Lot.LotID}, nil); msg != "" {
		t.Errorf("delete_lot after its batch is gone: %s", msg)
	}
}

// A flower input's profileSource and coaUrl survive create → get_batch →
// update_batch with the inputs resent unchanged (as Claude does, since
// inputs replaces the whole list), and the archived copy is kept.
func TestFlowerInputFieldsRoundTrip(t *testing.T) {
	cs := connect(t)
	const coa = "https://rogueorigin.com/cdn/shop/files/2025_Berry_Bliss_-_Sungrown_-_Post_Harvest_Lab_Analysis.jpg?v=1763066892&width=1200"

	var created oils.BatchResult
	if msg := call(t, cs, "create_batch", map[string]any{
		"type": "FS", "subType": "CBD", "discriminator": "BB", "date": "2026-07-11", "totalGrams": 120,
		"inputs": []any{map[string]any{
			"kind": "flower", "grams": 29.9, "strain": "Berry Bliss",
			"profile":    map[string]any{"cannabinoids": map[string]any{"cbda": 150, "thca": 5}},
			"efficiency": 0.7, "method": "MCT infusion", "confidence": "medium",
			"profileSource": "coa", "coaUrl": coa,
		}},
	}, &created); msg != "" {
		t.Fatalf("create_batch: %s", msg)
	}
	in := created.Batch.Inputs[0]
	if in.ProfileSource != model.ProfileFromCOA || in.COAURL != coa || in.COAS3Key == "" {
		t.Fatalf("created input = %+v", in)
	}

	var got oils.BatchView
	if msg := call(t, cs, "get_batch", map[string]any{"batch": "FS-CBD-BB-260711"}, &got); msg != "" {
		t.Fatalf("get_batch: %s", msg)
	}
	if !reflect.DeepEqual(got.Batch.Inputs[0], in) {
		t.Fatalf("get_batch input = %+v, want %+v", got.Batch.Inputs[0], in)
	}

	// Resend exactly what get_batch returned.
	var upd oils.BatchResult
	if msg := call(t, cs, "update_batch", map[string]any{
		"batch": "FS-CBD-BB-260711", "changes": map[string]any{"inputs": got.Batch.Inputs},
	}, &upd); msg != "" {
		t.Fatalf("update_batch: %s", msg)
	}
	if !reflect.DeepEqual(upd.Batch.Inputs[0], in) {
		t.Errorf("after update_batch input = %+v, want %+v", upd.Batch.Inputs[0], in)
	}
}
