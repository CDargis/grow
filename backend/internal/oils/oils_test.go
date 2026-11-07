package oils

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/cdargis/grow/internal/model"
)

// ── Fakes ────────────────────────────────────────────────────────────────────

type fakeLots struct{ m map[string]model.Lot }

func (f *fakeLots) List(_ context.Context, userID string) ([]model.Lot, error) {
	var out []model.Lot
	for _, l := range f.m {
		if l.UserID == userID {
			out = append(out, l)
		}
	}
	return out, nil
}
func (f *fakeLots) Get(_ context.Context, id string) (*model.Lot, error) {
	if l, ok := f.m[id]; ok {
		return &l, nil
	}
	return nil, nil
}
func (f *fakeLots) Put(_ context.Context, l model.Lot) error  { f.m[l.LotID] = l; return nil }
func (f *fakeLots) Delete(_ context.Context, id string) error { delete(f.m, id); return nil }

type fakeBatches struct{ m map[string]model.Batch }

func (f *fakeBatches) List(_ context.Context, userID string) ([]model.Batch, error) {
	var out []model.Batch
	for _, b := range f.m {
		if b.UserID == userID {
			out = append(out, b)
		}
	}
	return out, nil
}
func (f *fakeBatches) Get(_ context.Context, id string) (*model.Batch, error) {
	if b, ok := f.m[id]; ok {
		return &b, nil
	}
	return nil, nil
}
func (f *fakeBatches) Put(_ context.Context, b model.Batch) error { f.m[b.BatchID] = b; return nil }
func (f *fakeBatches) Delete(_ context.Context, id string) error  { delete(f.m, id); return nil }

type fakePlants struct{ m map[string]model.Plant }

func (f *fakePlants) Get(_ context.Context, id string) (*model.Plant, error) {
	if p, ok := f.m[id]; ok {
		return &p, nil
	}
	return nil, nil
}

type fakeArchiver struct {
	err     error
	calls   int
	removed []string
}

func (f *fakeArchiver) Remove(_ context.Context, key string) error {
	f.removed = append(f.removed, key)
	return nil
}

func (f *fakeArchiver) Archive(_ context.Context, url, prefix string) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	return fmt.Sprintf("%s/archived-%d.pdf", prefix, f.calls), nil
}

const user = "u1"

func newSvc() (*Service, *fakeArchiver) {
	arch := &fakeArchiver{}
	svc := NewService(
		&fakeLots{m: map[string]model.Lot{}},
		&fakeBatches{m: map[string]model.Batch{}},
		&fakePlants{m: map[string]model.Plant{
			"plant-bb":    {PlantID: "plant-bb", UserID: user},
			"plant-other": {PlantID: "plant-other", UserID: "someone-else"},
		}},
		arch,
	)
	return svc, arch
}

func isoLotReq() CreateLotRequest {
	return CreateLotRequest{
		Supplier: "Red Mesa", Product: "isolate", SupplierLotNumber: "2410",
		COAURL:  "https://lab.example/2410.pdf",
		Profile: model.Profile{Cannabinoids: map[string]float64{"cbd": 990, "d9thc": 0}},
	}
}

func mustLot(t *testing.T, svc *Service) model.Lot {
	t.Helper()
	res, err := svc.CreateLot(context.Background(), user, isoLotReq())
	if err != nil {
		t.Fatal(err)
	}
	return res.Lot
}

func isoBatchReq(lotID string) CreateBatchRequest {
	return CreateBatchRequest{
		Type: model.BatchISO, Date: "2026-10-24",
		Inputs:     []model.BatchInput{{Kind: model.InputLot, LotID: lotID, Grams: 3}},
		Carrier:    &model.Carrier{Name: "LouAna liquid coconut", Grams: 27},
		TotalGrams: 30, DensityGPerMl: 0.94,
	}
}

func cbd(t *testing.T, v BatchView) float64 {
	t.Helper()
	if v.Profile == nil {
		t.Fatalf("no profile: %s", v.ProfileError)
	}
	for _, c := range v.Profile.Compounds {
		if c.Key == "cbd" {
			return c.MgPerG
		}
	}
	t.Fatal("no cbd")
	return 0
}

// ── Lots ─────────────────────────────────────────────────────────────────────

func TestCreateLotArchivesCOA(t *testing.T) {
	svc, arch := newSvc()
	lot := mustLot(t, svc)
	if lot.COAS3Key == "" || arch.calls != 1 {
		t.Errorf("COA not archived: key=%q calls=%d", lot.COAS3Key, arch.calls)
	}
}

func TestCreateLotSavesWhenArchiveFails(t *testing.T) {
	svc, arch := newSvc()
	arch.err = errors.New("lab returned 404")
	res, err := svc.CreateLot(context.Background(), user, isoLotReq())
	if err != nil {
		t.Fatal(err)
	}
	if res.ArchiveError == "" || res.Lot.COAS3Key != "" {
		t.Errorf("want archive error and no key, got %+v", res)
	}
	got, _ := svc.GetLot(context.Background(), user, res.Lot.LotID)
	if got == nil || got.COAURL == "" {
		t.Error("lot should be saved with its link")
	}

	// Retried on the next update.
	arch.err = nil
	notes := "retry"
	upd, err := svc.UpdateLot(context.Background(), user, res.Lot.LotID, UpdateLotRequest{Notes: &notes})
	if err != nil {
		t.Fatal(err)
	}
	if upd.Lot.COAS3Key == "" {
		t.Error("archive should be retried on update")
	}
}

func TestLotUniqueBySupplierAndLotNumber(t *testing.T) {
	svc, _ := newSvc()
	mustLot(t, svc)

	dup := isoLotReq()
	dup.Supplier = "red mesa " // case and whitespace don't make it different
	if _, err := svc.CreateLot(context.Background(), user, dup); !errors.Is(err, ErrConflict) {
		t.Errorf("err = %v, want ErrConflict", err)
	}

	other := isoLotReq()
	other.Supplier = "Other Supplier"
	if _, err := svc.CreateLot(context.Background(), user, other); err != nil {
		t.Errorf("same lot number from another supplier should be allowed: %v", err)
	}
}

func TestGetLotOtherUserIsNotFound(t *testing.T) {
	svc, _ := newSvc()
	lot := mustLot(t, svc)
	if _, err := svc.GetLot(context.Background(), "intruder", lot.LotID); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// ── Batches ──────────────────────────────────────────────────────────────────

func TestCreateBatchBuildsCodeAndProfile(t *testing.T) {
	svc, _ := newSvc()
	lot := mustLot(t, svc)
	res, err := svc.CreateBatch(context.Background(), user, isoBatchReq(lot.LotID))
	if err != nil {
		t.Fatal(err)
	}
	if res.Batch.Code != "ISO-261024" {
		t.Errorf("code = %q", res.Batch.Code)
	}
	if got := cbd(t, res.BatchView); math.Abs(got-99) > 1e-9 {
		t.Errorf("CBD = %v, want 99", got)
	}
}

func TestCreateBatchDefaultsDensity(t *testing.T) {
	svc, _ := newSvc()
	lot := mustLot(t, svc)
	req := isoBatchReq(lot.LotID)
	req.DensityGPerMl = 0
	res, err := svc.CreateBatch(context.Background(), user, req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Batch.DensityGPerMl != model.DefaultCarrierDensity {
		t.Errorf("density = %v", res.Batch.DensityGPerMl)
	}
}

func TestBatchCodeConflictSuggestsDiscriminator(t *testing.T) {
	svc, _ := newSvc()
	lot := mustLot(t, svc)
	if _, err := svc.CreateBatch(context.Background(), user, isoBatchReq(lot.LotID)); err != nil {
		t.Fatal(err)
	}
	_, err := svc.CreateBatch(context.Background(), user, isoBatchReq(lot.LotID))
	if !errors.Is(err, ErrConflict) || !strings.Contains(err.Error(), "batchDiscriminator") {
		t.Fatalf("err = %v, want ErrConflict mentioning batchDiscriminator", err)
	}
	req := isoBatchReq(lot.LotID)
	req.BatchDiscriminator = "2"
	res, err := svc.CreateBatch(context.Background(), user, req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Batch.Code != "ISO-261024-2" {
		t.Errorf("code = %q", res.Batch.Code)
	}
}

func TestCreateBatchRejectsMissingAndForeignRefs(t *testing.T) {
	svc, _ := newSvc()
	if _, err := svc.CreateBatch(context.Background(), user, isoBatchReq("no-such-lot")); !errors.Is(err, ErrInvalid) {
		t.Errorf("missing lot: err = %v, want ErrInvalid", err)
	}

	flower := CreateBatchRequest{
		Type: model.BatchFS, SubType: "CBD", Discriminator: "BB", Date: "2026-07-17", TotalGrams: 100,
		Inputs: []model.BatchInput{{Kind: model.InputFlower, Grams: 10, Strain: "Berry Bliss",
			PlantID:    "plant-other",
			Profile:    &model.Profile{Cannabinoids: map[string]float64{"cbda": 120}},
			Efficiency: 0.7, Method: "MCT infusion", Confidence: model.ConfidenceMedium}},
	}
	if _, err := svc.CreateBatch(context.Background(), user, flower); !errors.Is(err, ErrInvalid) {
		t.Errorf("another user's plant: err = %v, want ErrInvalid", err)
	}
	flower.Inputs[0].PlantID = "plant-bb"
	if _, err := svc.CreateBatch(context.Background(), user, flower); err != nil {
		t.Errorf("own plant should be accepted: %v", err)
	}
}

func TestUpdateBatchRipplesAndRejectsCycles(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	lot := mustLot(t, svc)
	iso, err := svc.CreateBatch(ctx, user, isoBatchReq(lot.LotID))
	if err != nil {
		t.Fatal(err)
	}
	blend, err := svc.CreateBatch(ctx, user, CreateBatchRequest{
		Type: model.BatchBL, Date: "2026-11-07", TotalGrams: 10,
		Inputs: []model.BatchInput{{Kind: model.InputBatch, BatchID: iso.Batch.BatchID, Grams: 10}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := cbd(t, blend.BatchView); math.Abs(got-99) > 1e-9 {
		t.Fatalf("blend CBD = %v, want 99", got)
	}

	// Fix the ISO batch's yield by code: the blend's numbers follow.
	total := 33.0
	upd, err := svc.UpdateBatch(ctx, user, "iso-261024", UpdateBatchRequest{TotalGrams: &total})
	if err != nil {
		t.Fatal(err)
	}
	if len(upd.Affected) != 1 || upd.Affected[0].Code != "BL-261107" {
		t.Errorf("affected = %+v, want [BL-261107]", upd.Affected)
	}
	got, err := svc.GetBatch(ctx, user, "BL-261107")
	if err != nil {
		t.Fatal(err)
	}
	if c := cbd(t, *got); math.Abs(c-90) > 1e-9 {
		t.Errorf("blend CBD after update = %v, want 90", c)
	}

	// A notes-only edit doesn't report a ripple.
	notes := "smooth"
	upd, err = svc.UpdateBatch(ctx, user, iso.Batch.BatchID, UpdateBatchRequest{Notes: &notes})
	if err != nil {
		t.Fatal(err)
	}
	if len(upd.Affected) != 0 {
		t.Errorf("notes edit reported affected = %+v", upd.Affected)
	}

	// Pointing the ISO batch at the blend would make a loop.
	cyc := []model.BatchInput{{Kind: model.InputBatch, BatchID: blend.Batch.BatchID, Grams: 1}}
	if _, err := svc.UpdateBatch(ctx, user, iso.Batch.BatchID, UpdateBatchRequest{Inputs: &cyc}); !errors.Is(err, ErrInvalid) {
		t.Errorf("cycle: err = %v, want ErrInvalid", err)
	}
}

func TestUpdateLotProfileRipples(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	lot := mustLot(t, svc)
	if _, err := svc.CreateBatch(ctx, user, isoBatchReq(lot.LotID)); err != nil {
		t.Fatal(err)
	}
	p := model.Profile{Cannabinoids: map[string]float64{"cbd": 995, "d9thc": 0}}
	res, err := svc.UpdateLot(ctx, user, lot.LotID, UpdateLotRequest{Profile: &p})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Affected) != 1 || res.Affected[0].Code != "ISO-261024" {
		t.Errorf("affected = %+v", res.Affected)
	}
}

func TestDose(t *testing.T) {
	svc, _ := newSvc()
	lot := mustLot(t, svc)
	if _, err := svc.CreateBatch(context.Background(), user, isoBatchReq(lot.LotID)); err != nil {
		t.Fatal(err)
	}
	d, err := svc.Dose(context.Background(), user, "ISO-261024", "cbd", 100)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(d.Grams-100.0/99) > 1e-9 || math.Abs(d.Ml-100.0/99/0.94) > 1e-9 {
		t.Errorf("dose = %+v", d)
	}
	if _, err := svc.Dose(context.Background(), user, "ISO-261024", "cbg", 10); !errors.Is(err, ErrInvalid) {
		t.Errorf("missing compound: err = %v, want ErrInvalid", err)
	}
}

// ── Deletes ──────────────────────────────────────────────────────────────────

func TestDeleteRefusedWhileInUse(t *testing.T) {
	svc, arch := newSvc()
	ctx := context.Background()
	lot := mustLot(t, svc)
	iso, err := svc.CreateBatch(ctx, user, isoBatchReq(lot.LotID))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateBatch(ctx, user, CreateBatchRequest{
		Type: model.BatchBL, Date: "2026-11-07", TotalGrams: 10,
		Inputs: []model.BatchInput{{Kind: model.InputBatch, BatchID: iso.Batch.BatchID, Grams: 10}},
	}); err != nil {
		t.Fatal(err)
	}

	// Lot is used by ISO-261024; ISO-261024 is used by BL-261107.
	err = svc.DeleteLot(ctx, user, lot.LotID)
	if !errors.Is(err, ErrConflict) || !strings.Contains(err.Error(), "ISO-261024") {
		t.Errorf("delete used lot: err = %v, want ErrConflict naming ISO-261024", err)
	}
	_, err = svc.DeleteBatch(ctx, user, "ISO-261024")
	if !errors.Is(err, ErrConflict) || !strings.Contains(err.Error(), "BL-261107") {
		t.Errorf("delete used batch: err = %v, want ErrConflict naming BL-261107", err)
	}

	// Remove from the top down: blend, then ISO, then the lot.
	for _, code := range []string{"BL-261107", "ISO-261024"} {
		if _, err := svc.DeleteBatch(ctx, user, code); err != nil {
			t.Fatalf("delete %s: %v", code, err)
		}
	}
	if err := svc.DeleteLot(ctx, user, lot.LotID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetLot(ctx, user, lot.LotID); !errors.Is(err, ErrNotFound) {
		t.Errorf("lot still there: %v", err)
	}
	if len(arch.removed) != 1 || arch.removed[0] != lot.COAS3Key {
		t.Errorf("archived COA removed = %v, want [%s]", arch.removed, lot.COAS3Key)
	}
}

func TestDeleteOtherUsersIsNotFound(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	lot := mustLot(t, svc)
	if _, err := svc.CreateBatch(ctx, user, isoBatchReq(lot.LotID)); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteLot(ctx, "intruder", lot.LotID); !errors.Is(err, ErrNotFound) {
		t.Errorf("lot: err = %v, want ErrNotFound", err)
	}
	if _, err := svc.DeleteBatch(ctx, "intruder", "ISO-261024"); !errors.Is(err, ErrNotFound) {
		t.Errorf("batch: err = %v, want ErrNotFound", err)
	}
}

// ── Flower COAs ──────────────────────────────────────────────────────────────

func flowerBatchReq(coaURL string) CreateBatchRequest {
	return CreateBatchRequest{
		Type: model.BatchFS, SubType: "CBD", Discriminator: "BB", Date: "2026-07-17", TotalGrams: 100,
		Inputs: []model.BatchInput{{Kind: model.InputFlower, Grams: 14, Strain: "Berry Bliss",
			Profile:    &model.Profile{Cannabinoids: map[string]float64{"cbda": 150}},
			Efficiency: 0.7, Method: "MCT infusion", Confidence: model.ConfidenceMedium,
			COAURL: coaURL, COAS3Key: "client-supplied-should-be-ignored"}},
	}
}

func TestFlowerCOALifecycle(t *testing.T) {
	svc, arch := newSvc()
	ctx := context.Background()

	res, err := svc.CreateBatch(ctx, user, flowerBatchReq("https://rogue.example/bb.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	key := res.Batch.Inputs[0].COAS3Key
	if key == "" || key == "client-supplied-should-be-ignored" || arch.calls != 1 {
		t.Fatalf("flower COA not archived by us: key=%q calls=%d", key, arch.calls)
	}

	// Re-sending the same inputs (e.g. to fix grams) keeps the archived copy.
	inputs := flowerBatchReq("https://rogue.example/bb.jpg").Inputs
	inputs[0].Grams = 15
	upd, err := svc.UpdateBatch(ctx, user, "FS-CBD-BB-260717", UpdateBatchRequest{Inputs: &inputs})
	if err != nil {
		t.Fatal(err)
	}
	if upd.Batch.Inputs[0].COAS3Key != key || arch.calls != 1 || len(arch.removed) != 0 {
		t.Errorf("unchanged link re-archived or removed: key=%q calls=%d removed=%v",
			upd.Batch.Inputs[0].COAS3Key, arch.calls, arch.removed)
	}

	// A new link archives the new file and cleans up the old copy.
	inputs = flowerBatchReq("https://rogue.example/bb-v2.jpg").Inputs
	upd, err = svc.UpdateBatch(ctx, user, "FS-CBD-BB-260717", UpdateBatchRequest{Inputs: &inputs})
	if err != nil {
		t.Fatal(err)
	}
	newKey := upd.Batch.Inputs[0].COAS3Key
	if newKey == "" || arch.calls != 2 || len(arch.removed) != 1 || arch.removed[0] != key {
		t.Errorf("link change: newKey=%q calls=%d removed=%v (want old %q removed)", newKey, arch.calls, arch.removed, key)
	}

	// Deleting the batch removes its archived flower COA.
	if _, err := svc.DeleteBatch(ctx, user, "FS-CBD-BB-260717"); err != nil {
		t.Fatal(err)
	}
	if arch.removed[len(arch.removed)-1] != newKey {
		t.Errorf("removed = %v, want %q last", arch.removed, newKey)
	}
}

func TestFlowerCOAArchiveFailureStillSaves(t *testing.T) {
	svc, arch := newSvc()
	arch.err = errors.New("returned 403 Forbidden")
	res, err := svc.CreateBatch(context.Background(), user, flowerBatchReq("https://rogue.example/bb.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.ArchiveError, "Berry Bliss") || res.Batch.Inputs[0].COAURL == "" {
		t.Errorf("archiveError = %q, link = %q", res.ArchiveError, res.Batch.Inputs[0].COAURL)
	}
}

func TestCOAURLOnlyOnFlower(t *testing.T) {
	svc, _ := newSvc()
	lot := mustLot(t, svc)
	req := isoBatchReq(lot.LotID)
	req.Inputs[0].COAURL = "https://lab.example/x.pdf"
	if _, err := svc.CreateBatch(context.Background(), user, req); !errors.Is(err, ErrInvalid) {
		t.Errorf("err = %v, want ErrInvalid", err)
	}
}
