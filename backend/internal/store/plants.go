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

type PlantStore struct {
	ddb       *dynamodb.Client
	tableName string
	indexName string
}

func NewPlantStore(ddb *dynamodb.Client, tableName string) *PlantStore {
	return &PlantStore{ddb: ddb, tableName: tableName, indexName: "user-index"}
}

func (s *PlantStore) List(ctx context.Context, userID string) ([]model.Plant, error) {
	out, err := s.ddb.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		IndexName:              aws.String(s.indexName),
		KeyConditionExpression: aws.String("userId = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: userID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("list plants: %w", err)
	}
	plants := make([]model.Plant, 0, len(out.Items))
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &plants); err != nil {
		return nil, fmt.Errorf("unmarshal plants: %w", err)
	}
	return plants, nil
}

func (s *PlantStore) Get(ctx context.Context, plantID string) (*model.Plant, error) {
	out, err := s.ddb.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"plantId": &types.AttributeValueMemberS{Value: plantID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get plant: %w", err)
	}
	if out.Item == nil {
		return nil, nil
	}
	plant := &model.Plant{}
	if err := attributevalue.UnmarshalMap(out.Item, plant); err != nil {
		return nil, fmt.Errorf("unmarshal plant: %w", err)
	}
	return plant, nil
}

func (s *PlantStore) Create(ctx context.Context, userID string, req model.CreatePlantRequest) (*model.Plant, error) {
	now := time.Now().UTC()
	plant := model.Plant{
		PlantID:        ulid.Make().String(),
		UserID:         userID,
		Name:           req.Name,
		Strain:         req.Strain,
		Genetics:       req.Genetics,
		SeedBank:       req.SeedBank,
		PlantType:      req.PlantType,
		Phase:          req.Phase,
		PhaseStartDate: now.Format("2006-01-02"),
		EnvironmentID:  req.EnvironmentID,
		CreatedAt:      now.Format(time.RFC3339),
	}
	item, err := attributevalue.MarshalMap(plant)
	if err != nil {
		return nil, fmt.Errorf("marshal plant: %w", err)
	}
	if _, err := s.ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	}); err != nil {
		return nil, fmt.Errorf("create plant: %w", err)
	}
	return &plant, nil
}

func (s *PlantStore) UpdateAvatar(ctx context.Context, plantID, avatarKey string) error {
	_, err := s.ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"plantId": &types.AttributeValueMemberS{Value: plantID},
		},
		UpdateExpression: aws.String("SET avatarKey = :ak"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":ak": &types.AttributeValueMemberS{Value: avatarKey},
		},
	})
	if err != nil {
		return fmt.Errorf("update avatar: %w", err)
	}
	return nil
}

func (s *PlantStore) UpdatePhase(ctx context.Context, plantID string, phase model.PlantPhase, phaseStartDate string) (model.PlantPhase, error) {
	plant, err := s.Get(ctx, plantID)
	if err != nil {
		return "", err
	}
	if plant == nil {
		return "", fmt.Errorf("plant not found: %s", plantID)
	}
	fromPhase := plant.Phase
	_, err = s.ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"plantId": &types.AttributeValueMemberS{Value: plantID},
		},
		UpdateExpression: aws.String("SET #ph = :phase, phaseStartDate = :psd"),
		ExpressionAttributeNames: map[string]string{"#ph": "phase"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":phase": &types.AttributeValueMemberS{Value: string(phase)},
			":psd":   &types.AttributeValueMemberS{Value: phaseStartDate},
		},
	})
	if err != nil {
		return "", fmt.Errorf("update phase: %w", err)
	}
	return fromPhase, nil
}

// AssignEnvironment updates a plant's environmentId and returns the previous value.
// Pass nil toEnvID to remove the assignment.
func (s *PlantStore) AssignEnvironment(ctx context.Context, plantID string, toEnvID *string) (fromEnvID string, err error) {
	plant, err := s.Get(ctx, plantID)
	if err != nil {
		return "", err
	}
	if plant == nil {
		return "", fmt.Errorf("plant not found: %s", plantID)
	}
	fromEnvID = plant.EnvironmentID

	if toEnvID != nil && *toEnvID != "" {
		_, err = s.ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(s.tableName),
			Key: map[string]types.AttributeValue{
				"plantId": &types.AttributeValueMemberS{Value: plantID},
			},
			UpdateExpression: aws.String("SET environmentId = :eid"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":eid": &types.AttributeValueMemberS{Value: *toEnvID},
			},
		})
	} else {
		_, err = s.ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(s.tableName),
			Key: map[string]types.AttributeValue{
				"plantId": &types.AttributeValueMemberS{Value: plantID},
			},
			UpdateExpression: aws.String("REMOVE environmentId"),
		})
	}
	if err != nil {
		return "", fmt.Errorf("assign environment: %w", err)
	}
	return fromEnvID, nil
}

func (s *PlantStore) UpdateDetails(ctx context.Context, plantID string, req model.UpdatePlantDetailsRequest) (*model.Plant, error) {
	plant, err := s.Get(ctx, plantID)
	if err != nil {
		return nil, err
	}
	if plant == nil {
		return nil, fmt.Errorf("plant not found: %s", plantID)
	}
	plant.Name      = req.Name
	plant.Strain    = req.Strain
	plant.Genetics  = req.Genetics
	plant.SeedBank  = req.SeedBank
	plant.PlantType = req.PlantType
	item, err := attributevalue.MarshalMap(plant)
	if err != nil {
		return nil, fmt.Errorf("marshal plant: %w", err)
	}
	if _, err := s.ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	}); err != nil {
		return nil, fmt.Errorf("update plant: %w", err)
	}
	return plant, nil
}

func (s *PlantStore) Delete(ctx context.Context, plantID string) error {
	if _, err := s.ddb.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"plantId": &types.AttributeValueMemberS{Value: plantID},
		},
	}); err != nil {
		return fmt.Errorf("delete plant: %w", err)
	}
	return nil
}
