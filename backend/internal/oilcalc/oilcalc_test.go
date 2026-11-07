package oilcalc

import (
	"errors"
	"math"
	"testing"

	"github.com/cdargis/grow/internal/model"
)

func approx(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func compound(t *testing.T, p Profile, key string) Compound {
	t.Helper()
	for _, c := range p.Compounds {
		if c.Key == key {
			return c
		}
	}
	t.Fatalf("compound %q not in profile %s", key, p.Code)
	return Compound{}
}

// Red Mesa isolate: 99.0% CBD, THC not detected.
var isoLot = model.Lot{
	LotID: "lot-iso", Supplier: "Red Mesa", Product: "isolate", SupplierLotNumber: "2410",
	Profile: model.Profile{Cannabinoids: map[string]float64{"cbd": 990, "d9thc": 0}},
}

// fsOil is a flower oil at exactly 15 mg/g CBD, medium confidence: 10 g of
// flower at 150 mg/g (already neutral CBD, so no decarb), full efficiency,
// 100 g of oil.
var fsOil = model.Batch{
	BatchID: "b-fs", Code: "FS-CBD-BB-260717", Type: model.BatchFS,
	Inputs: []model.BatchInput{{
		Kind: model.InputFlower, Grams: 10, Strain: "Berry Bliss",
		Profile:    &model.Profile{Cannabinoids: map[string]float64{"cbd": 150}},
		Efficiency: 1, Method: "MCT infusion", Confidence: model.ConfidenceMedium,
	}},
	DensityGPerMl: 0.94, TotalGrams: 100,
}

// The worked example from the plan: 99 g FS (medium) + 1 g isolate →
// CBD 24.75 mg/g, 60% medium, 40% lab.
func TestWorkedExampleSharesByContribution(t *testing.T) {
	blend := model.Batch{
		BatchID: "b-bl", Code: "BL-261107", Type: model.BatchBL,
		Inputs: []model.BatchInput{
			{Kind: model.InputBatch, BatchID: "b-fs", Grams: 99},
			{Kind: model.InputLot, LotID: "lot-iso", Grams: 1},
		},
		DensityGPerMl: 0.94, TotalGrams: 100,
	}
	c := New([]model.Lot{isoLot}, []model.Batch{fsOil, blend})
	p, err := c.Batch("b-bl")
	if err != nil {
		t.Fatal(err)
	}
	cbd := compound(t, p, "cbd")
	approx(t, "CBD mg/g", cbd.MgPerG, 24.75)
	approx(t, "CBD mg/mL", cbd.MgPerMl, 24.75*0.94)
	approx(t, "medium share", cbd.Shares[SourceMedium], 0.6)
	approx(t, "lab share", cbd.Shares[SourceLab], 0.4)
}

// ISO oil: isolate diluted in carrier is plain arithmetic, so it's 100% lab.
// THC "not detected" stays on the profile at 0, attributed to lab.
func TestISODilutionIsLab(t *testing.T) {
	iso := model.Batch{
		BatchID: "b-iso", Code: "ISO-261024", Type: model.BatchISO,
		Inputs:        []model.BatchInput{{Kind: model.InputLot, LotID: "lot-iso", Grams: 3}},
		Carrier:       &model.Carrier{Name: "LouAna liquid coconut", Grams: 27},
		DensityGPerMl: 0.94, TotalGrams: 30,
	}
	c := New([]model.Lot{isoLot}, []model.Batch{iso})
	p, err := c.Batch("b-iso")
	if err != nil {
		t.Fatal(err)
	}
	cbd := compound(t, p, "cbd")
	approx(t, "CBD mg/g", cbd.MgPerG, 99)
	approx(t, "CBD mg/mL", cbd.MgPerMl, 93.06)
	approx(t, "CBD lab share", cbd.Shares[SourceLab], 1)

	thc := compound(t, p, "d9thc")
	approx(t, "THC mg/g", thc.MgPerG, 0)
	approx(t, "THC lab share (not detected)", thc.Shares[SourceLab], 1)
	approx(t, "Total THC", p.TotalTHC.MgPerG, 0)
}

// An ISO batch used in a blend carries its lab provenance through.
func TestBatchOfBatchCarriesProvenance(t *testing.T) {
	iso := model.Batch{
		BatchID: "b-iso", Code: "ISO-261024", Type: model.BatchISO,
		Inputs:        []model.BatchInput{{Kind: model.InputLot, LotID: "lot-iso", Grams: 3}},
		DensityGPerMl: 0.94, TotalGrams: 30,
	}
	blend := model.Batch{
		BatchID: "b-bl", Code: "BL-261107", Type: model.BatchBL,
		Inputs: []model.BatchInput{
			{Kind: model.InputBatch, BatchID: "b-iso", Grams: 10}, // 990 mg CBD, lab
			{Kind: model.InputBatch, BatchID: "b-fs", Grams: 10},  // 150 mg CBD, medium
		},
		DensityGPerMl: 0.94, TotalGrams: 20,
	}
	c := New([]model.Lot{isoLot}, []model.Batch{iso, fsOil, blend})
	p, err := c.Batch("b-bl")
	if err != nil {
		t.Fatal(err)
	}
	cbd := compound(t, p, "cbd")
	approx(t, "CBD mg/g", cbd.MgPerG, 57)
	approx(t, "lab share", cbd.Shares[SourceLab], 990.0/1140)
	approx(t, "medium share", cbd.Shares[SourceMedium], 150.0/1140)
}

// Per-compound provenance: ISO + THC flower oil is ~0% estimated for CBD but
// 100% estimated for THC.
func TestPerCompoundProvenance(t *testing.T) {
	thcOil := model.Batch{
		BatchID: "b-thc", Code: "THC-TST-000000", Type: model.BatchTHC,
		Inputs: []model.BatchInput{{
			Kind: model.InputFlower, Grams: 5, Strain: "dispensary blend",
			Profile:    &model.Profile{Cannabinoids: map[string]float64{"thca": 200}},
			Efficiency: 0.8, Method: "stovetop", Confidence: model.ConfidenceLow,
		}},
		DensityGPerMl: 0.94, TotalGrams: 50,
	}
	blend := model.Batch{
		BatchID: "b-bl", Code: "BL-261107", Type: model.BatchBL,
		Inputs: []model.BatchInput{
			{Kind: model.InputLot, LotID: "lot-iso", Grams: 1},
			{Kind: model.InputBatch, BatchID: "b-thc", Grams: 10},
		},
		DensityGPerMl: 0.94, TotalGrams: 11,
	}
	c := New([]model.Lot{isoLot}, []model.Batch{thcOil, blend})
	p, err := c.Batch("b-bl")
	if err != nil {
		t.Fatal(err)
	}
	approx(t, "CBD lab share", compound(t, p, "cbd").Shares[SourceLab], 1)
	thc := compound(t, p, "d9thc")
	approx(t, "THC low share", thc.Shares[SourceLow], 1)
	if _, ok := thc.Shares[SourceLab]; ok {
		t.Errorf("THC should have no lab share (isolate's THC was not detected), got %v", thc.Shares)
	}
}

// Flower acids are decarbed (×0.877) and scaled by efficiency.
func TestFlowerDecarbAndEfficiency(t *testing.T) {
	b := model.Batch{
		BatchID: "b-fs", Code: "FS-261107", Type: model.BatchFS,
		Inputs: []model.BatchInput{{
			Kind: model.InputFlower, Grams: 20, Strain: "Rogue Origin",
			Profile:    &model.Profile{Cannabinoids: map[string]float64{"cbda": 100, "cbd": 5}},
			Efficiency: 0.5, Method: "MCT infusion", Confidence: model.ConfidenceMedium,
		}},
		DensityGPerMl: 0.95, TotalGrams: 100,
	}
	p, err := New(nil, []model.Batch{b}).Batch("b-fs")
	if err != nil {
		t.Fatal(err)
	}
	// (100*0.877 + 5) * 20 g * 0.5 / 100 g
	approx(t, "CBD mg/g", compound(t, p, "cbd").MgPerG, (87.7+5)*20*0.5/100)
	for _, c := range p.Compounds {
		if c.Key == "cbda" {
			t.Errorf("CBDA should have been decarbed into CBD, found %v", c)
		}
	}
}

// A batch's own lab COA replaces the calculated numbers and counts as lab
// downstream; the calculated numbers stay available for comparison.
func TestLabCOAOverrides(t *testing.T) {
	tested := fsOil
	tested.LabCOA = &model.LabCOA{URL: "https://lab.example/coa.pdf",
		Profile: model.Profile{Cannabinoids: map[string]float64{"cbd": 12}}}
	blend := model.Batch{
		BatchID: "b-bl", Code: "BL-261107", Type: model.BatchBL,
		Inputs:        []model.BatchInput{{Kind: model.InputBatch, BatchID: "b-fs", Grams: 10}},
		DensityGPerMl: 0.94, TotalGrams: 10,
	}
	c := New(nil, []model.Batch{tested, blend})

	p, err := c.Batch("b-fs")
	if err != nil {
		t.Fatal(err)
	}
	if !p.FromLabCOA {
		t.Error("expected FromLabCOA")
	}
	approx(t, "lab CBD", compound(t, p, "cbd").MgPerG, 12)
	if len(p.Calculated) != 1 || p.Calculated[0].MgPerG != 15 {
		t.Errorf("calculated = %+v, want CBD 15 for comparison", p.Calculated)
	}

	bp, err := c.Batch("b-bl")
	if err != nil {
		t.Fatal(err)
	}
	approx(t, "downstream CBD", compound(t, bp, "cbd").MgPerG, 12)
	approx(t, "downstream lab share", compound(t, bp, "cbd").Shares[SourceLab], 1)
}

func TestTotals(t *testing.T) {
	lot := model.Lot{LotID: "lot-dis", Profile: model.Profile{Cannabinoids: map[string]float64{
		"cbd": 600, "cbda": 100, "d9thc": 2, "thca": 1,
	}}}
	b := model.Batch{
		BatchID: "b-dis", Code: "DIS-261024", Type: model.BatchDIS,
		Inputs:        []model.BatchInput{{Kind: model.InputLot, LotID: "lot-dis", Grams: 1}},
		DensityGPerMl: 0.95, TotalGrams: 10,
	}
	p, err := New([]model.Lot{lot}, []model.Batch{b}).Batch("b-dis")
	if err != nil {
		t.Fatal(err)
	}
	approx(t, "Total CBD", p.TotalCBD.MgPerG, 60+10*0.877)
	approx(t, "Total THC", p.TotalTHC.MgPerG, 0.2+0.1*0.877)
	approx(t, "Total THC mg/mL", p.TotalTHC.MgPerMl, (0.2+0.1*0.877)*0.95)
}

func TestCBDFirstInDisplayOrder(t *testing.T) {
	lot := model.Lot{LotID: "l", Profile: model.Profile{
		Cannabinoids: map[string]float64{"cbn": 1, "d9thc": 1, "cbd": 1},
		Terpenes:     map[string]float64{"limonene": 1},
		Other:        []model.NamedAmount{{Name: "Zeta", MgPerG: 1}},
	}}
	b := model.Batch{BatchID: "b", Code: "DIS-261024",
		Inputs: []model.BatchInput{{Kind: model.InputLot, LotID: "l", Grams: 1}}, DensityGPerMl: 0.95, TotalGrams: 1}
	p, err := New([]model.Lot{lot}, []model.Batch{b}).Batch("b")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range p.Compounds {
		got = append(got, c.Key)
	}
	want := []string{"cbd", "d9thc", "cbn", "limonene", "zeta"}
	if len(got) != len(want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestCycleDetected(t *testing.T) {
	a := model.Batch{BatchID: "a", Code: "BL-261101",
		Inputs: []model.BatchInput{{Kind: model.InputBatch, BatchID: "b", Grams: 1}}, DensityGPerMl: 0.95, TotalGrams: 1}
	b := model.Batch{BatchID: "b", Code: "BL-261102",
		Inputs: []model.BatchInput{{Kind: model.InputBatch, BatchID: "a", Grams: 1}}, DensityGPerMl: 0.95, TotalGrams: 1}
	_, err := New(nil, []model.Batch{a, b}).Batch("a")
	if !errors.Is(err, ErrCycle) {
		t.Fatalf("err = %v, want ErrCycle", err)
	}
}

func TestMissingLot(t *testing.T) {
	b := model.Batch{BatchID: "b", Code: "ISO-261024",
		Inputs: []model.BatchInput{{Kind: model.InputLot, LotID: "nope", Grams: 1}}, DensityGPerMl: 0.95, TotalGrams: 1}
	_, err := New(nil, []model.Batch{b}).Batch("b")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDependents(t *testing.T) {
	iso := model.Batch{BatchID: "iso", Inputs: []model.BatchInput{{Kind: model.InputLot, LotID: "lot-iso", Grams: 1}}}
	bl1 := model.Batch{BatchID: "bl1", Inputs: []model.BatchInput{{Kind: model.InputBatch, BatchID: "iso", Grams: 1}}}
	bl2 := model.Batch{BatchID: "bl2", Inputs: []model.BatchInput{{Kind: model.InputBatch, BatchID: "bl1", Grams: 1}}}
	other := model.Batch{BatchID: "other", Inputs: []model.BatchInput{{Kind: model.InputLot, LotID: "x", Grams: 1}}}
	c := New(nil, []model.Batch{iso, bl1, bl2, other})

	got := c.Dependents("iso")
	if len(got) != 2 || got[0] != "bl1" || got[1] != "bl2" {
		t.Errorf("Dependents(iso) = %v, want [bl1 bl2]", got)
	}
	got = c.LotDependents("lot-iso")
	if len(got) != 3 || got[0] != "bl1" || got[1] != "bl2" || got[2] != "iso" {
		t.Errorf("LotDependents(lot-iso) = %v, want [bl1 bl2 iso]", got)
	}
}

func TestDose(t *testing.T) {
	d, err := Dose(24.75, 0.94, 100)
	if err != nil {
		t.Fatal(err)
	}
	approx(t, "grams", d.Grams, 100/24.75)
	approx(t, "ml", d.Ml, 100/24.75/0.94)

	if _, err := Dose(0, 0.94, 100); err == nil {
		t.Error("expected error for a batch with none of the compound")
	}
}
