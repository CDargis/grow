package dose

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/cdargis/grow/internal/model"
	"github.com/cdargis/grow/internal/oils"
)

// ── Fakes ────────────────────────────────────────────────────────────────────

type memVersions map[string]model.DoseVersion

func (m memVersions) List(_ context.Context, userID string) ([]model.DoseVersion, error) {
	out := []model.DoseVersion{}
	for _, v := range m {
		if v.UserID == userID {
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

func (m memShelf) List(_ context.Context, userID string) ([]model.ShelfItem, error) {
	out := []model.ShelfItem{}
	for _, it := range m {
		if it.UserID == userID {
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

type memLots map[string]model.Lot

func (m memLots) List(_ context.Context, userID string) ([]model.Lot, error) {
	out := []model.Lot{}
	for _, l := range m {
		out = append(out, l)
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

func (m memBatches) List(_ context.Context, userID string) ([]model.Batch, error) {
	out := []model.Batch{}
	for _, b := range m {
		out = append(out, b)
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

type okArchiver struct{}

func (okArchiver) Archive(_ context.Context, _, prefix string) (string, error) {
	return prefix + "/archived.pdf", nil
}
func (okArchiver) Remove(context.Context, string) error { return nil }

// ── Setup ────────────────────────────────────────────────────────────────────

const user = "u1"

func approx(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

// fixture: distillate oil at exactly 100 mg/mL CBD (density 1 keeps the
// arithmetic readable), a 10 g/100 g isolate oil, and a THC oil.
type fixture struct {
	svc      *Service
	versions memVersions
	shelf    memShelf
	clock    *time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	lots := memLots{
		"lot-dis": {LotID: "lot-dis", UserID: user, Supplier: "Red Mesa", Product: "distillate", SupplierLotNumber: "R260310MS52",
			Profile: model.Profile{Cannabinoids: map[string]float64{"cbd": 800, "cbg": 20, "d9thc": 0}}},
	}
	batches := memBatches{
		// 12.5 g distillate in 100 g oil → 100 mg/g CBD, 2.5 mg/g CBG.
		"b-dis": {BatchID: "b-dis", UserID: user, Code: "DIS-260712", Type: model.BatchDIS,
			Inputs:        []model.BatchInput{{Kind: model.InputLot, LotID: "lot-dis", Grams: 12.5}},
			DensityGPerMl: 1, TotalGrams: 100},
		"b-dis2": {BatchID: "b-dis2", UserID: user, Code: "DIS-261024", Type: model.BatchDIS,
			Inputs:        []model.BatchInput{{Kind: model.InputLot, LotID: "lot-dis", Grams: 25}},
			DensityGPerMl: 1, TotalGrams: 100}, // 200 mg/mL
	}
	clock := time.Date(2026, 11, 14, 18, 0, 0, 0, time.UTC) // noon in Chicago
	f := &fixture{versions: memVersions{}, shelf: memShelf{}, clock: &clock}
	f.svc = NewService(f.versions, f.shelf, lots, batches, okArchiver{})
	f.svc.now = func() time.Time { return *f.clock }
	return f
}

func (f *fixture) setDay(t *testing.T, date string) {
	t.Helper()
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		t.Fatal(err)
	}
	*f.clock = d.Add(18 * time.Hour)
}

func (f *fixture) shelfUp(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	if _, err := f.svc.SetShelfItem(ctx, user, SetShelfItemRequest{Name: "Distillate oil",
		Source: model.ShelfSource{Kind: model.ShelfFromBatch, BatchID: "DIS-260712"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.SetShelfItem(ctx, user, SetShelfItemRequest{Name: "Night",
		Source: model.ShelfSource{Kind: model.ShelfFromProduct, ProductName: "Sunset Lake Vibe gummy",
			Unit: model.ProductPerPiece, UnitName: "gummy", MgPerUnit: map[string]float64{"cbd": 30}}}); err != nil {
		t.Fatal(err)
	}
}

func plan(morningMg float64) []TimeRequest {
	return []TimeRequest{
		{Name: "Morning", Lines: []LineRequest{{Shelf: "distillate oil", Mg: morningMg, Compound: "cbd"}}},
		{Name: "Night", Lines: []LineRequest{{Shelf: "Night", Mg: 15, Compound: "cbd"}}},
	}
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestDerivedCard(t *testing.T) {
	f := newFixture(t)
	f.shelfUp(t)
	v, err := f.svc.CreateVersion(context.Background(), user, CreateVersionRequest{Times: plan(260)})
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != model.DoseCurrent || v.Version.StartDate != "2026-11-14" {
		t.Fatalf("first plan: status %s start %s, want current from today", v.Status, v.Version.StartDate)
	}
	morning := v.Times[0].Lines[0]
	approx(t, "distillate mL", morning.Amount, 2.6)
	if morning.Unit != model.ProductPerMl || morning.Source != "DIS-260712" {
		t.Errorf("morning line unit/source = %s/%s", morning.Unit, morning.Source)
	}
	if morning.Shares["lab"] != 1 {
		t.Errorf("distillate shares = %v, want all lab", morning.Shares)
	}
	night := v.Times[1].Lines[0]
	approx(t, "gummies", night.Amount, 0.5)
	if night.UnitName != "gummy" {
		t.Errorf("night unit name = %q", night.UnitName)
	}

	approx(t, "day CBD", v.TotalCBD, 275)
	approx(t, "minors (CBG from 2.6 mL)", v.Minors, 2.6*2.5)
	approx(t, "distillate mL/30d", v.Shelf[0].Per30Days, 78)
	approx(t, "gummies/30d", v.Shelf[1].Per30Days, 15)
	if len(v.RawLots) != 1 {
		t.Fatalf("raw lots = %+v", v.RawLots)
	}
	// 2.6 mL/day × 0.125 g distillate per mL × 30.
	approx(t, "raw distillate g/30d", v.RawLots[0].Per30Days, 2.6*0.125*30)
	if v.RawLots[0].Label != "Red Mesa · distillate · R260310MS52" {
		t.Errorf("lot label = %q", v.RawLots[0].Label)
	}
}

// Opening a new bottle repoints the shelf: same mg, new mL, old source kept.
func TestRepointKeepsDoseAndHistory(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.shelfUp(t)
	if _, err := f.svc.CreateVersion(ctx, user, CreateVersionRequest{Times: plan(260)}); err != nil {
		t.Fatal(err)
	}
	f.setDay(t, "2026-11-20")
	res, err := f.svc.SetShelfItem(ctx, user, SetShelfItemRequest{Name: "Distillate oil",
		Source: model.ShelfSource{Kind: model.ShelfFromBatch, BatchID: "b-dis2"}})
	if err != nil {
		t.Fatal(err)
	}
	h := res.Item.History
	if len(h) != 1 || h[0].Source.BatchID != "b-dis" || h[0].From != "2026-11-14" || h[0].To != "2026-11-20" {
		t.Errorf("history = %+v", h)
	}
	view, err := f.svc.Get(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	line := view.Current.Times[0].Lines[0]
	approx(t, "mg unchanged", line.Mg, 260)
	approx(t, "mL at 200 mg/mL", line.Amount, 1.3)
	if len(view.History) != 0 {
		t.Errorf("repointing made a plan version: %+v", view.History)
	}
}

func TestDraftStartAndHistory(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.shelfUp(t)
	first, err := f.svc.CreateVersion(ctx, user, CreateVersionRequest{Times: plan(260), StartDate: "2026-10-01"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != model.DoseCurrent {
		t.Fatalf("backfilled first plan status = %s", first.Status)
	}

	draft, err := f.svc.CreateVersion(ctx, user, CreateVersionRequest{Name: "Distillate-led", Times: plan(300)})
	if err != nil {
		t.Fatal(err)
	}
	if draft.Status != model.DoseDraft {
		t.Fatalf("second create status = %s, want draft", draft.Status)
	}
	if _, err := f.svc.CreateVersion(ctx, user, CreateVersionRequest{Times: plan(1)}); !errors.Is(err, oils.ErrConflict) {
		t.Errorf("second draft err = %v, want conflict", err)
	}

	// Look at it side by side, then start it tomorrow.
	view, _ := f.svc.Get(ctx, user)
	if view.Current == nil || view.Draft == nil {
		t.Fatalf("want current and draft, got %+v / %+v", view.Current, view.Draft)
	}
	if _, err := f.svc.StartVersion(ctx, user, StartVersionRequest{Date: "2026-11-15"}); err != nil {
		t.Fatal(err)
	}
	view, _ = f.svc.Get(ctx, user)
	if view.Current.Version.VersionID != first.Version.VersionID {
		t.Errorf("today the old plan should still be current")
	}
	if view.Draft == nil || view.Draft.Version.StartDate != "2026-11-15" {
		t.Errorf("scheduled draft = %+v", view.Draft)
	}

	f.setDay(t, "2026-11-15")
	view, _ = f.svc.Get(ctx, user)
	if view.Current.Version.VersionID != draft.Version.VersionID || view.Draft != nil {
		t.Fatalf("on the 15th the draft should be current")
	}
	if len(view.History) != 1 || view.History[0].Version.EndDate != "2026-11-15" {
		t.Errorf("history = %+v", view.History)
	}
	if view.History[0].ShelfNames[first.Times[0].Lines[0].ShelfItemID] != "Distillate oil" {
		t.Errorf("history shelf names = %v", view.History[0].ShelfNames)
	}
}

func TestBumpAndAmend(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.shelfUp(t)
	if _, err := f.svc.CreateVersion(ctx, user, CreateVersionRequest{Times: plan(260), StartDate: "2026-10-01"}); err != nil {
		t.Fatal(err)
	}

	if _, err := f.svc.UpdateVersion(ctx, user, UpdateVersionRequest{Name: ptr("x")}); !errors.Is(err, oils.ErrInvalid) {
		t.Errorf("missing mode err = %v, want invalid (ask Chris)", err)
	}

	name := "typo fixed"
	if _, err := f.svc.UpdateVersion(ctx, user, UpdateVersionRequest{Mode: "amend", Name: &name}); err != nil {
		t.Fatal(err)
	}
	if len(f.versions) != 1 {
		t.Fatalf("amend made a new version")
	}

	times := plan(310)
	bumped, err := f.svc.UpdateVersion(ctx, user, UpdateVersionRequest{Mode: "bump", Times: &times})
	if err != nil {
		t.Fatal(err)
	}
	if bumped.Version.StartDate != "2026-11-14" || bumped.Version.Name != "typo fixed" {
		t.Errorf("bumped = start %s name %q", bumped.Version.StartDate, bumped.Version.Name)
	}
	view, _ := f.svc.Get(ctx, user)
	approx(t, "current morning mg", view.Current.Times[0].Lines[0].Mg, 310)
	if len(view.History) != 1 || view.History[0].Version.Times[0].Lines[0].Mg != 260 {
		t.Errorf("history should keep the 260 version: %+v", view.History)
	}

	// A second bump the same day would make a zero-length version.
	if _, err := f.svc.UpdateVersion(ctx, user, UpdateVersionRequest{Mode: "bump", Times: &times}); !errors.Is(err, oils.ErrInvalid) {
		t.Errorf("same-day bump err = %v, want invalid (amend instead)", err)
	}
}

func TestDeleteScheduledDraftRestoresCurrent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.shelfUp(t)
	cur, err := f.svc.CreateVersion(ctx, user, CreateVersionRequest{Times: plan(260), StartDate: "2026-10-01"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CreateVersion(ctx, user, CreateVersionRequest{Times: plan(300), StartDate: "2026-11-20"}); err != nil {
		t.Fatal(err)
	}
	if f.versions[cur.Version.VersionID].EndDate != "2026-11-20" {
		t.Fatalf("scheduling should end the current version on the 20th")
	}
	if err := f.svc.DeleteVersion(ctx, user, ""); err != nil {
		t.Fatal(err)
	}
	if f.versions[cur.Version.VersionID].EndDate != "" {
		t.Errorf("deleting the scheduled draft should reopen the current version")
	}
	if err := f.svc.DeleteVersion(ctx, user, cur.Version.VersionID); !errors.Is(err, oils.ErrInvalid) {
		t.Errorf("deleting a started version err = %v, want invalid", err)
	}
}

func TestShelfRules(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.shelfUp(t)
	if _, err := f.svc.CreateVersion(ctx, user, CreateVersionRequest{Times: plan(260)}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.DeleteShelfItem(ctx, user, "Night"); !errors.Is(err, oils.ErrConflict) {
		t.Errorf("deleting a used item err = %v, want conflict", err)
	}
	if _, err := f.svc.CreateVersion(ctx, user, CreateVersionRequest{Times: []TimeRequest{
		{Name: "Morning", Lines: []LineRequest{{Shelf: "nope", Mg: 1, Compound: "cbd"}}}}}); !errors.Is(err, oils.ErrInvalid) {
		t.Errorf("unknown shelf item err = %v, want invalid", err)
	}
	if _, err := f.svc.SetShelfItem(ctx, user, SetShelfItemRequest{Name: "Night", NewName: "distillate OIL",
		Source: model.ShelfSource{Kind: model.ShelfFromBatch, BatchID: "b-dis"}}); !errors.Is(err, oils.ErrConflict) {
		t.Errorf("rename onto another name err = %v, want conflict", err)
	}
	if _, err := f.svc.SetShelfItem(ctx, user, SetShelfItemRequest{Name: "X",
		Source: model.ShelfSource{Kind: model.ShelfFromBatch, BatchID: "NOPE-1"}}); !errors.Is(err, oils.ErrNotFound) {
		t.Errorf("unknown batch err = %v, want not found", err)
	}
	res, err := f.svc.SetShelfItem(ctx, user, SetShelfItemRequest{Name: "Peach",
		Source: model.ShelfSource{Kind: model.ShelfFromProduct, ProductName: "Peach Power", Unit: model.ProductPerPiece,
			UnitName: "gummy", MgPerUnit: map[string]float64{"cbd": 40}, COAURL: "https://example.com/coa.pdf"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Item.Source.COAS3Key == "" {
		t.Errorf("product COA not archived")
	}
}

// A line on a source with none of the compound carries an error instead of
// failing the plan.
func TestLineErrorDoesNotFailPlan(t *testing.T) {
	f := newFixture(t)
	f.shelfUp(t)
	v, err := f.svc.CreateVersion(context.Background(), user, CreateVersionRequest{Times: []TimeRequest{
		{Name: "Morning", Lines: []LineRequest{
			{Shelf: "Distillate oil", Mg: 2, Compound: "thc"}, // THC not detected
			{Shelf: "Distillate oil", Mg: 100, Compound: "cbd"},
		}}}})
	if err != nil {
		t.Fatal(err)
	}
	if v.Times[0].Lines[0].Error == "" {
		t.Errorf("THC line should carry an error")
	}
	approx(t, "CBD still counted", v.TotalCBD, 100)
}

func ptr[T any](v T) *T { return &v }
