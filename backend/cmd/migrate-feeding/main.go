// One-time migration: collapse 'feeding' logs into the unified 'watering' shape
// (nutrients live on WateringData now). Also detects watering+feeding pairs that
// recorded the same real-world event (same plant, same date, matching volume,
// logged within an hour) and merges them: the feeding row absorbs the watering's
// runoff/note/amount-unit and the duplicate watering row is deleted.
//
// Dry run by default — prints the full plan. Pass -apply to execute.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type nutrient struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
	Unit   string  `json:"unit"`
}

type feedingData struct {
	Nutrients []nutrient `json:"nutrients,omitempty"`
	Ph        *float64   `json:"ph,omitempty"`
	TotalVol  *float64   `json:"totalVol,omitempty"`
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
	PlantID     string `dynamodbav:"plantId"`
	LogID       string `dynamodbav:"logId"`
	LogType     string `dynamodbav:"logType"`
	Date        string `dynamodbav:"date"`
	LoggedAt    string `dynamodbav:"loggedAt"`
	LogTypeDate string `dynamodbav:"logTypeDate"`
	Data        []byte `dynamodbav:"data"`
}

func toMl(amount float64, unit string) float64 {
	switch unit {
	case "ml":
		return amount
	case "l":
		return amount * 1000
	case "oz":
		return amount * 29.5735
	case "gal":
		return amount * 3785.41
	default:
		return amount
	}
}

func parseTime(iso string) time.Time {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return time.Time{}
	}
	return t
}

func main() {
	apply := flag.Bool("apply", false, "execute the migration (default is dry run)")
	force := flag.String("force-merge", "", "comma-separated feedingLogId=wateringLogId pairs to merge regardless of auto-match rules")
	flag.Parse()

	forcePairs := map[string]string{}
	if *force != "" {
		for _, pair := range strings.Split(*force, ",") {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) != 2 {
				log.Fatalf("bad -force-merge pair %q", pair)
			}
			forcePairs[parts[0]] = parts[1]
		}
	}

	table := os.Getenv("LOGS_TABLE")
	if table == "" {
		table = "grow-logs"
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

	// Collect all feeding and watering logs
	var feedings, waterings []logItem
	var lastKey map[string]types.AttributeValue
	for {
		input := &dynamodb.ScanInput{
			TableName:        aws.String(table),
			FilterExpression: aws.String("logType IN (:f, :w)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":f": &types.AttributeValueMemberS{Value: "feeding"},
				":w": &types.AttributeValueMemberS{Value: "watering"},
			},
		}
		if lastKey != nil {
			input.ExclusiveStartKey = lastKey
		}
		out, err := ddb.Scan(ctx, input)
		if err != nil {
			log.Fatalf("scan: %v", err)
		}
		for _, raw := range out.Items {
			var item logItem
			if err := attributevalue.UnmarshalMap(raw, &item); err != nil {
				log.Fatalf("unmarshal item: %v", err)
			}
			if item.LogType == "feeding" {
				feedings = append(feedings, item)
			} else {
				waterings = append(waterings, item)
			}
		}
		lastKey = out.LastEvaluatedKey
		if lastKey == nil {
			break
		}
	}
	fmt.Printf("found %d feeding logs, %d watering logs\n\n", len(feedings), len(waterings))

	sort.Slice(feedings, func(i, j int) bool { return feedings[i].LoggedAt < feedings[j].LoggedAt })

	usedWaterings := map[string]bool{}
	deleteIDs := map[string]logItem{}

	for _, f := range feedings {
		var fd feedingData
		if err := json.Unmarshal(f.Data, &fd); err != nil {
			log.Fatalf("parse feeding data %s/%s: %v", f.PlantID, f.LogID, err)
		}

		merged := wateringData{Nutrients: fd.Nutrients, Ph: fd.Ph}
		if fd.TotalVol != nil {
			merged.Amount = fd.TotalVol
			merged.Unit = "ml"
		}
		newLoggedAt := f.LoggedAt

		// Look for a twin watering: same plant + date, volume matches the
		// feeding's totalVol. Sessions can involve several distinct mixes
		// minutes apart, so a matching pH outranks a closer timestamp.
		var twin *logItem
		if forced, ok := forcePairs[f.LogID]; ok {
			for i := range waterings {
				if waterings[i].LogID == forced {
					twin = &waterings[i]
					break
				}
			}
			if twin == nil {
				log.Fatalf("force-merge: watering %s not found (or not a watering)", forced)
			}
			if usedWaterings[twin.LogID] {
				log.Fatalf("force-merge: watering %s already merged with another feeding", forced)
			}
		} else if fd.TotalVol != nil {
			fTime := parseTime(f.LoggedAt)
			bestGap := time.Duration(math.MaxInt64)
			bestPh := false
			for i := range waterings {
				w := &waterings[i]
				if usedWaterings[w.LogID] || w.PlantID != f.PlantID || w.Date != f.Date {
					continue
				}
				var wd wateringData
				if err := json.Unmarshal(w.Data, &wd); err != nil || wd.Amount == nil {
					continue
				}
				if math.Abs(toMl(*wd.Amount, wd.Unit)-*fd.TotalVol) > 1 {
					continue
				}
				phMatch := fd.Ph != nil && wd.Ph != nil && *fd.Ph == *wd.Ph
				gap := fTime.Sub(parseTime(w.LoggedAt))
				if gap < 0 {
					gap = -gap
				}
				if (phMatch && !bestPh) || (phMatch == bestPh && gap < bestGap) {
					bestGap = gap
					bestPh = phMatch
					twin = w
				}
			}
		}

		if twin != nil {
			usedWaterings[twin.LogID] = true
			deleteIDs[twin.LogID] = *twin
			var wd wateringData
			_ = json.Unmarshal(twin.Data, &wd)
			// Keep the watering's nicer units and its measurements
			merged.Amount = wd.Amount
			merged.Unit = wd.Unit
			if merged.Ph == nil {
				merged.Ph = wd.Ph
			}
			merged.Runoff = wd.Runoff
			merged.Note = wd.Note
			if twin.LoggedAt < newLoggedAt {
				newLoggedAt = twin.LoggedAt
			}
		}

		newData, err := json.Marshal(merged)
		if err != nil {
			log.Fatalf("marshal merged data: %v", err)
		}

		fmt.Printf("feeding %s  %s/%s\n", f.Date, f.PlantID[:8], f.LogID[:8])
		fmt.Printf("  old: %s\n", string(f.Data))
		if twin != nil {
			fmt.Printf("  MERGE with watering %s/%s (%s): %s\n", twin.PlantID[:8], twin.LogID[:8], twin.LoggedAt[11:16], string(twin.Data))
		}
		fmt.Printf("  new: watering %s\n\n", string(newData))

		if !*apply {
			continue
		}

		if _, err := ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(table),
			Key: map[string]types.AttributeValue{
				"plantId": &types.AttributeValueMemberS{Value: f.PlantID},
				"logId":   &types.AttributeValueMemberS{Value: f.LogID},
			},
			UpdateExpression: aws.String("SET logType = :t, logTypeDate = :ltd, loggedAt = :la, #d = :data"),
			ExpressionAttributeNames: map[string]string{"#d": "data"},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":t":    &types.AttributeValueMemberS{Value: "watering"},
				":ltd":  &types.AttributeValueMemberS{Value: "watering#" + f.Date},
				":la":      &types.AttributeValueMemberS{Value: newLoggedAt},
				":data":    &types.AttributeValueMemberB{Value: newData},
				":feeding": &types.AttributeValueMemberS{Value: "feeding"},
			},
			ConditionExpression: aws.String("logType = :feeding"),
		}); err != nil {
			log.Fatalf("update %s/%s: %v", f.PlantID, f.LogID, err)
		}
		fmt.Printf("  ✔ converted\n")
	}

	if *apply {
		for _, w := range deleteIDs {
			if _, err := ddb.DeleteItem(ctx, &dynamodb.DeleteItemInput{
				TableName: aws.String(table),
				Key: map[string]types.AttributeValue{
					"plantId": &types.AttributeValueMemberS{Value: w.PlantID},
					"logId":   &types.AttributeValueMemberS{Value: w.LogID},
				},
			}); err != nil {
				log.Fatalf("delete watering %s/%s: %v", w.PlantID, w.LogID, err)
			}
			fmt.Printf("✔ deleted merged watering %s/%s\n", w.PlantID[:8], w.LogID[:8])
		}
	}

	mode := "DRY RUN — nothing written. Re-run with -apply to execute."
	if *apply {
		mode = "applied"
	}
	fmt.Printf("\n%d feedings converted, %d twin waterings merged+deleted (%s)\n", len(feedings), len(deleteIDs), mode)
}
