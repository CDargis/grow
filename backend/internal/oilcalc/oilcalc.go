// Package oilcalc calculates batch profiles (the ROBP -- Record of Batch
// Profile) from a batch's inputs. Profiles are never stored: they are
// calculated on read from the current lots and batches, so an edit to any
// batch flows into every blend made from it.
//
// Besides mg/g, every compound carries a provenance split: what share of
// that compound's mg came from lab-backed numbers (lot COAs, batch lab COAs,
// additives) versus flower estimates, broken down by the method's confidence.
// Shares are weighted by contribution (mg), not by grams of input.
package oilcalc

import (
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/cdargis/grow/internal/model"
)

// Source is a provenance bucket for a compound's mg.
type Source string

const (
	SourceLab    Source = "lab"
	SourceHigh   Source = "high"
	SourceMedium Source = "medium"
	SourceLow    Source = "low"
)

// Decarb: acid → neutral, by molecular-weight ratio. Applied to flower
// inputs only. Lot COAs and lab COAs are taken as reported -- a finished oil
// that still lists THCA really does contain THCA.
var decarb = map[string]struct {
	to     string
	factor float64
}{
	"thca": {"d9thc", 0.877},
	"cbda": {"cbd", 0.877},
	"cbga": {"cbg", 0.878},
}

const (
	groupCannabinoid = "cannabinoid"
	groupTerpene     = "terpene"
	groupOther       = "other"
)

var (
	ErrNotFound = errors.New("not found")
	ErrCycle    = errors.New("batch inputs form a cycle")
)

// Compound is one line of a profile.
type Compound struct {
	Group   string  `json:"group"` // cannabinoid | terpene | other
	Key     string  `json:"key"`
	Name    string  `json:"name"`
	MgPerG  float64 `json:"mgPerG"`
	MgPerMl float64 `json:"mgPerMl"`
	// Shares maps each source to its fraction (0-1) of this compound's mg.
	// Sums to 1. For a compound present only as "not detected" (0 mg), the
	// split falls back to grams of the inputs that reported it.
	Shares map[Source]float64 `json:"shares"`
}

// Profile is a batch's calculated (or lab-tested) profile.
type Profile struct {
	BatchID       string  `json:"batchId"`
	Code          string  `json:"code"`
	DensityGPerMl float64 `json:"densityGPerMl"`
	// FromLabCOA is true when the batch's own lab COA supplied the numbers.
	// Calculated then holds what the math would have said, for comparison.
	FromLabCOA bool       `json:"fromLabCoa"`
	Compounds  []Compound `json:"compounds"`
	Calculated []Compound `json:"calculated,omitempty"`
	// TotalCBD = CBD + 0.877*CBDA; TotalTHC = Δ9-THC + 0.877*THCA (the
	// standard "total" convention). Δ8-THC is reported on its own line.
	TotalCBD Compound `json:"totalCbd"`
	TotalTHC Compound `json:"totalThc"`
}

// perGram is a batch's effective profile in internal form, keyed by
// group:key, used when the batch is an input to another batch.
type perGram map[string]*entry

type entry struct {
	group, key, name string
	mgPerG           float64
	// mg and fallbackGrams are both split by source; mg is the real split,
	// fallbackGrams is used only when total mg is 0.
	shares        map[Source]float64
	fallbackShare map[Source]float64
}

type accumulator map[string]*accEntry

type accEntry struct {
	group, key, name string
	mg               map[Source]float64
	grams            map[Source]float64
}

func (a accumulator) add(group, key, name string, mg, grams float64, src Source) {
	id := group + ":" + key
	e, ok := a[id]
	if !ok {
		e = &accEntry{group: group, key: key, name: name, mg: map[Source]float64{}, grams: map[Source]float64{}}
		a[id] = e
	}
	e.mg[src] += mg
	e.grams[src] += grams
}

// Calculator resolves batch profiles against a fixed set of lots and
// batches. Results are memoized; build a new Calculator after any write.
type Calculator struct {
	lots     map[string]model.Lot
	batches  map[string]model.Batch
	memo     map[string]perGram
	visiting map[string]bool
}

func New(lots []model.Lot, batches []model.Batch) *Calculator {
	c := &Calculator{
		lots:     make(map[string]model.Lot, len(lots)),
		batches:  make(map[string]model.Batch, len(batches)),
		memo:     map[string]perGram{},
		visiting: map[string]bool{},
	}
	for _, l := range lots {
		c.lots[l.LotID] = l
	}
	for _, b := range batches {
		c.batches[b.BatchID] = b
	}
	return c
}

// Batch returns the profile for one batch.
func (c *Calculator) Batch(batchID string) (Profile, error) {
	b, ok := c.batches[batchID]
	if !ok {
		return Profile{}, fmt.Errorf("batch %s: %w", batchID, ErrNotFound)
	}
	calc, err := c.calculated(b)
	if err != nil {
		return Profile{}, err
	}
	out := Profile{BatchID: b.BatchID, Code: b.Code, DensityGPerMl: b.DensityGPerMl}
	if b.LabCOA != nil {
		out.FromLabCOA = true
		out.Compounds = render(fromLabProfile(b.LabCOA.Profile), b.DensityGPerMl)
		out.Calculated = render(calc, b.DensityGPerMl)
	} else {
		out.Compounds = render(calc, b.DensityGPerMl)
	}
	out.TotalCBD = total(out.Compounds, "cbd", "cbda", "Total CBD", b.DensityGPerMl)
	out.TotalTHC = total(out.Compounds, "d9thc", "thca", "Total THC", b.DensityGPerMl)
	return out, nil
}

// effective is what a batch contributes when used as an input: its lab COA
// if it has one, otherwise its calculated profile.
func (c *Calculator) effective(batchID string) (perGram, error) {
	if pg, ok := c.memo[batchID]; ok {
		return pg, nil
	}
	b, ok := c.batches[batchID]
	if !ok {
		return nil, fmt.Errorf("batch %s: %w", batchID, ErrNotFound)
	}
	var pg perGram
	if b.LabCOA != nil {
		pg = fromLabProfile(b.LabCOA.Profile)
	} else {
		var err error
		if pg, err = c.calculated(b); err != nil {
			return nil, err
		}
	}
	c.memo[batchID] = pg
	return pg, nil
}

func (c *Calculator) calculated(b model.Batch) (perGram, error) {
	if c.visiting[b.BatchID] {
		return nil, fmt.Errorf("%w at batch %s (%s)", ErrCycle, b.Code, b.BatchID)
	}
	c.visiting[b.BatchID] = true
	defer delete(c.visiting, b.BatchID)

	if b.TotalGrams <= 0 {
		return nil, fmt.Errorf("batch %s: totalGrams must be greater than 0", b.Code)
	}

	acc := accumulator{}
	for i, in := range b.Inputs {
		switch in.Kind {
		case model.InputLot:
			lot, ok := c.lots[in.LotID]
			if !ok {
				return nil, fmt.Errorf("batch %s input %d: lot %s: %w", b.Code, i, in.LotID, ErrNotFound)
			}
			addProfile(acc, lot.Profile, in.Grams, 1, SourceLab, false)

		case model.InputBatch:
			src, err := c.effective(in.BatchID)
			if err != nil {
				return nil, fmt.Errorf("batch %s input %d: %w", b.Code, i, err)
			}
			for _, e := range src {
				mg := in.Grams * e.mgPerG
				for s, share := range e.shares {
					acc.add(e.group, e.key, e.name, mg*share, in.Grams*e.fallbackShare[s], s)
				}
				// A 0-mg compound has no mg shares; carry its fallback split so
				// "not detected" still knows where it came from.
				if mg == 0 {
					for s, share := range e.fallbackShare {
						if _, seen := e.shares[s]; !seen {
							acc.add(e.group, e.key, e.name, 0, in.Grams*share, s)
						}
					}
				}
			}

		case model.InputFlower:
			if in.Profile == nil {
				return nil, fmt.Errorf("batch %s input %d: flower has no profile", b.Code, i)
			}
			addProfile(acc, *in.Profile, in.Grams, in.Efficiency, Source(in.Confidence), true)

		case model.InputAdditive:
			// Additives (terps etc.) are taken as their declared composition:
			// a known substance, counted as lab-backed. No profile = carries
			// no tracked compounds.
			if in.Profile != nil {
				addProfile(acc, *in.Profile, in.Grams, 1, SourceLab, false)
			}

		default:
			return nil, fmt.Errorf("batch %s input %d: unknown kind %q", b.Code, i, in.Kind)
		}
	}

	pg := perGram{}
	for id, e := range acc {
		var mg, grams float64
		for _, v := range e.mg {
			mg += v
		}
		for _, v := range e.grams {
			grams += v
		}
		out := &entry{
			group: e.group, key: e.key, name: e.name,
			mgPerG:        mg / b.TotalGrams,
			shares:        map[Source]float64{},
			fallbackShare: map[Source]float64{},
		}
		if mg > 0 {
			for s, v := range e.mg {
				if v > 0 {
					out.shares[s] = v / mg
				}
			}
		}
		if grams > 0 {
			for s, v := range e.grams {
				if v > 0 {
					out.fallbackShare[s] = v / grams
				}
			}
		}
		pg[id] = out
	}
	return pg, nil
}

// addProfile adds grams × mg/g × factor for every compound in p. For flower,
// acids are decarbed into their neutral form first.
func addProfile(acc accumulator, p model.Profile, grams, factor float64, src Source, applyDecarb bool) {
	for k, v := range p.Cannabinoids {
		key, mgPerG := k, v
		if applyDecarb {
			if d, ok := decarb[k]; ok {
				key, mgPerG = d.to, v*d.factor
			}
		}
		acc.add(groupCannabinoid, key, "", grams*mgPerG*factor, grams, src)
	}
	for k, v := range p.Terpenes {
		acc.add(groupTerpene, k, "", grams*v*factor, grams, src)
	}
	for _, o := range p.Other {
		acc.add(groupOther, strings.ToLower(strings.TrimSpace(o.Name)), o.Name, grams*o.MgPerG*factor, grams, src)
	}
}

func fromLabProfile(p model.Profile) perGram {
	pg := perGram{}
	put := func(group, key, name string, v float64) {
		e := &entry{group: group, key: key, name: name, mgPerG: v,
			shares: map[Source]float64{}, fallbackShare: map[Source]float64{SourceLab: 1}}
		if v > 0 {
			e.shares[SourceLab] = 1
		}
		pg[group+":"+key] = e
	}
	for k, v := range p.Cannabinoids {
		put(groupCannabinoid, k, "", v)
	}
	for k, v := range p.Terpenes {
		put(groupTerpene, k, "", v)
	}
	for _, o := range p.Other {
		put(groupOther, strings.ToLower(strings.TrimSpace(o.Name)), o.Name, o.MgPerG)
	}
	return pg
}

var displayOrder = func() map[string]int {
	m := map[string]int{}
	for i, d := range model.KnownCannabinoids {
		m[groupCannabinoid+":"+d.Key] = i
	}
	for i, d := range model.KnownTerpenes {
		m[groupTerpene+":"+d.Key] = 1000 + i
	}
	return m
}()

var displayName = func() map[string]string {
	m := map[string]string{}
	for _, d := range model.KnownCannabinoids {
		m[groupCannabinoid+":"+d.Key] = d.Name
	}
	for _, d := range model.KnownTerpenes {
		m[groupTerpene+":"+d.Key] = d.Name
	}
	return m
}()

// render turns a perGram into display order: known cannabinoids (CBD
// first), known terpenes, then other compounds alphabetically.
func render(pg perGram, density float64) []Compound {
	out := make([]Compound, 0, len(pg))
	for id, e := range pg {
		name := e.name
		if n, ok := displayName[id]; ok {
			name = n
		}
		shares := e.shares
		if e.mgPerG == 0 {
			shares = e.fallbackShare
		}
		out = append(out, Compound{
			Group: e.group, Key: e.key, Name: name,
			MgPerG: e.mgPerG, MgPerMl: e.mgPerG * density,
			Shares: copyShares(shares),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		oi, iKnown := displayOrder[out[i].Group+":"+out[i].Key]
		oj, jKnown := displayOrder[out[j].Group+":"+out[j].Key]
		switch {
		case iKnown && jKnown:
			return oi < oj
		case iKnown != jKnown:
			return iKnown
		default:
			return out[i].Key < out[j].Key
		}
	})
	return out
}

// total combines a neutral cannabinoid and its acid (×0.877) into one line,
// with shares weighted by each part's mg.
func total(cs []Compound, neutral, acid, name string, density float64) Compound {
	t := Compound{Group: groupCannabinoid, Key: "total_" + neutral, Name: name, Shares: map[Source]float64{}}
	mgBySource := map[Source]float64{}
	var anyPresent bool
	var fallback map[Source]float64
	for _, c := range cs {
		if c.Group != groupCannabinoid {
			continue
		}
		var w float64
		switch c.Key {
		case neutral:
			w = 1
		case acid:
			w = 0.877
		default:
			continue
		}
		anyPresent = true
		t.MgPerG += c.MgPerG * w
		for s, share := range c.Shares {
			mgBySource[s] += c.MgPerG * w * share
		}
		if fallback == nil {
			fallback = c.Shares
		}
	}
	if !anyPresent {
		return Compound{Group: groupCannabinoid, Key: t.Key, Name: name, Shares: map[Source]float64{}}
	}
	t.MgPerMl = t.MgPerG * density
	if t.MgPerG > 0 {
		for s, v := range mgBySource {
			if v > 0 {
				t.Shares[s] = v / t.MgPerG
			}
		}
	} else {
		t.Shares = copyShares(fallback)
	}
	return t
}

func copyShares(m map[Source]float64) map[Source]float64 {
	if m == nil {
		return map[Source]float64{}
	}
	return maps.Clone(m)
}

// Dependents returns the IDs of every batch that uses batchID as an input,
// directly or through other batches. These are the blends whose numbers
// change when batchID is edited.
func (c *Calculator) Dependents(batchID string) []string {
	users := map[string][]string{} // input batch → batches that use it
	for id, b := range c.batches {
		for _, in := range b.Inputs {
			if in.Kind == model.InputBatch {
				users[in.BatchID] = append(users[in.BatchID], id)
			}
		}
	}
	seen := map[string]bool{batchID: true}
	queue := []string{batchID}
	var out []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, u := range users[cur] {
			if !seen[u] {
				seen[u] = true
				out = append(out, u)
				queue = append(queue, u)
			}
		}
	}
	sort.Strings(out)
	return out
}

// LotDependents returns the IDs of every batch that uses lotID, directly or
// through other batches.
func (c *Calculator) LotDependents(lotID string) []string {
	direct := map[string]bool{}
	for id, b := range c.batches {
		for _, in := range b.Inputs {
			if in.Kind == model.InputLot && in.LotID == lotID {
				direct[id] = true
			}
		}
	}
	all := map[string]bool{}
	for id := range direct {
		all[id] = true
		for _, d := range c.Dependents(id) {
			all[d] = true
		}
	}
	out := make([]string, 0, len(all))
	for id := range all {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// DoseResult is how much oil delivers a target amount of one compound.
type DoseResult struct {
	Grams float64 `json:"grams"`
	Ml    float64 `json:"ml"`
}

// Dose answers "how much oil for targetMg of a compound at mgPerG". It saves
// nothing.
func Dose(mgPerG, densityGPerMl, targetMg float64) (DoseResult, error) {
	if targetMg <= 0 {
		return DoseResult{}, errors.New("target mg must be greater than 0")
	}
	if mgPerG <= 0 {
		return DoseResult{}, errors.New("batch has none of that compound")
	}
	if densityGPerMl <= 0 {
		return DoseResult{}, errors.New("density must be greater than 0")
	}
	g := targetMg / mgPerG
	return DoseResult{Grams: g, Ml: g / densityGPerMl}, nil
}
