package dose

import (
	"fmt"
	"sort"

	"github.com/cdargis/grow/internal/model"
	"github.com/cdargis/grow/internal/oilcalc"
)

// Strength is what one unit (one mL, or one piece) of a shelf item's
// current source delivers. Built from a batch's ROBP or a product's label.
type Strength struct {
	Unit     model.ProductUnit `json:"unit"`               // mL | piece
	UnitName string            `json:"unitName,omitempty"` // "gummy" for pieces
	Label    string            `json:"label"`              // batch code or product name
	BatchID  string            `json:"batchId,omitempty"`
	// Cannabinoids and terpenes per unit, in display order. Totals use the
	// ROBP convention.
	Cannabinoids []Amount `json:"cannabinoids"`
	Terpenes     []Amount `json:"terpenes"`
	TotalCBD     float64  `json:"totalCbd"`
	TotalTHC     float64  `json:"totalThc"`
	// Provenance of the totals; only batches have it (products are a label).
	SharesCBD map[oilcalc.Source]float64 `json:"sharesCbd,omitempty"`
	SharesTHC map[oilcalc.Source]float64 `json:"sharesThc,omitempty"`
	// LotGrams is grams of each raw lot per unit; batches only.
	LotGrams map[string]float64 `json:"-"`
}

func (s Strength) total(c model.DoseCompound) float64 {
	if c == model.DoseTHC {
		return s.TotalTHC
	}
	return s.TotalCBD
}

func (s Strength) shares(c model.DoseCompound) map[oilcalc.Source]float64 {
	if c == model.DoseTHC {
		return s.SharesTHC
	}
	return s.SharesCBD
}

type Amount struct {
	Key  string  `json:"key"`
	Name string  `json:"name"`
	Mg   float64 `json:"mg"`
}

type LineView struct {
	ShelfItemID string             `json:"shelfItemId"`
	ShelfName   string             `json:"shelfName"`
	Compound    model.DoseCompound `json:"compound"`
	Mg          float64            `json:"mg"`
	Note        string             `json:"note,omitempty"`
	// Amount of the current source that delivers Mg, in Unit. Zero when
	// Error is set.
	Amount   float64                    `json:"amount"`
	Unit     model.ProductUnit          `json:"unit,omitempty"`
	UnitName string                     `json:"unitName,omitempty"`
	Source   string                     `json:"source,omitempty"` // batch code or product name
	BatchID  string                     `json:"batchId,omitempty"`
	Shares   map[oilcalc.Source]float64 `json:"shares,omitempty"`
	Error    string                     `json:"error,omitempty"`
}

type TimeView struct {
	Name     string     `json:"name"`
	Lines    []LineView `json:"lines"`
	TotalCBD float64    `json:"totalCbd"`
	TotalTHC float64    `json:"totalThc"`
}

type ShelfUsage struct {
	ShelfItemID string            `json:"shelfItemId"`
	Name        string            `json:"name"`
	Unit        model.ProductUnit `json:"unit"`
	UnitName    string            `json:"unitName,omitempty"`
	PerDay      float64           `json:"perDay"`
	Per30Days   float64           `json:"per30Days"`
}

type LotUsage struct {
	LotID     string  `json:"lotId"`
	Label     string  `json:"label"` // "Red Mesa · isolate · R260611MS08"
	Per30Days float64 `json:"gramsPer30Days"`
}

type VersionView struct {
	Version model.DoseVersion `json:"version"`
	Status  model.DoseStatus  `json:"status"`
	Times   []TimeView        `json:"times"`
	// Day totals: every cannabinoid delivered, plus the two totals and the
	// sum of minors (everything but CBD/CBDA/Δ9-THC/THCA).
	Cannabinoids []Amount `json:"cannabinoids"`
	TotalCBD     float64  `json:"totalCbd"`
	TotalTHC     float64  `json:"totalThc"`
	Minors       float64  `json:"minors"`
	// Terpenes delivered per day, and their sum.
	Terpenes      []Amount     `json:"terpenes"`
	TotalTerpenes float64      `json:"totalTerpenes"`
	Shelf         []ShelfUsage `json:"shelf"`
	RawLots       []LotUsage   `json:"rawLots"`
}

const daysPerMonth = 30

var majorKeys = map[string]bool{"cbd": true, "cbda": true, "d9thc": true, "thca": true}

// Derive turns a version into what the dose card shows, using each shelf
// item's current strength. A line whose strength is missing (a batch whose
// profile can't be calculated, a source with none of the compound) carries
// an Error and contributes nothing, rather than failing the whole plan.
func Derive(v model.DoseVersion, today string, items map[string]model.ShelfItem,
	strengths map[string]Strength, strengthErrs map[string]string, lotLabels map[string]string) VersionView {

	out := VersionView{Version: v, Status: v.Status(today)}
	dayMg := map[string]float64{}
	perDay := map[string]float64{} // shelf item → units/day
	lotPerDay := map[string]float64{}

	for _, t := range v.Times {
		tv := TimeView{Name: t.Name, Lines: make([]LineView, 0, len(t.Lines))}
		for _, l := range t.Lines {
			lv := LineView{ShelfItemID: l.ShelfItemID, Compound: l.Compound, Mg: l.Mg, Note: l.Note}
			item, ok := items[l.ShelfItemID]
			if !ok {
				lv.Error = "shelf item not found"
				tv.Lines = append(tv.Lines, lv)
				continue
			}
			lv.ShelfName = item.Name
			s, ok := strengths[l.ShelfItemID]
			if !ok {
				lv.Error = strengthErrs[l.ShelfItemID]
				if lv.Error == "" {
					lv.Error = "no strength for this shelf item"
				}
				tv.Lines = append(tv.Lines, lv)
				continue
			}
			lv.Unit, lv.UnitName, lv.Source, lv.BatchID = s.Unit, s.UnitName, s.Label, s.BatchID
			lv.Shares = s.shares(l.Compound)
			per := s.total(l.Compound)
			if per <= 0 {
				lv.Error = fmt.Sprintf("%s has no %s", s.Label, compoundName(l.Compound))
				tv.Lines = append(tv.Lines, lv)
				continue
			}
			lv.Amount = l.Mg / per
			tv.TotalCBD += lv.Amount * s.TotalCBD
			tv.TotalTHC += lv.Amount * s.TotalTHC
			for _, a := range s.Cannabinoids {
				dayMg[a.Key] += lv.Amount * a.Mg
			}
			for _, a := range s.Terpenes {
				dayMg[a.Key] += lv.Amount * a.Mg
			}
			perDay[l.ShelfItemID] += lv.Amount
			for lot, g := range s.LotGrams {
				lotPerDay[lot] += lv.Amount * g
			}
			tv.Lines = append(tv.Lines, lv)
		}
		out.TotalCBD += tv.TotalCBD
		out.TotalTHC += tv.TotalTHC
		out.Times = append(out.Times, tv)
	}

	for _, d := range model.KnownCannabinoids {
		mg, ok := dayMg[d.Key]
		if !ok {
			continue
		}
		out.Cannabinoids = append(out.Cannabinoids, Amount{Key: d.Key, Name: d.Name, Mg: mg})
		if !majorKeys[d.Key] {
			out.Minors += mg
		}
	}

	for _, d := range model.KnownTerpenes {
		if mg, ok := dayMg[d.Key]; ok {
			out.Terpenes = append(out.Terpenes, Amount{Key: d.Key, Name: d.Name, Mg: mg})
			out.TotalTerpenes += mg
		}
	}

	// Shelf usage in the order items first appear in the plan.
	seen := map[string]bool{}
	for _, t := range v.Times {
		for _, l := range t.Lines {
			units, ok := perDay[l.ShelfItemID]
			if !ok || seen[l.ShelfItemID] {
				continue
			}
			seen[l.ShelfItemID] = true
			s := strengths[l.ShelfItemID]
			out.Shelf = append(out.Shelf, ShelfUsage{
				ShelfItemID: l.ShelfItemID, Name: items[l.ShelfItemID].Name,
				Unit: s.Unit, UnitName: s.UnitName,
				PerDay: units, Per30Days: units * daysPerMonth,
			})
		}
	}

	for lot, g := range lotPerDay {
		label := lotLabels[lot]
		if label == "" {
			label = lot
		}
		out.RawLots = append(out.RawLots, LotUsage{LotID: lot, Label: label, Per30Days: g * daysPerMonth})
	}
	sort.Slice(out.RawLots, func(i, j int) bool { return out.RawLots[i].Label < out.RawLots[j].Label })
	if out.Times == nil {
		out.Times = []TimeView{}
	}
	if out.Cannabinoids == nil {
		out.Cannabinoids = []Amount{}
	}
	if out.Terpenes == nil {
		out.Terpenes = []Amount{}
	}
	if out.Shelf == nil {
		out.Shelf = []ShelfUsage{}
	}
	if out.RawLots == nil {
		out.RawLots = []LotUsage{}
	}
	return out
}

func compoundName(c model.DoseCompound) string {
	if c == model.DoseTHC {
		return "THC"
	}
	return "CBD"
}

// BatchStrength builds a per-mL strength from a batch's ROBP and its raw lot
// trace (grams of lot per gram of batch).
func BatchStrength(p oilcalc.Profile, lotGramsPerG map[string]float64) Strength {
	s := Strength{
		Unit: model.ProductPerMl, Label: p.Code, BatchID: p.BatchID,
		TotalCBD: p.TotalCBD.MgPerMl, TotalTHC: p.TotalTHC.MgPerMl,
		SharesCBD: p.TotalCBD.Shares, SharesTHC: p.TotalTHC.Shares,
		LotGrams: make(map[string]float64, len(lotGramsPerG)),
	}
	for _, c := range p.Compounds {
		switch c.Group {
		case "cannabinoid":
			s.Cannabinoids = append(s.Cannabinoids, Amount{Key: c.Key, Name: c.Name, Mg: c.MgPerMl})
		case "terpene":
			s.Terpenes = append(s.Terpenes, Amount{Key: c.Key, Name: c.Name, Mg: c.MgPerMl})
		}
	}
	for lot, g := range lotGramsPerG {
		s.LotGrams[lot] = g * p.DensityGPerMl
	}
	return s
}

// ProductStrength builds a strength from a product's label.
func ProductStrength(src model.ShelfSource) Strength {
	s := Strength{Unit: src.Unit, UnitName: src.UnitName, Label: src.ProductName}
	for _, d := range model.KnownCannabinoids {
		if mg, ok := src.MgPerUnit[d.Key]; ok {
			s.Cannabinoids = append(s.Cannabinoids, Amount{Key: d.Key, Name: d.Name, Mg: mg})
		}
	}
	for _, d := range model.KnownTerpenes {
		if mg, ok := src.MgPerUnit[d.Key]; ok {
			s.Terpenes = append(s.Terpenes, Amount{Key: d.Key, Name: d.Name, Mg: mg})
		}
	}
	m := src.MgPerUnit
	s.TotalCBD = m["cbd"] + 0.877*m["cbda"]
	s.TotalTHC = m["d9thc"] + 0.877*m["thca"]
	return s
}
