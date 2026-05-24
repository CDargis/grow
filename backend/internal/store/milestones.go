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
)

type MilestoneStore struct {
	ddb          *dynamodb.Client
	tableName    string
	historyTable string
}

func NewMilestoneStore(ddb *dynamodb.Client, tableName, historyTable string) *MilestoneStore {
	return &MilestoneStore{ddb: ddb, tableName: tableName, historyTable: historyTable}
}

func (s *MilestoneStore) ListForPlant(ctx context.Context, plantID string) ([]model.Milestone, error) {
	out, err := s.ddb.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		KeyConditionExpression: aws.String("plantId = :pid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pid": &types.AttributeValueMemberS{Value: plantID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("list milestones: %w", err)
	}
	milestones := make([]model.Milestone, 0, len(out.Items))
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &milestones); err != nil {
		return nil, fmt.Errorf("unmarshal milestones: %w", err)
	}
	return milestones, nil
}

func (s *MilestoneStore) Upsert(ctx context.Context, m model.Milestone) error {
	m.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	item, err := attributevalue.MarshalMap(m)
	if err != nil {
		return fmt.Errorf("marshal milestone: %w", err)
	}
	if _, err := s.ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	}); err != nil {
		return fmt.Errorf("upsert milestone: %w", err)
	}
	return nil
}

func (s *MilestoneStore) Confirm(ctx context.Context, plantID string, milestoneType model.MilestoneType, confirmedDate string) error {
	_, err := s.ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"plantId":       &types.AttributeValueMemberS{Value: plantID},
			"milestoneType": &types.AttributeValueMemberS{Value: string(milestoneType)},
		},
		UpdateExpression: aws.String("SET #s = :status, confirmedDate = :cd, updatedAt = :ua"),
		ExpressionAttributeNames: map[string]string{"#s": "status"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: string(model.MilestoneStatusConfirmed)},
			":cd":     &types.AttributeValueMemberS{Value: confirmedDate},
			":ua":     &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
		},
	})
	if err != nil {
		return fmt.Errorf("confirm milestone: %w", err)
	}
	return nil
}

func (s *MilestoneStore) Skip(ctx context.Context, plantID string, milestoneType model.MilestoneType) error {
	_, err := s.ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"plantId":       &types.AttributeValueMemberS{Value: plantID},
			"milestoneType": &types.AttributeValueMemberS{Value: string(milestoneType)},
		},
		UpdateExpression: aws.String("SET #s = :status, updatedAt = :ua"),
		ExpressionAttributeNames: map[string]string{"#s": "status"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: string(model.MilestoneStatusSkipped)},
			":ua":     &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
		},
	})
	if err != nil {
		return fmt.Errorf("skip milestone: %w", err)
	}
	return nil
}

func (s *MilestoneStore) AppendHistory(ctx context.Context, h model.MilestoneRecalibrationHistory) error {
	item, err := attributevalue.MarshalMap(h)
	if err != nil {
		return fmt.Errorf("marshal recal history: %w", err)
	}
	if _, err := s.ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.historyTable),
		Item:      item,
	}); err != nil {
		return fmt.Errorf("append recal history: %w", err)
	}
	return nil
}
