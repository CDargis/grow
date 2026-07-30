// One-time migration for Phase 1 of the delivered-NPK feature (see
// delivered-npk-feature-spec.md). Two passes, in order:
//
//  1. Backfill Inventory products created before this feature existed --
//     they have no ElementalNPK/Density/DensityCalibrated. Computes
//     ElementalNPK from the label NPK (P as P2O5 x 0.436, K as K2O x 0.83)
//     and a unit-appropriate default Density, same as the live product
//     store does for new products.
//  2. Backfill delivered elemental grams on watering-log nutrient entries
//     that already have a productId (linked either by the live app or by
//     the earlier migrate-nutrient-products run) but predate this feature,
//     using the now-complete product data from pass 1.
//
// Nutrient entries with no productId are untouched -- run
// migrate-nutrient-products first if you want more of them linked.
//
// Dry run by default -- prints every computed value for review. Pass
// -apply to execute.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type npk struct {
	N float64 `json:"n" dynamodbav:"n"`
	P float64 `json:"p" dynamodbav:"p"`
	K float64 `json:"k" dynamodbav:"k"`
}

type referenceDose struct {
	Min           float64 `dynamodbav:"min"`
	Max           float64 `dynamodbav:"max"`
	Unit          string  `dynamodbav:"unit"`
	PerVolume     float64 `dynamodbav:"perVolume"`
	PerVolumeUnit string  `dynamodbav:"perVolumeUnit"`
}

type product struct {
	ProductID         string        `dynamodbav:"productId"`
	Name              string        `dynamodbav:"name"`
	Form              string        `dynamodbav:"form"`
	NPK               npk           `dynamodbav:"npk"`
	ElementalNPK      npk           `dynamodbav:"elementalNpk"`
	ReferenceDose     referenceDose `dynamodbav:"referenceDose"`
	Density           float64       `dynamodbav:"density"`
	DensityCalibrated bool          `dynamodbav:"densityCalibrated"`
}

type nutrient struct {
	Name       string   `json:"name"`
	Amount     float64  `json:"amount"`
	Unit       string   `json:"unit"`
	ProductID  string   `json:"productId,omitempty"`
	NPK        *npk     `json:"npk,omitempty"`
	PctOfDose  *float64 `json:"pctOfDose,omitempty"`
	DeliveredN *float64 `json:"deliveredN,omitempty"`
	DeliveredP *float64 `json:"deliveredP,omitempty"`
	DeliveredK *float64 `json:"deliveredK,omitempty"`
}

type wateringData struct {
	Amount    *float64   `json:"amount,omitempty"`
	Unit      string     `json:"unit,omitempty"`
	Ph        *float64   `json:"ph,omitempty"`
	Runoff    *float64   `json:"runoff,omitempty"`
	Tds       *float64   `json:"tds,omitempty"`
	RunoffTds *float64   `json:"runoffTds,omitempty"`
	Nutrients []nutrient `json:"nutrients,omitempty"`
	Note      string     `json:"note,omitempty"`
}

type logItem struct {
	PlantID string `dynamodbav:"plantId"`
	LogID   string `dynamodbav:"logId"`
	LogType string `dynamodbav:"logType"`
	Date    string `dynamodbav:"date"`
	Data    []byte `dynamodbav:"data"`
}

// elementalFromLabel mirrors backend/internal/model.ElementalFromLabel --
// fertilizer labels report P as P2O5 and K as K2O (oxide forms), not
// elemental; N is already elemental.
func elementalFromLabel(label npk) npk {
	return npk{N: label.N, P: label.P * 0.436, K: label.K * 0.83}
}

// defaultDensity mirrors backend/internal/model.DefaultDensity.
func defaultDensity(unit string) (density float64, calibrated bool) {
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

// Dose units (ml/oz/g/tsp/tbsp), mirroring frontend lib/nutrientDose.ts. "g"
// is mass -- converting it against the volume-based ones needs a density
// conversion done separately (via massGrams below), not this table.
var doseMlPerUnit = map[string]float64{"ml": 1, "oz": 29.5735, "tsp": 4.92892, "tbsp": 14.7868}

func convertDoseAmount(amount float64, from, to string) (float64, bool) {
	if from == to {
		return amount, true
	}
	fv, fok := doseMlPerUnit[from]
	tv, tok := doseMlPerUnit[to]
	if !fok || !tok {
		return 0, false
	}
	return amount * fv / tv, true
}

func main() {
	apply := flag.Bool("apply", false, "execute the migration (default is dry run)")
	flag.Parse()

	productsTable := os.Getenv("PRODUCTS_TABLE")
	if productsTable == "" {
		productsTable = "grow-products"
	}
	logsTable := os.Getenv("LOGS_TABLE")
	if logsTable == "" {
		logsTable = "grow-logs"
	}
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		log.Fatalf("AWS_REGION must be set")
	}

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithSharedConfigProfile(os.Getenv("AWS_PROFILE")),
	)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	ddb := dynamodb.NewFromConfig(cfg)

	// ── Pass 1: backfill products ────────────────────────────────────────

	var products []product
	{
		var lastKey map[string]types.AttributeValue
		for {
			input := &dynamodb.ScanInput{TableName: aws.String(productsTable)}
			if lastKey != nil {
				input.ExclusiveStartKey = lastKey
			}
			out, err := ddb.Scan(ctx, input)
			if err != nil {
				log.Fatalf("scan products: %v", err)
			}
			var page []product
			if err := attributevalue.UnmarshalListOfMaps(out.Items, &page); err != nil {
				log.Fatalf("unmarshal products: %v", err)
			}
			products = append(products, page...)
			lastKey = out.LastEvaluatedKey
			if lastKey == nil {
				break
			}
		}
	}

	productsByID := map[string]product{}
	productsBackfilled := 0
	for i := range products {
		p := &products[i]
		productsByID[p.ProductID] = *p
		if p.Density > 0 {
			continue // already has this feature's fields
		}
		elemental := elementalFromLabel(p.NPK)
		density, calibrated := defaultDensity(p.ReferenceDose.Unit)
		fmt.Printf("product %q (%s): label %.2f-%.2f-%.2f -> elemental %.2f-%.2f-%.2f, density %.3f g/%s (calibrated=%v)\n",
			p.Name, p.ProductID[:8], p.NPK.N, p.NPK.P, p.NPK.K, elemental.N, elemental.P, elemental.K, density, p.ReferenceDose.Unit, calibrated)

		p.ElementalNPK = elemental
		p.Density = density
		p.DensityCalibrated = calibrated
		productsByID[p.ProductID] = *p
		productsBackfilled++

		if !*apply {
			continue
		}
		elementalAV, err := attributevalue.Marshal(elemental)
		if err != nil {
			log.Fatalf("marshal elemental npk: %v", err)
		}
		if _, err := ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(productsTable),
			Key: map[string]types.AttributeValue{
				"productId": &types.AttributeValueMemberS{Value: p.ProductID},
			},
			UpdateExpression: aws.String("SET elementalNpk = :en, density = :d, densityCalibrated = :dc"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":en": elementalAV,
				":d":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%v", density)},
				":dc": &types.AttributeValueMemberBOOL{Value: calibrated},
			},
		}); err != nil {
			log.Fatalf("update product %s: %v", p.ProductID, err)
		}
	}
	fmt.Printf("\n%d products backfilled\n\n", productsBackfilled)

	// ── Pass 2: backfill delivered grams on already-linked log entries ──

	var logs []logItem
	{
		var lastKey map[string]types.AttributeValue
		for {
			input := &dynamodb.ScanInput{
				TableName:        aws.String(logsTable),
				FilterExpression: aws.String("logType = :w"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":w": &types.AttributeValueMemberS{Value: "watering"},
				},
			}
			if lastKey != nil {
				input.ExclusiveStartKey = lastKey
			}
			out, err := ddb.Scan(ctx, input)
			if err != nil {
				log.Fatalf("scan logs: %v", err)
			}
			var page []logItem
			if err := attributevalue.UnmarshalListOfMaps(out.Items, &page); err != nil {
				log.Fatalf("unmarshal logs: %v", err)
			}
			logs = append(logs, page...)
			lastKey = out.LastEvaluatedKey
			if lastKey == nil {
				break
			}
		}
	}

	touchedLogs, touchedNutrients, skippedNoProduct, skippedUnitMismatch := 0, 0, 0, 0

	for _, l := range logs {
		var wd wateringData
		if err := json.Unmarshal(l.Data, &wd); err != nil {
			log.Fatalf("parse log data %s/%s: %v", l.PlantID, l.LogID, err)
		}

		changed := false
		for i := range wd.Nutrients {
			n := &wd.Nutrients[i]
			if n.ProductID == "" || n.DeliveredN != nil {
				continue
			}
			p, ok := productsByID[n.ProductID]
			if !ok {
				fmt.Printf("%s  %s/%s  %q: linked productId %s not found -- skipping\n", l.Date, l.PlantID[:8], l.LogID[:8], n.Name, n.ProductID)
				skippedNoProduct++
				continue
			}
			converted, ok := convertDoseAmount(n.Amount, n.Unit, p.ReferenceDose.Unit)
			if !ok {
				fmt.Printf("%s  %s/%s  %q: can't convert %v%s to %s -- skipping\n", l.Date, l.PlantID[:8], l.LogID[:8], n.Name, n.Amount, n.Unit, p.ReferenceDose.Unit)
				skippedUnitMismatch++
				continue
			}
			massGrams := converted * p.Density
			dN, dP, dK := massGrams*(p.ElementalNPK.N/100), massGrams*(p.ElementalNPK.P/100), massGrams*(p.ElementalNPK.K/100)
			n.DeliveredN, n.DeliveredP, n.DeliveredK = &dN, &dP, &dK
			changed = true
			touchedNutrients++
			fmt.Printf("%s  %s/%s  %q: %v%s -> N %.3fg · P %.3fg · K %.3fg\n", l.Date, l.PlantID[:8], l.LogID[:8], n.Name, n.Amount, n.Unit, dN, dP, dK)
		}

		if !changed {
			continue
		}
		touchedLogs++

		if !*apply {
			continue
		}
		newData, err := json.Marshal(wd)
		if err != nil {
			log.Fatalf("marshal updated data %s/%s: %v", l.PlantID, l.LogID, err)
		}
		if _, err := ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(logsTable),
			Key: map[string]types.AttributeValue{
				"plantId": &types.AttributeValueMemberS{Value: l.PlantID},
				"logId":   &types.AttributeValueMemberS{Value: l.LogID},
			},
			UpdateExpression:          aws.String("SET #d = :data"),
			ExpressionAttributeNames:  map[string]string{"#d": "data"},
			ExpressionAttributeValues: map[string]types.AttributeValue{":data": &types.AttributeValueMemberB{Value: newData}},
		}); err != nil {
			log.Fatalf("update %s/%s: %v", l.PlantID, l.LogID, err)
		}
	}

	mode := "DRY RUN — nothing written. Re-run with -apply to execute."
	if *apply {
		mode = "applied"
	}
	fmt.Printf("\n%d logs updated, %d nutrient entries backfilled with delivered grams, %d skipped (product not found), %d skipped (unit mismatch) (%s)\n",
		touchedLogs, touchedNutrients, skippedNoProduct, skippedUnitMismatch, mode)
}
