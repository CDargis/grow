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

type EnvironmentStore struct {
	ddb       *dynamodb.Client
	tableName string
	indexName string
}

func NewEnvironmentStore(ddb *dynamodb.Client, tableName string) *EnvironmentStore {
	return &EnvironmentStore{ddb: ddb, tableName: tableName, indexName: "user-index"}
}

func (s *EnvironmentStore) List(ctx context.Context, userID string) ([]model.Environment, error) {
	out, err := s.ddb.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		IndexName:              aws.String(s.indexName),
		KeyConditionExpression: aws.String("userId = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: userID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	envs := make([]model.Environment, 0, len(out.Items))
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &envs); err != nil {
		return nil, fmt.Errorf("unmarshal environments: %w", err)
	}
	return envs, nil
}

func (s *EnvironmentStore) Get(ctx context.Context, environmentID string) (*model.Environment, error) {
	out, err := s.ddb.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"environmentId": &types.AttributeValueMemberS{Value: environmentID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get environment: %w", err)
	}
	if out.Item == nil {
		return nil, nil
	}
	env := &model.Environment{}
	if err := attributevalue.UnmarshalMap(out.Item, env); err != nil {
		return nil, fmt.Errorf("unmarshal environment: %w", err)
	}
	return env, nil
}

func (s *EnvironmentStore) Create(ctx context.Context, userID string, req model.CreateEnvironmentRequest) (*model.Environment, error) {
	now := time.Now().UTC()
	env := model.Environment{
		EnvironmentID:  ulid.Make().String(),
		UserID:         userID,
		Name:           req.Name,
		Type:           req.Type,
		LightSchedule:  req.LightSchedule,
		TargetTempF:    req.TargetTempF,
		TargetHumidity: req.TargetHumidity,
		CreatedAt:      now.Format(time.RFC3339),
	}
	item, err := attributevalue.MarshalMap(env)
	if err != nil {
		return nil, fmt.Errorf("marshal environment: %w", err)
	}
	if _, err := s.ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	}); err != nil {
		return nil, fmt.Errorf("create environment: %w", err)
	}
	return &env, nil
}

func (s *EnvironmentStore) Update(ctx context.Context, environmentID string, req model.CreateEnvironmentRequest) (*model.Environment, error) {
	existing, err := s.Get(ctx, environmentID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("environment not found: %s", environmentID)
	}
	existing.Name           = req.Name
	existing.Type           = req.Type
	existing.LightSchedule  = req.LightSchedule
	existing.TargetTempF    = req.TargetTempF
	existing.TargetHumidity = req.TargetHumidity
	item, err := attributevalue.MarshalMap(existing)
	if err != nil {
		return nil, fmt.Errorf("marshal environment: %w", err)
	}
	if _, err := s.ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	}); err != nil {
		return nil, fmt.Errorf("update environment: %w", err)
	}
	return existing, nil
}

func (s *EnvironmentStore) Delete(ctx context.Context, environmentID string) error {
	if _, err := s.ddb.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"environmentId": &types.AttributeValueMemberS{Value: environmentID},
		},
	}); err != nil {
		return fmt.Errorf("delete environment: %w", err)
	}
	return nil
}
