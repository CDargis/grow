package store

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/cdargis/grow/internal/model"
	"github.com/oklog/ulid/v2"
)

type LogStore struct {
	ddb       *dynamodb.Client
	tableName string
	dateGSI   string
}

func NewLogStore(ddb *dynamodb.Client, tableName, dateGSI string) *LogStore {
	return &LogStore{ddb: ddb, tableName: tableName, dateGSI: dateGSI}
}

func (s *LogStore) ListForPlant(ctx context.Context, plantID string) ([]model.Log, error) {
	out, err := s.ddb.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		KeyConditionExpression: aws.String("plantId = :pid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pid": &types.AttributeValueMemberS{Value: plantID},
		},
		ScanIndexForward: aws.Bool(false), // newest first
	})
	if err != nil {
		return nil, fmt.Errorf("list logs: %w", err)
	}
	logs := make([]model.Log, 0, len(out.Items))
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &logs); err != nil {
		return nil, fmt.Errorf("unmarshal logs: %w", err)
	}
	return logs, nil
}

func (s *LogStore) ListForDate(ctx context.Context, userID, date string) ([]model.Log, error) {
	out, err := s.ddb.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		IndexName:              aws.String(s.dateGSI),
		KeyConditionExpression: aws.String("userId = :uid AND #d = :date"),
		ExpressionAttributeNames: map[string]string{
			"#d": "date",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid":  &types.AttributeValueMemberS{Value: userID},
			":date": &types.AttributeValueMemberS{Value: date},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("list logs for date: %w", err)
	}
	logs := make([]model.Log, 0, len(out.Items))
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &logs); err != nil {
		return nil, fmt.Errorf("unmarshal logs: %w", err)
	}
	return logs, nil
}

func (s *LogStore) Create(ctx context.Context, plantID, userID string, req model.CreateLogRequest) (*model.Log, error) {
	now := time.Now().UTC()
	date := req.Date
	if date == "" {
		date = now.Format("2006-01-02")
	}
	loggedAt := req.LoggedAt
	if loggedAt == "" {
		loggedAt = now.Format(time.RFC3339)
	}
	entry := model.Log{
		PlantID:  plantID,
		LogID:    ulid.Make().String(),
		UserID:   userID,
		LogType:  req.LogType,
		Date:     date,
		LoggedAt: loggedAt,
		Data:     req.Data,
	}
	item, err := attributevalue.MarshalMap(entry)
	if err != nil {
		return nil, fmt.Errorf("marshal log: %w", err)
	}
	if _, err := s.ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	}); err != nil {
		return nil, fmt.Errorf("create log: %w", err)
	}
	return &entry, nil
}

func (s *LogStore) Delete(ctx context.Context, plantID, logID string) error {
	if _, err := s.ddb.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"plantId": &types.AttributeValueMemberS{Value: plantID},
			"logId":   &types.AttributeValueMemberS{Value: logID},
		},
	}); err != nil {
		return fmt.Errorf("delete log: %w", err)
	}
	return nil
}
