package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Dose: what Chris takes each day. Two independent layers, because the
// dose and the bottle change for different reasons:
//   - DoseVersion (the plan): mg per time of day, each line naming a shelf
//     item. Versioned: a dose change ends the current version and starts a
//     new one; a version with no start date (or a future one) is the draft.
//   - ShelfItem: which bottle currently fills a named role ("Distillate oil"
//     → batch DIS-260712). Repointing it changes the mL, never the mg.
// mL/pieces, day totals and monthly use are derived on read, never stored.

// DoseCompound is what a plan line's mg is measured in. Both are totals:
// cbd = CBD + 0.877×CBDA, thc = Δ9-THC + 0.877×THCA (the ROBP convention).
type DoseCompound string

const (
	DoseCBD DoseCompound = "cbd"
	DoseTHC DoseCompound = "thc"
)

func (c DoseCompound) Valid() bool { return c == DoseCBD || c == DoseTHC }

type ShelfSourceKind string

const (
	ShelfFromBatch   ShelfSourceKind = "batch"
	ShelfFromProduct ShelfSourceKind = "product"
)

// ProductUnit is what a product's strength is per.
type ProductUnit string

const (
	ProductPerMl    ProductUnit = "mL"
	ProductPerPiece ProductUnit = "piece"
)

// ShelfSource is what fills a shelf item: one of Chris's batches (strength
// from its ROBP) or something bought premade (strength from its label).
type ShelfSource struct {
	Kind    ShelfSourceKind `dynamodbav:"kind"              json:"kind"              jsonschema:"batch (one of Chris's oil batches) | product (premade oil, gummy, etc.)"`
	BatchID string          `dynamodbav:"batchId,omitempty" json:"batchId,omitempty" jsonschema:"batch sources: batchId or batch code from list_batches"`

	ProductName string      `dynamodbav:"productName,omitempty" json:"productName,omitempty" jsonschema:"product sources: brand and product, e.g. Sunset Lake Vibe gummy"`
	Unit        ProductUnit `dynamodbav:"unit,omitempty"        json:"unit,omitempty"        jsonschema:"product sources: mL (an oil, strength per mL) or piece (gummy, capsule; strength per piece)"`
	UnitName    string      `dynamodbav:"unitName,omitempty"    json:"unitName,omitempty"    jsonschema:"product sources with unit piece: what one piece is called, e.g. gummy"`
	// MgPerUnit is per mL or per piece. Keys are the known cannabinoid and
	// terpene keys, so totals use the same CBD + 0.877×CBDA convention as
	// batches and terpenes add up alongside.
	MgPerUnit map[string]float64 `dynamodbav:"mgPerUnit,omitempty" json:"mgPerUnit,omitempty" jsonschema:"product sources: mg per unit by cannabinoid key (cbd cbda d9thc thca d8thc cbg cbga cbn cbc cbdv thcv) or terpene key (myrcene limonene beta_caryophyllene linalool alpha_pinene beta_pinene humulene terpinolene ocimene bisabolol nerolidol caryophyllene_oxide eucalyptol geraniol camphene terpineol), e.g. {cbd: 30, myrcene: 0.4}"`
	// A product can have several COAs, e.g. one for cannabinoids and one for
	// terpenes.
	COAs []ShelfCOA `dynamodbav:"coas,omitempty" json:"coas,omitempty" jsonschema:"product sources, optional: the product's COAs, each labeled with what it covers; the app keeps a copy of each"`

	// Single COA from before 2026-11-16. Read only: UpgradeCOAs folds it into COAs.
	LegacyCOAURL   string `dynamodbav:"coaUrl,omitempty"   json:"-"`
	LegacyCOAS3Key string `dynamodbav:"coaS3Key,omitempty" json:"-"`
}

type ShelfCOA struct {
	URL   string `dynamodbav:"url"             json:"url"             jsonschema:"link to the COA"`
	Label string `dynamodbav:"label,omitempty" json:"label,omitempty" jsonschema:"what it covers, e.g. Cannabinoids or Terpenes"`
	S3Key string `dynamodbav:"s3Key,omitempty" json:"s3Key,omitempty" jsonschema:"ignored on input; set by archiving"`
}

// UpgradeCOAs moves a legacy single COA into COAs.
func (s *ShelfSource) UpgradeCOAs() {
	if s.LegacyCOAURL == "" && s.LegacyCOAS3Key == "" {
		return
	}
	s.COAs = append([]ShelfCOA{{URL: s.LegacyCOAURL, S3Key: s.LegacyCOAS3Key}}, s.COAs...)
	s.LegacyCOAURL, s.LegacyCOAS3Key = "", ""
}

// UpgradeCOAs upgrades the current source and every history entry.
func (s *ShelfItem) UpgradeCOAs() {
	s.Source.UpgradeCOAs()
	for i := range s.History {
		s.History[i].Source.UpgradeCOAs()
	}
}

// ShelfHistoryEntry is a source a shelf item used to point at, [From, To).
type ShelfHistoryEntry struct {
	Source ShelfSource `dynamodbav:"source" json:"source"`
	From   string      `dynamodbav:"from"   json:"from"` // YYYY-MM-DD
	To     string      `dynamodbav:"to"     json:"to"`
}

type ShelfItem struct {
	ShelfItemID string `dynamodbav:"shelfItemId" json:"shelfItemId"`
	UserID      string `dynamodbav:"userId"      json:"userId"`
	// Name is the role, unique per user (case-insensitive): "Distillate oil",
	// "Night". Plan lines point at the item, so renaming is safe.
	Name      string              `dynamodbav:"name"              json:"name"`
	Source    ShelfSource         `dynamodbav:"source"            json:"source"`
	Since     string              `dynamodbav:"since"             json:"since"` // YYYY-MM-DD the current source started
	History   []ShelfHistoryEntry `dynamodbav:"history,omitempty" json:"history,omitempty"`
	Notes     string              `dynamodbav:"notes,omitempty"   json:"notes,omitempty"`
	CreatedAt string              `dynamodbav:"createdAt"         json:"createdAt"`
	UpdatedAt string              `dynamodbav:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}

type DoseLine struct {
	ShelfItemID string       `dynamodbav:"shelfItemId"    json:"shelfItemId"`
	Mg          float64      `dynamodbav:"mg"             json:"mg"`
	Compound    DoseCompound `dynamodbav:"compound"       json:"compound"`
	Note        string       `dynamodbav:"note,omitempty" json:"note,omitempty"`
}

// DoseTime is a time of day: Morning, Midday, Night.
type DoseTime struct {
	Name  string     `dynamodbav:"name"  json:"name"`
	Lines []DoseLine `dynamodbav:"lines" json:"lines"`
}

// DoseVersion is one version of the plan. Active over [StartDate, EndDate).
// No StartDate, or one in the future, means it hasn't started: the draft.
type DoseVersion struct {
	VersionID string     `dynamodbav:"versionId"           json:"versionId"`
	UserID    string     `dynamodbav:"userId"              json:"userId"`
	Name      string     `dynamodbav:"name,omitempty"      json:"name,omitempty"`
	StartDate string     `dynamodbav:"startDate,omitempty" json:"startDate,omitempty"`
	EndDate   string     `dynamodbav:"endDate,omitempty"   json:"endDate,omitempty"`
	Times     []DoseTime `dynamodbav:"times"               json:"times"`
	Notes     string     `dynamodbav:"notes,omitempty"     json:"notes,omitempty"`
	CreatedAt string     `dynamodbav:"createdAt"           json:"createdAt"`
	UpdatedAt string     `dynamodbav:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}

type DoseStatus string

const (
	DoseCurrent DoseStatus = "current"
	DoseDraft   DoseStatus = "draft"
	DosePast    DoseStatus = "past"
)

// Status of a version on a given day (YYYY-MM-DD; ISO dates compare as strings).
func (v DoseVersion) Status(today string) DoseStatus {
	switch {
	case v.StartDate == "" || v.StartDate > today:
		return DoseDraft
	case v.EndDate != "" && v.EndDate <= today:
		return DosePast
	default:
		return DoseCurrent
	}
}

func ValidDate(s string) error {
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return fmt.Errorf("date %q must be YYYY-MM-DD", s)
	}
	return nil
}

// Validate checks the version's own fields. Whether each line's shelf item
// exists is checked by the dose service.
func (v DoseVersion) Validate() error {
	var errs []error
	if v.StartDate != "" {
		if err := ValidDate(v.StartDate); err != nil {
			errs = append(errs, fmt.Errorf("startDate: %w", err))
		}
	}
	if v.EndDate != "" {
		if err := ValidDate(v.EndDate); err != nil {
			errs = append(errs, fmt.Errorf("endDate: %w", err))
		}
		if v.StartDate == "" || v.EndDate <= v.StartDate {
			errs = append(errs, errors.New("endDate must be after startDate"))
		}
	}
	if len(v.Times) == 0 {
		errs = append(errs, errors.New("at least one time of day is required"))
	}
	seen := map[string]bool{}
	for i, t := range v.Times {
		name := strings.TrimSpace(t.Name)
		if name == "" {
			errs = append(errs, fmt.Errorf("time %d: name is required", i))
		}
		if seen[strings.ToLower(name)] {
			errs = append(errs, fmt.Errorf("time %q appears twice", name))
		}
		seen[strings.ToLower(name)] = true
		if len(t.Lines) == 0 {
			errs = append(errs, fmt.Errorf("time %q: at least one line is required", name))
		}
		for j, l := range t.Lines {
			if l.ShelfItemID == "" {
				errs = append(errs, fmt.Errorf("time %q line %d: shelf item is required", name, j))
			}
			if l.Mg <= 0 {
				errs = append(errs, fmt.Errorf("time %q line %d: mg must be greater than 0", name, j))
			}
			if !l.Compound.Valid() {
				errs = append(errs, fmt.Errorf("time %q line %d: compound %q must be cbd or thc", name, j, l.Compound))
			}
		}
	}
	return errors.Join(errs...)
}

func (s ShelfSource) Validate() error {
	var errs []error
	switch s.Kind {
	case ShelfFromBatch:
		if s.BatchID == "" {
			errs = append(errs, errors.New("batchId is required"))
		}
		if s.ProductName != "" || s.Unit != "" || len(s.MgPerUnit) > 0 || len(s.COAs) > 0 {
			errs = append(errs, errors.New("product fields don't apply to a batch source (its strength comes from the ROBP)"))
		}
	case ShelfFromProduct:
		if s.BatchID != "" {
			errs = append(errs, errors.New("batchId doesn't apply to a product source"))
		}
		if strings.TrimSpace(s.ProductName) == "" {
			errs = append(errs, errors.New("productName is required"))
		}
		switch s.Unit {
		case ProductPerMl:
		case ProductPerPiece:
			if strings.TrimSpace(s.UnitName) == "" {
				errs = append(errs, errors.New("unitName is required for piece products, e.g. gummy"))
			}
		default:
			errs = append(errs, fmt.Errorf("unit %q must be mL or piece", s.Unit))
		}
		if len(s.MgPerUnit) == 0 {
			errs = append(errs, errors.New("mgPerUnit is required (the label strength)"))
		}
		for k, v := range s.MgPerUnit {
			if !knownCannabinoidKeys[k] && !knownTerpeneKeys[k] {
				errs = append(errs, fmt.Errorf("unknown cannabinoid or terpene %q in mgPerUnit", k))
			}
			if v < 0 {
				errs = append(errs, fmt.Errorf("mgPerUnit %q must not be negative", k))
			}
		}
		for i, c := range s.COAs {
			if strings.TrimSpace(c.URL) == "" {
				errs = append(errs, fmt.Errorf("coas[%d]: url is required", i))
			}
		}
	default:
		errs = append(errs, fmt.Errorf("source kind %q must be batch or product", s.Kind))
	}
	return errors.Join(errs...)
}

func (s ShelfItem) Validate() error {
	var errs []error
	if strings.TrimSpace(s.Name) == "" {
		errs = append(errs, errors.New("name is required"))
	}
	if err := ValidDate(s.Since); err != nil {
		errs = append(errs, fmt.Errorf("since: %w", err))
	}
	if err := s.Source.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("source: %w", err))
	}
	return errors.Join(errs...)
}
