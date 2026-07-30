package model

type ProductForm string

const (
	ProductLiquid ProductForm = "liquid"
	ProductDry    ProductForm = "dry"
)

type NPK struct {
	N float64 `dynamodbav:"n" json:"n"`
	P float64 `dynamodbav:"p" json:"p"`
	K float64 `dynamodbav:"k" json:"k"`
}

// ReferenceDose is the label's dosing instruction: Amount (Min-Max) of the
// product per PerVolume of water (liquid) or container/pot size (dry).
// Min == Max for a single labeled dose rather than a range.
type ReferenceDose struct {
	Min           float64 `dynamodbav:"min"           json:"min"`
	Max           float64 `dynamodbav:"max"           json:"max"`
	Unit          string  `dynamodbav:"unit"          json:"unit"`          // ml | oz | g | tsp | tbsp
	PerVolume     float64 `dynamodbav:"perVolume"     json:"perVolume"`
	PerVolumeUnit string  `dynamodbav:"perVolumeUnit" json:"perVolumeUnit"` // gal | l
}

type Product struct {
	ProductID     string        `dynamodbav:"productId"     json:"productId"`
	UserID        string        `dynamodbav:"userId"        json:"userId"`
	Name          string        `dynamodbav:"name"          json:"name"`
	Brand         string        `dynamodbav:"brand"         json:"brand,omitempty"`
	Form          ProductForm   `dynamodbav:"form"          json:"form"`
	NPK           NPK           `dynamodbav:"npk"           json:"npk"`
	// ElementalNPK is derived from NPK (the label's guaranteed analysis,
	// which reports P/K as oxides) at write time -- never entered directly,
	// never recomputed ad hoc elsewhere. All delivered-grams math uses this,
	// never the label form.
	ElementalNPK      NPK           `dynamodbav:"elementalNpk"      json:"elementalNpk"`
	ReferenceDose     ReferenceDose `dynamodbav:"referenceDose"     json:"referenceDose"`
	// Density is grams per one ReferenceDose.Unit -- lets delivered-grams
	// math convert a dose amount into mass. Defaulted by unit (see
	// DefaultDensity); DensityCalibrated is false until the user has
	// actually weighed the product (only meaningful for tbsp/tsp units,
	// where bulk density genuinely varies -- ml/oz/g get a trustworthy
	// default and are never flagged).
	Density           float64       `dynamodbav:"density"           json:"density,omitempty"`
	DensityCalibrated bool          `dynamodbav:"densityCalibrated" json:"densityCalibrated"`
	StockQty      float64       `dynamodbav:"stockQty"      json:"stockQty,omitempty"`
	StockUnit     string        `dynamodbav:"stockUnit"     json:"stockUnit,omitempty"`
	Notes         string        `dynamodbav:"notes"         json:"notes,omitempty"`
	CreatedAt     string        `dynamodbav:"createdAt"     json:"createdAt"`
}

type CreateProductRequest struct {
	Name          string        `json:"name"`
	Brand         string        `json:"brand,omitempty"`
	Form          ProductForm   `json:"form"`
	NPK           NPK           `json:"npk"`
	ReferenceDose ReferenceDose `json:"referenceDose"`
	// Density/DensityCalibrated are optional -- when omitted (Density == 0),
	// the store fills in the unit-appropriate default.
	Density           float64 `json:"density,omitempty"`
	DensityCalibrated bool    `json:"densityCalibrated,omitempty"`
	StockQty      float64       `json:"stockQty,omitempty"`
	StockUnit     string        `json:"stockUnit,omitempty"`
	Notes         string        `json:"notes,omitempty"`
}

// ElementalFromLabel converts a label's guaranteed analysis (P as P2O5, K as
// K2O -- oxide forms, industry-standard for fertilizer labels) into
// elemental N/P/K, which is what's physically delivered to the plant and
// what all delivered-grams math must use.
func ElementalFromLabel(label NPK) NPK {
	return NPK{N: label.N, P: label.P * 0.436, K: label.K * 0.83}
}

// DefaultDensity returns grams-per-one-ReferenceDose.Unit and whether that
// default is trustworthy without calibration. ml/oz assume ~1.0 specific
// gravity (true within a few percent for these products); g is exact by
// definition. tbsp/tsp are the only units where dry bulk density varies
// enough to need a real measurement.
func DefaultDensity(unit string) (density float64, calibrated bool) {
	switch unit {
	case "ml":
		return 1.0, true
	case "oz":
		return 29.5735, true
	case "g":
		return 1.0, true
	case "tbsp":
		return 14.0, false
	case "tsp":
		return 14.0 / 3, false
	default:
		return 1.0, false
	}
}
