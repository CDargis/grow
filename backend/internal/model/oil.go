package model

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Oils: supplier lots (with COAs) go in, batches come out. All potency values
// are mg/g -- the single stored unit. mg/mL is always derived from mg/g and a
// batch's carrier density, never stored. Batch profiles are calculated on read
// (see internal/oilcalc), never stored, so editing a batch flows into every
// blend made from it.

type BatchType string

const (
	BatchDIS BatchType = "DIS"
	BatchISO BatchType = "ISO"
	BatchFS  BatchType = "FS"
	BatchTHC BatchType = "THC"
	BatchBL  BatchType = "BL"
)

func (t BatchType) Valid() bool {
	switch t {
	case BatchDIS, BatchISO, BatchFS, BatchTHC, BatchBL:
		return true
	}
	return false
}

// Confidence is Chris's own rating of a flower-oil method. Lab-backed numbers
// never carry one.
type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

func (c Confidence) Valid() bool {
	switch c {
	case ConfidenceLow, ConfidenceMedium, ConfidenceHigh:
		return true
	}
	return false
}

type InputKind string

const (
	InputLot      InputKind = "lot"
	InputBatch    InputKind = "batch"
	InputFlower   InputKind = "flower"
	InputAdditive InputKind = "additive"
)

const (
	// DefaultCarrierDensity is g/mL for MCT. MCT-type carriers sit around
	// 0.93-0.96, so the default is within ~2% (LouAna liquid coconut ≈ 0.94).
	DefaultCarrierDensity = 0.95

	// NoDateCode stands in for YYMMDD in a batch code when the date is unknown.
	NoDateCode = "000000"
)

// CompoundDef is one entry in a fixed known list. Key is the storage key;
// Name is the display name.
type CompoundDef struct {
	Key  string
	Name string
}

// KnownCannabinoids is ordered for display: CBD first (the main use case),
// THC alongside, minors after.
var KnownCannabinoids = []CompoundDef{
	{"cbd", "CBD"},
	{"cbda", "CBDA"},
	{"d9thc", "Δ9-THC"},
	{"thca", "THCA"},
	{"d8thc", "Δ8-THC"},
	{"cbg", "CBG"},
	{"cbga", "CBGA"},
	{"cbn", "CBN"},
	{"cbc", "CBC"},
	{"cbdv", "CBDV"},
	{"thcv", "THCV"},
}

var KnownTerpenes = []CompoundDef{
	{"myrcene", "β-Myrcene"},
	{"limonene", "Limonene"},
	{"beta_caryophyllene", "β-Caryophyllene"},
	{"linalool", "Linalool"},
	{"alpha_pinene", "α-Pinene"},
	{"beta_pinene", "β-Pinene"},
	{"humulene", "α-Humulene"},
	{"terpinolene", "Terpinolene"},
	{"ocimene", "Ocimene"},
	{"bisabolol", "α-Bisabolol"},
	{"nerolidol", "Nerolidol"},
	{"caryophyllene_oxide", "Caryophyllene Oxide"},
	{"eucalyptol", "Eucalyptol"},
	{"geraniol", "Geraniol"},
	{"camphene", "Camphene"},
	{"terpineol", "Terpineol"},
}

// NamedAmount holds a compound the lab reported that isn't on a known list,
// so nothing on a COA gets thrown away.
type NamedAmount struct {
	Name   string  `dynamodbav:"name"   json:"name"   jsonschema:"compound name as the lab wrote it"`
	MgPerG float64 `dynamodbav:"mgPerG" json:"mgPerG" jsonschema:"mg/g (1% = 10 mg/g); 0 = not detected"`
}

// Profile is the standard storage format for anything a COA (or a label)
// reports. All values mg/g; 1% = 10 mg/g. A present key with value 0 means
// "tested, not detected" -- distinct from an absent key ("not reported").
type Profile struct {
	Cannabinoids map[string]float64 `dynamodbav:"cannabinoids,omitempty" json:"cannabinoids,omitempty" jsonschema:"mg/g by key (1% = 10 mg/g; 0 = not detected; omit if not reported). Keys: cbd cbda d9thc thca d8thc cbg cbga cbn cbc cbdv thcv"`
	Terpenes     map[string]float64 `dynamodbav:"terpenes,omitempty"     json:"terpenes,omitempty"     jsonschema:"mg/g by key. Keys: myrcene limonene beta_caryophyllene linalool alpha_pinene beta_pinene humulene terpinolene ocimene bisabolol nerolidol caryophyllene_oxide eucalyptol geraniol camphene terpineol"`
	Other        []NamedAmount      `dynamodbav:"other,omitempty"        json:"other,omitempty"        jsonschema:"anything the COA reports that has no key above, so nothing is lost"`
}

type Lot struct {
	LotID             string  `dynamodbav:"lotId"             json:"lotId"`
	UserID            string  `dynamodbav:"userId"            json:"userId"`
	Supplier          string  `dynamodbav:"supplier"          json:"supplier"`
	Product           string  `dynamodbav:"product"           json:"product"`
	SupplierLotNumber string  `dynamodbav:"supplierLotNumber" json:"supplierLotNumber"`
	GramsPurchased    float64 `dynamodbav:"gramsPurchased"    json:"gramsPurchased,omitempty"`
	COAURL            string  `dynamodbav:"coaUrl"            json:"coaUrl,omitempty"`
	// COAS3Key is set once the COA at COAURL has been copied to S3. Absent
	// means "not archived" (no URL, or the fetch failed).
	COAS3Key  string  `dynamodbav:"coaS3Key"  json:"coaS3Key,omitempty"`
	Profile   Profile `dynamodbav:"profile"   json:"profile"`
	Notes     string  `dynamodbav:"notes"     json:"notes,omitempty"`
	CreatedAt string  `dynamodbav:"createdAt" json:"createdAt"`
	UpdatedAt string  `dynamodbav:"updatedAt" json:"updatedAt,omitempty"`
}

// BatchInput is one ingredient. Which fields apply depends on Kind:
//   - lot:      LotID
//   - batch:    BatchID
//   - flower:   Strain, Profile (label/COA numbers, acids as reported),
//     Efficiency, Method, Confidence; PlantID optional. Grams is
//     grams of flower, not oil.
//   - additive: Name, optional Profile (e.g. pure limonene), Note
type BatchInput struct {
	Kind  InputKind `dynamodbav:"kind"  json:"kind"  jsonschema:"lot | batch | flower | additive"`
	Grams float64   `dynamodbav:"grams" json:"grams" jsonschema:"grams used; for flower, grams of flower (not oil); convert drops etc. to grams before sending"`

	LotID   string `dynamodbav:"lotId,omitempty"   json:"lotId,omitempty"   jsonschema:"lot inputs: the lot's lotId from list_lots"`
	BatchID string `dynamodbav:"batchId,omitempty" json:"batchId,omitempty" jsonschema:"batch inputs: the source batch's batchId from list_batches"`

	Strain  string   `dynamodbav:"strain,omitempty"  json:"strain,omitempty"  jsonschema:"flower inputs: strain name"`
	PlantID string   `dynamodbav:"plantId,omitempty" json:"plantId,omitempty" jsonschema:"flower inputs, optional: the grow plant this flower came from (list_plants)"`
	Profile *Profile `dynamodbav:"profile,omitempty" json:"profile,omitempty" jsonschema:"flower inputs: label or COA numbers as reported (THCA/CBDA as acids; decarb is applied by the app). additive inputs, optional: its composition, e.g. limonene 1000 for pure limonene"`
	// Efficiency (0-1] is how much of the decarbed cannabinoid content made it
	// into the oil. Settled per batch by Chris and Claude, not derived.
	Efficiency float64    `dynamodbav:"efficiency,omitempty" json:"efficiency,omitempty" jsonschema:"flower inputs: (0, 1], share of decarbed cannabinoids that ended up in the oil; agree it with Chris, do not assume"`
	Method     string     `dynamodbav:"method,omitempty"     json:"method,omitempty"     jsonschema:"flower inputs: how the oil was made, free text"`
	Confidence Confidence `dynamodbav:"confidence,omitempty" json:"confidence,omitempty" jsonschema:"flower inputs: low | medium | high, Chris's trust in this method; ask if unsure"`
	// COAURL is the flower's own COA (often a JPG for flower bought online).
	// It documents the flower, not the oil, so the flower's numbers still
	// count as estimates. COAS3Key is set once the link has been archived.
	COAURL   string `dynamodbav:"coaUrl,omitempty"   json:"coaUrl,omitempty"   jsonschema:"flower inputs, optional: direct link to the flower's COA file (PDF or image, not a share page); the app downloads and keeps a copy"`
	COAS3Key string `dynamodbav:"coaS3Key,omitempty" json:"coaS3Key,omitempty" jsonschema:"ignored on input; set by archiving"`

	Name string `dynamodbav:"name,omitempty" json:"name,omitempty" jsonschema:"additive inputs: what was added, e.g. limonene"`
	Note string `dynamodbav:"note,omitempty" json:"note,omitempty" jsonschema:"free text, e.g. the original wording like 3 drops"`
}

type Carrier struct {
	Name  string  `dynamodbav:"name"  json:"name"  jsonschema:"carrier oil, e.g. LouAna liquid coconut oil"`
	Grams float64 `dynamodbav:"grams" json:"grams" jsonschema:"grams of carrier oil"`
}

// LabCOA is an optional lab test of the finished batch. When present its
// profile replaces the calculated one (and counts as 100% lab downstream).
type LabCOA struct {
	URL     string  `dynamodbav:"url"             json:"url"             jsonschema:"link to the batch's own lab COA"`
	S3Key   string  `dynamodbav:"s3Key,omitempty" json:"s3Key,omitempty" jsonschema:"ignored on input; set by archiving"`
	Profile Profile `dynamodbav:"profile"         json:"profile"         jsonschema:"numbers from the batch's lab COA"`
}

type Batch struct {
	BatchID string `dynamodbav:"batchId" json:"batchId"`
	UserID  string `dynamodbav:"userId"  json:"userId"`
	// Code is the human batch ID, built by BuildBatchCode from the parts
	// below and unique per user. It is a label, not the key, so editing the
	// parts doesn't break blends that reference this batch.
	Code               string    `dynamodbav:"code"                         json:"code"`
	Type               BatchType `dynamodbav:"type"                         json:"type"`
	SubType            string    `dynamodbav:"subType,omitempty"            json:"subType,omitempty"`
	Discriminator      string    `dynamodbav:"discriminator,omitempty"      json:"discriminator,omitempty"`
	Date               string    `dynamodbav:"date,omitempty"               json:"date,omitempty"` // YYYY-MM-DD; empty = unknown
	BatchDiscriminator string    `dynamodbav:"batchDiscriminator,omitempty" json:"batchDiscriminator,omitempty"`

	Inputs        []BatchInput `dynamodbav:"inputs"            json:"inputs"`
	Carrier       *Carrier     `dynamodbav:"carrier,omitempty" json:"carrier,omitempty"`
	DensityGPerMl float64      `dynamodbav:"densityGPerMl"     json:"densityGPerMl"`
	TotalGrams    float64      `dynamodbav:"totalGrams"        json:"totalGrams"`
	LabCOA        *LabCOA      `dynamodbav:"labCoa,omitempty"  json:"labCoa,omitempty"`
	Notes         string       `dynamodbav:"notes,omitempty"   json:"notes,omitempty"`
	CreatedAt     string       `dynamodbav:"createdAt"         json:"createdAt"`
	UpdatedAt     string       `dynamodbav:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}

var codePartRe = regexp.MustCompile(`^[A-Za-z0-9]+$`)

// BuildBatchCode builds [type]-<subType>-<discriminator>-[YYMMDD]-<batchDiscriminator>.
// Optional parts are omitted when empty. Parts must be alphanumeric (no
// hyphens), since a hyphen inside a part would make the code ambiguous.
// Parts are uppercased. An empty date becomes NoDateCode.
func BuildBatchCode(t BatchType, subType, discriminator, date, batchDiscriminator string) (string, error) {
	if !t.Valid() {
		return "", fmt.Errorf("invalid batch type %q (want DIS, ISO, FS, THC or BL)", t)
	}
	parts := []string{string(t)}
	for _, p := range []struct{ field, value string }{
		{"subType", subType},
		{"discriminator", discriminator},
	} {
		if p.value == "" {
			continue
		}
		if !codePartRe.MatchString(p.value) {
			return "", fmt.Errorf("%s %q must be letters and digits only", p.field, p.value)
		}
		parts = append(parts, strings.ToUpper(p.value))
	}

	if date == "" {
		parts = append(parts, NoDateCode)
	} else {
		d, err := time.Parse("2006-01-02", date)
		if err != nil {
			return "", fmt.Errorf("date %q must be YYYY-MM-DD", date)
		}
		parts = append(parts, d.Format("060102"))
	}

	if batchDiscriminator != "" {
		if !codePartRe.MatchString(batchDiscriminator) {
			return "", fmt.Errorf("batchDiscriminator %q must be letters and digits only", batchDiscriminator)
		}
		parts = append(parts, strings.ToUpper(batchDiscriminator))
	}
	return strings.Join(parts, "-"), nil
}

func knownKeys(defs []CompoundDef) map[string]bool {
	m := make(map[string]bool, len(defs))
	for _, d := range defs {
		m[d.Key] = true
	}
	return m
}

var (
	knownCannabinoidKeys = knownKeys(KnownCannabinoids)
	knownTerpeneKeys     = knownKeys(KnownTerpenes)
)

// Validate checks keys against the known lists and that values are sane.
// Off-list compounds belong in Other, so an unknown key is an error rather
// than silently stored under a name nothing will display.
func (p Profile) Validate() error {
	var errs []error
	for k, v := range p.Cannabinoids {
		if !knownCannabinoidKeys[k] {
			errs = append(errs, fmt.Errorf("unknown cannabinoid %q (put it in other)", k))
		}
		if v < 0 || v > 1000 {
			errs = append(errs, fmt.Errorf("cannabinoid %q = %v mg/g, must be 0-1000", k, v))
		}
	}
	for k, v := range p.Terpenes {
		if !knownTerpeneKeys[k] {
			errs = append(errs, fmt.Errorf("unknown terpene %q (put it in other)", k))
		}
		if v < 0 || v > 1000 {
			errs = append(errs, fmt.Errorf("terpene %q = %v mg/g, must be 0-1000", k, v))
		}
	}
	for _, o := range p.Other {
		if strings.TrimSpace(o.Name) == "" {
			errs = append(errs, errors.New("other compound with empty name"))
		}
		if o.MgPerG < 0 || o.MgPerG > 1000 {
			errs = append(errs, fmt.Errorf("other %q = %v mg/g, must be 0-1000", o.Name, o.MgPerG))
		}
	}
	return errors.Join(errs...)
}

func (l Lot) Validate() error {
	var errs []error
	if strings.TrimSpace(l.Supplier) == "" {
		errs = append(errs, errors.New("supplier is required"))
	}
	if strings.TrimSpace(l.Product) == "" {
		errs = append(errs, errors.New("product is required"))
	}
	if strings.TrimSpace(l.SupplierLotNumber) == "" {
		errs = append(errs, errors.New("supplierLotNumber is required"))
	}
	if l.GramsPurchased < 0 {
		errs = append(errs, errors.New("gramsPurchased must not be negative"))
	}
	if err := l.Profile.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("profile: %w", err))
	}
	return errors.Join(errs...)
}

// Validate checks the batch's own fields. It does not resolve references to
// lots or other batches -- oilcalc does that (and catches cycles).
func (b Batch) Validate() error {
	var errs []error
	if _, err := BuildBatchCode(b.Type, b.SubType, b.Discriminator, b.Date, b.BatchDiscriminator); err != nil {
		errs = append(errs, err)
	}
	if len(b.Inputs) == 0 {
		errs = append(errs, errors.New("at least one input is required"))
	}
	for i, in := range b.Inputs {
		if err := in.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("input %d: %w", i, err))
		}
	}
	if b.TotalGrams <= 0 {
		errs = append(errs, errors.New("totalGrams must be greater than 0"))
	}
	if b.DensityGPerMl <= 0 {
		errs = append(errs, errors.New("densityGPerMl must be greater than 0"))
	}
	if b.Carrier != nil {
		if strings.TrimSpace(b.Carrier.Name) == "" {
			errs = append(errs, errors.New("carrier name is required"))
		}
		if b.Carrier.Grams <= 0 {
			errs = append(errs, errors.New("carrier grams must be greater than 0"))
		}
	}
	if b.LabCOA != nil {
		if strings.TrimSpace(b.LabCOA.URL) == "" {
			errs = append(errs, errors.New("labCoa url is required"))
		}
		if err := b.LabCOA.Profile.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("labCoa profile: %w", err))
		}
	}
	return errors.Join(errs...)
}

func (in BatchInput) Validate() error {
	var errs []error
	if in.Grams <= 0 {
		errs = append(errs, errors.New("grams must be greater than 0"))
	}
	if in.COAURL != "" && in.Kind != InputFlower {
		errs = append(errs, errors.New("coaUrl only applies to flower inputs (a lot carries its own COA)"))
	}
	switch in.Kind {
	case InputLot:
		if in.LotID == "" {
			errs = append(errs, errors.New("lotId is required"))
		}
	case InputBatch:
		if in.BatchID == "" {
			errs = append(errs, errors.New("batchId is required"))
		}
	case InputFlower:
		if strings.TrimSpace(in.Strain) == "" {
			errs = append(errs, errors.New("strain is required"))
		}
		if in.Profile == nil {
			errs = append(errs, errors.New("profile is required (label or COA numbers)"))
		} else if err := in.Profile.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("profile: %w", err))
		}
		if in.Efficiency <= 0 || in.Efficiency > 1 {
			errs = append(errs, fmt.Errorf("efficiency %v must be in (0, 1]", in.Efficiency))
		}
		if strings.TrimSpace(in.Method) == "" {
			errs = append(errs, errors.New("method is required"))
		}
		if !in.Confidence.Valid() {
			errs = append(errs, fmt.Errorf("confidence %q must be low, medium or high", in.Confidence))
		}
	case InputAdditive:
		if strings.TrimSpace(in.Name) == "" {
			errs = append(errs, errors.New("name is required"))
		}
		if in.Profile != nil {
			if err := in.Profile.Validate(); err != nil {
				errs = append(errs, fmt.Errorf("profile: %w", err))
			}
		}
	default:
		errs = append(errs, fmt.Errorf("kind %q must be lot, batch, flower or additive", in.Kind))
	}
	return errors.Join(errs...)
}
