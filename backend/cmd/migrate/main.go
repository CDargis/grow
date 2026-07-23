package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func main() {
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

	var lastKey map[string]types.AttributeValue
	updated, skipped := 0, 0

	for {
		input := &dynamodb.ScanInput{
			TableName:        aws.String(table),
			FilterExpression: aws.String("attribute_not_exists(logTypeDate)"),
		}
		if lastKey != nil {
			input.ExclusiveStartKey = lastKey
		}

		out, err := ddb.Scan(ctx, input)
		if err != nil {
			log.Fatalf("scan: %v", err)
		}

		for _, item := range out.Items {
			logType, ok1 := item["logType"].(*types.AttributeValueMemberS)
			date, ok2 := item["date"].(*types.AttributeValueMemberS)
			plantId, ok3 := item["plantId"].(*types.AttributeValueMemberS)
			logId, ok4 := item["logId"].(*types.AttributeValueMemberS)
			if !ok1 || !ok2 || !ok3 || !ok4 {
				skipped++
				continue
			}

			logTypeDate := logType.Value + "#" + date.Value
			_, err := ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
				TableName: aws.String(table),
				Key: map[string]types.AttributeValue{
					"plantId": &types.AttributeValueMemberS{Value: plantId.Value},
					"logId":   &types.AttributeValueMemberS{Value: logId.Value},
				},
				UpdateExpression: aws.String("SET logTypeDate = :v"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":v": &types.AttributeValueMemberS{Value: logTypeDate},
				},
				ConditionExpression: aws.String("attribute_not_exists(logTypeDate)"),
			})
			if err != nil {
				log.Printf("skip %s/%s: %v", plantId.Value, logId.Value, err)
				skipped++
				continue
			}
			updated++
			fmt.Printf("  updated %s/%s → %s\n", plantId.Value, logId.Value, logTypeDate)
		}

		lastKey = out.LastEvaluatedKey
		if lastKey == nil {
			break
		}
	}

	fmt.Printf("\ndone: %d updated, %d skipped\n", updated, skipped)
}
