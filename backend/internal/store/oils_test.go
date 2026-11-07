package store

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/cdargis/grow/internal/model"
)

// "Not detected" is stored as an explicit 0, which must survive the trip
// through DynamoDB rather than being dropped as an empty value.
func TestOilRoundTripKeepsNotDetected(t *testing.T) {
	in := model.Batch{
		BatchID: "b", UserID: "u", Code: "FS-261107", Type: model.BatchFS,
		Inputs: []model.BatchInput{{
			Kind: model.InputFlower, Grams: 10, Strain: "x",
			Profile:    &model.Profile{Cannabinoids: map[string]float64{"cbda": 120, "d9thc": 0}},
			Efficiency: 0.7, Method: "m", Confidence: model.ConfidenceLow,
		}},
		DensityGPerMl: 0.94, TotalGrams: 100,
	}
	item, err := attributevalue.MarshalMap(in)
	if err != nil {
		t.Fatal(err)
	}
	var out model.Batch
	if err := attributevalue.UnmarshalMap(item, &out); err != nil {
		t.Fatal(err)
	}
	v, ok := out.Inputs[0].Profile.Cannabinoids["d9thc"]
	if !ok || v != 0 {
		t.Errorf("d9thc after round trip = %v, present=%v; want explicit 0", v, ok)
	}
	if out.Inputs[0].Efficiency != 0.7 || out.Inputs[0].Confidence != model.ConfidenceLow {
		t.Errorf("flower fields lost: %+v", out.Inputs[0])
	}
}
