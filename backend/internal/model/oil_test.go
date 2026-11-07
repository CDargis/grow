package model

import (
	"strings"
	"testing"
)

func TestBuildBatchCode(t *testing.T) {
	cases := []struct {
		name                       string
		typ                        BatchType
		sub, disc, date, batchDisc string
		want                       string
	}{
		{"distillate", BatchDIS, "", "", "2026-10-24", "", "DIS-261024"},
		{"isolate", BatchISO, "", "", "2026-10-24", "", "ISO-261024"},
		{"homegrow CBD plant", BatchFS, "CBD", "BB", "2026-07-17", "", "FS-CBD-BB-260717"},
		{"unknown date", BatchTHC, "TST", "", "", "", "THC-TST-000000"},
		{"second same day", BatchDIS, "", "", "2026-10-24", "2", "DIS-261024-2"},
		{"lowercase parts", BatchFS, "cbd", "bb", "2026-07-17", "", "FS-CBD-BB-260717"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := BuildBatchCode(c.typ, c.sub, c.disc, c.date, c.batchDisc)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestBuildBatchCodeRejects(t *testing.T) {
	cases := []struct {
		name                       string
		typ                        BatchType
		sub, disc, date, batchDisc string
	}{
		{"unknown type", "DIST", "", "", "2026-10-24", ""},
		{"hyphen in part", BatchFS, "CBD-BB", "", "2026-07-17", ""},
		{"bad date", BatchDIS, "", "", "261024", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := BuildBatchCode(c.typ, c.sub, c.disc, c.date, c.batchDisc); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestProfileValidate(t *testing.T) {
	ok := Profile{
		Cannabinoids: map[string]float64{"cbd": 992, "d9thc": 0},
		Terpenes:     map[string]float64{"limonene": 3.1},
		Other:        []NamedAmount{{Name: "CBT", MgPerG: 0.4}},
	}
	if err := ok.Validate(); err != nil {
		t.Errorf("valid profile rejected: %v", err)
	}

	bad := Profile{
		Cannabinoids: map[string]float64{"cbt": 1, "cbd": 1200},
		Other:        []NamedAmount{{Name: " ", MgPerG: 1}},
	}
	err := bad.Validate()
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, want := range []string{`unknown cannabinoid "cbt"`, `"cbd" = 1200`, "empty name"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}
}

func TestBatchValidateFlower(t *testing.T) {
	b := Batch{
		Type: BatchFS, Date: "2026-07-17", DensityGPerMl: 0.94, TotalGrams: 100,
		Inputs: []BatchInput{{Kind: InputFlower, Grams: 10, Strain: "Berry Bliss",
			Profile: &Profile{Cannabinoids: map[string]float64{"cbda": 120}}}},
	}
	err := b.Validate()
	if err == nil {
		t.Fatal("expected errors for missing efficiency, method and confidence")
	}
	for _, want := range []string{"efficiency", "method", "confidence"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}

	b.Inputs[0].Efficiency = 0.7
	b.Inputs[0].Method = "MCT infusion"
	b.Inputs[0].Confidence = ConfidenceMedium
	if err := b.Validate(); err != nil {
		t.Errorf("valid batch rejected: %v", err)
	}
}
