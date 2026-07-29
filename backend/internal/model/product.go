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
	ReferenceDose ReferenceDose `dynamodbav:"referenceDose" json:"referenceDose"`
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
	StockQty      float64       `json:"stockQty,omitempty"`
	StockUnit     string        `json:"stockUnit,omitempty"`
	Notes         string        `json:"notes,omitempty"`
}
