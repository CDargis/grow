package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/cdargis/grow/internal/model"
)

type SettingsStore struct {
	ddb       *dynamodb.Client
	tableName string
}

func NewSettingsStore(ddb *dynamodb.Client, tableName string) *SettingsStore {
	return &SettingsStore{ddb: ddb, tableName: tableName}
}

func (s *SettingsStore) Get(ctx context.Context, userID string) (*model.Settings, error) {
	out, err := s.ddb.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"userId": &types.AttributeValueMemberS{Value: userID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}
	settings := &model.Settings{UserID: userID}
	if out.Item == nil {
		return settings, nil
	}
	if err := attributevalue.UnmarshalMap(out.Item, settings); err != nil {
		return nil, fmt.Errorf("unmarshal settings: %w", err)
	}
	return settings, nil
}

// Update merges only the fields present on req into the stored settings item,
// so independent editors (shortcuts, sort order, layout mode) never clobber
// each other's fields.
func (s *SettingsStore) Update(ctx context.Context, userID string, req model.UpdateSettingsRequest) (*model.Settings, error) {
	names := map[string]string{}
	values := map[string]types.AttributeValue{}
	var sets []string

	if req.ShortcutLogTypes != nil {
		av, err := attributevalue.Marshal(req.ShortcutLogTypes)
		if err != nil {
			return nil, fmt.Errorf("marshal shortcutLogTypes: %w", err)
		}
		names["#slt"] = "shortcutLogTypes"
		values[":slt"] = av
		sets = append(sets, "#slt = :slt")
	}
	if req.SortChipOrder != nil {
		av, err := attributevalue.Marshal(req.SortChipOrder)
		if err != nil {
			return nil, fmt.Errorf("marshal sortChipOrder: %w", err)
		}
		names["#sco"] = "sortChipOrder"
		values[":sco"] = av
		sets = append(sets, "#sco = :sco")
	}
	if req.PlantsLayoutMode != "" {
		names["#plm"] = "plantsLayoutMode"
		values[":plm"] = &types.AttributeValueMemberS{Value: req.PlantsLayoutMode}
		sets = append(sets, "#plm = :plm")
	}
	if req.NutrientEntryMode != "" {
		names["#nem"] = "nutrientEntryMode"
		values[":nem"] = &types.AttributeValueMemberS{Value: req.NutrientEntryMode}
		sets = append(sets, "#nem = :nem")
	}

	if len(sets) == 0 {
		return s.Get(ctx, userID)
	}

	out, err := s.ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"userId": &types.AttributeValueMemberS{Value: userID},
		},
		UpdateExpression:          aws.String("SET " + strings.Join(sets, ", ")),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
		ReturnValues:              types.ReturnValueAllNew,
	})
	if err != nil {
		return nil, fmt.Errorf("update settings: %w", err)
	}
	settings := &model.Settings{}
	if err := attributevalue.UnmarshalMap(out.Attributes, settings); err != nil {
		return nil, fmt.Errorf("unmarshal settings: %w", err)
	}
	return settings, nil
}
