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

type ObservationStore struct {
	ddb       *dynamodb.Client
	tableName string
}

func NewObservationStore(ddb *dynamodb.Client, tableName string) *ObservationStore {
	return &ObservationStore{ddb: ddb, tableName: tableName}
}

func (s *ObservationStore) ListForPlant(ctx context.Context, plantID string) ([]model.PlantObservation, error) {
	out, err := s.ddb.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		KeyConditionExpression: aws.String("plantId = :pid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pid": &types.AttributeValueMemberS{Value: plantID},
		},
		ScanIndexForward: aws.Bool(false),
	})
	if err != nil {
		return nil, fmt.Errorf("list observations: %w", err)
	}
	obs := make([]model.PlantObservation, 0, len(out.Items))
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &obs); err != nil {
		return nil, fmt.Errorf("unmarshal observations: %w", err)
	}
	return obs, nil
}

func (s *ObservationStore) Create(ctx context.Context, plantID string, o model.PlantObservation) (*model.PlantObservation, error) {
	o.PlantID       = plantID
	o.ObservationID = ulid.Make().String()
	o.ObservedAt    = time.Now().UTC().Format(time.RFC3339)
	item, err := attributevalue.MarshalMap(o)
	if err != nil {
		return nil, fmt.Errorf("marshal observation: %w", err)
	}
	if _, err := s.ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	}); err != nil {
		return nil, fmt.Errorf("create observation: %w", err)
	}
	return &o, nil
}

func (s *ObservationStore) DeleteUnactioned(ctx context.Context, plantID string) error {
	existing, err := s.ListForPlant(ctx, plantID)
	if err != nil {
		return err
	}
	for _, o := range existing {
		if o.ActionedAt != "" {
			continue
		}
		if _, err := s.ddb.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(s.tableName),
			Key: map[string]types.AttributeValue{
				"plantId":       &types.AttributeValueMemberS{Value: plantID},
				"observationId": &types.AttributeValueMemberS{Value: o.ObservationID},
			},
		}); err != nil {
			return fmt.Errorf("delete observation %s: %w", o.ObservationID, err)
		}
	}
	return nil
}

func (s *ObservationStore) MarkActioned(ctx context.Context, plantID, observationID, actionNote string) error {
	_, err := s.ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"plantId":       &types.AttributeValueMemberS{Value: plantID},
			"observationId": &types.AttributeValueMemberS{Value: observationID},
		},
		UpdateExpression: aws.String("SET actionedAt = :aa, actionNote = :an"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":aa": &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
			":an": &types.AttributeValueMemberS{Value: actionNote},
		},
	})
	if err != nil {
		return fmt.Errorf("mark actioned: %w", err)
	}
	return nil
}
