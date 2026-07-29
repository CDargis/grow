// One-time migration: link existing watering/feeding log nutrient entries to
// Inventory products, retroactively giving old logs the NPK/% dose display
// that new wizard/manual entries get automatically.
//
// Historical nutrient names are free text ("Cal-Mag", "cal mag 2.5ml", ...)
// entered before Inventory existed, so this can't be an exact join -- it
// fuzzy-matches each nutrient's name against product names (normalized
// Levenshtein similarity) and only touches entries that don't already have
// a productId.
//
// Dry run by default -- prints every match (and every unmatched name) for
// review. Pass -apply to execute. Raise/lower -min-similarity to control how
// loose an auto-applied fuzzy match can be (exact matches always apply).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

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
	Min           float64 `json:"min"           dynamodbav:"min"`
	Max           float64 `json:"max"           dynamodbav:"max"`
	Unit          string  `json:"unit"          dynamodbav:"unit"`
	PerVolume     float64 `json:"perVolume"     dynamodbav:"perVolume"`
	PerVolumeUnit string  `json:"perVolumeUnit" dynamodbav:"perVolumeUnit"`
}

type product struct {
	ProductID     string        `dynamodbav:"productId"`
	Name          string        `dynamodbav:"name"`
	NPK           npk           `dynamodbav:"npk"`
	ReferenceDose referenceDose `dynamodbav:"referenceDose"`
}

type nutrient struct {
	Name      string   `json:"name"`
	Amount    float64  `json:"amount"`
	Unit      string   `json:"unit"`
	ProductID string   `json:"productId,omitempty"`
	NPK       *npk     `json:"npk,omitempty"`
	PctOfDose *float64 `json:"pctOfDose,omitempty"`
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

var mlPerUnit = map[string]float64{"ml": 1, "l": 1000, "oz": 29.5735, "gal": 3785.41}

func convertVolume(amount float64, from, to string) float64 {
	if from == to {
		return amount
	}
	return amount * mlPerUnit[from] / mlPerUnit[to]
}

func scaledFullDose(rd referenceDose, batchAmount float64, batchUnit string) float64 {
	if batchAmount == 0 || rd.PerVolume == 0 {
		return 0
	}
	inProductUnits := convertVolume(batchAmount, batchUnit, rd.PerVolumeUnit)
	return rd.Max * (inProductUnits / rd.PerVolume)
}

// Dose units (ml/oz/g/tsp/tbsp), distinct from the batch/perVolume units
// above. "g" is mass, not volume -- converting it against the others needs
// the product's density, which we don't have, so it's refused, not guessed.
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

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// similarity is a 0..1 score (1 = identical) over normalized names.
func similarity(a, b string) float64 {
	na, nb := normalize(a), normalize(b)
	if na == nb {
		return 1
	}
	maxLen := max(len(na), len(nb))
	if maxLen == 0 {
		return 1
	}
	return 1 - float64(levenshtein(na, nb))/float64(maxLen)
}

type match struct {
	product    product
	similarity float64
}

func bestMatch(name string, products []product) (match, bool) {
	var best match
	found := false
	for _, p := range products {
		s := similarity(name, p.Name)
		if !found || s > best.similarity {
			best = match{product: p, similarity: s}
			found = true
		}
	}
	return best, found
}

func main() {
	apply := flag.Bool("apply", false, "execute the migration (default is dry run)")
	minSimilarity := flag.Float64("min-similarity", 0.75, "minimum similarity (0-1) for a fuzzy match to be applied")
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
	if len(products) == 0 {
		fmt.Println("no products found in Inventory -- add products first, then re-run this migration")
		return
	}
	fmt.Printf("found %d products in Inventory\n\n", len(products))

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
	fmt.Printf("found %d watering logs\n\n", len(logs))

	unmatchedNames := map[string]bool{}
	touchedLogs, touchedNutrients, fuzzyCount := 0, 0, 0

	for _, l := range logs {
		var wd wateringData
		if err := json.Unmarshal(l.Data, &wd); err != nil {
			log.Fatalf("parse log data %s/%s: %v", l.PlantID, l.LogID, err)
		}

		changed := false
		for i := range wd.Nutrients {
			n := &wd.Nutrients[i]
			if n.ProductID != "" || n.Name == "" {
				continue
			}
			m, found := bestMatch(n.Name, products)
			if !found || m.similarity < *minSimilarity {
				unmatchedNames[n.Name] = true
				continue
			}

			tier := "fuzzy"
			if m.similarity >= 0.999 {
				tier = "exact"
			} else {
				fuzzyCount++
			}

			n.ProductID = m.product.ProductID
			npkCopy := m.product.NPK
			n.NPK = &npkCopy
			if wd.Amount != nil && wd.Unit != "" {
				full := scaledFullDose(m.product.ReferenceDose, *wd.Amount, wd.Unit)
				if converted, ok := convertDoseAmount(n.Amount, n.Unit, m.product.ReferenceDose.Unit); ok && full > 0 {
					pct := (converted / full) * 100
					n.PctOfDose = &pct
				}
			}
			changed = true
			touchedNutrients++

			fmt.Printf("%s  %s/%s  %q -> %q (%s, %.0f%% similar)", l.Date, l.PlantID[:8], l.LogID[:8], n.Name, m.product.Name, tier, m.similarity*100)
			if n.PctOfDose != nil {
				fmt.Printf("  [%.0f%% dose]", *n.PctOfDose)
			}
			fmt.Println()
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

	if len(unmatchedNames) > 0 {
		names := make([]string, 0, len(unmatchedNames))
		for n := range unmatchedNames {
			names = append(names, n)
		}
		sort.Strings(names)
		fmt.Printf("\nno confident match (add these to Inventory, or rename, then re-run):\n")
		for _, n := range names {
			fmt.Printf("  - %q\n", n)
		}
	}

	mode := "DRY RUN — nothing written. Re-run with -apply to execute."
	if *apply {
		mode = "applied"
	}
	fmt.Printf("\n%d logs updated, %d nutrient entries linked (%d fuzzy), %d unmatched names (%s)\n",
		touchedLogs, touchedNutrients, fuzzyCount, len(unmatchedNames), mode)
}
