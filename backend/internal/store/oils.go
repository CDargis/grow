package store

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/cdargis/grow/internal/model"
)

// Oil stores are plain persistence: list/get/put. Validation, uniqueness,
// ID/code assignment and profile calculation live in internal/oils, which is
// shared by the HTTP API and the MCP tools.

type OilLotStore struct {
	ddb       *dynamodb.Client
	tableName string
	indexName string
}

func NewOilLotStore(ddb *dynamodb.Client, tableName string) *OilLotStore {
	return &OilLotStore{ddb: ddb, tableName: tableName, indexName: "user-index"}
}

func (s *OilLotStore) List(ctx context.Context, userID string) ([]model.Lot, error) {
	lots := []model.Lot{}
	if err := queryAllByUser(ctx, s.ddb, s.tableName, s.indexName, userID, &lots); err != nil {
		return nil, fmt.Errorf("list oil lots: %w", err)
	}
	return lots, nil
}

// Get returns nil when the lot doesn't exist.
func (s *OilLotStore) Get(ctx context.Context, lotID string) (*model.Lot, error) {
	lot := &model.Lot{}
	found, err := getItem(ctx, s.ddb, s.tableName, "lotId", lotID, lot)
	if err != nil {
		return nil, fmt.Errorf("get oil lot: %w", err)
	}
	if !found {
		return nil, nil
	}
	return lot, nil
}

func (s *OilLotStore) Put(ctx context.Context, lot model.Lot) error {
	if err := putItem(ctx, s.ddb, s.tableName, lot); err != nil {
		return fmt.Errorf("put oil lot: %w", err)
	}
	return nil
}

type OilBatchStore struct {
	ddb       *dynamodb.Client
	tableName string
	indexName string
}

func NewOilBatchStore(ddb *dynamodb.Client, tableName string) *OilBatchStore {
	return &OilBatchStore{ddb: ddb, tableName: tableName, indexName: "user-index"}
}

func (s *OilBatchStore) List(ctx context.Context, userID string) ([]model.Batch, error) {
	batches := []model.Batch{}
	if err := queryAllByUser(ctx, s.ddb, s.tableName, s.indexName, userID, &batches); err != nil {
		return nil, fmt.Errorf("list oil batches: %w", err)
	}
	return batches, nil
}

// Get returns nil when the batch doesn't exist.
func (s *OilBatchStore) Get(ctx context.Context, batchID string) (*model.Batch, error) {
	batch := &model.Batch{}
	found, err := getItem(ctx, s.ddb, s.tableName, "batchId", batchID, batch)
	if err != nil {
		return nil, fmt.Errorf("get oil batch: %w", err)
	}
	if !found {
		return nil, nil
	}
	return batch, nil
}

func (s *OilBatchStore) Put(ctx context.Context, batch model.Batch) error {
	if err := putItem(ctx, s.ddb, s.tableName, batch); err != nil {
		return fmt.Errorf("put oil batch: %w", err)
	}
	return nil
}

// queryAllByUser pages through a user-index GSI. Profiles are calculated from
// the full set of a user's lots and batches, so a silently truncated first
// page would produce wrong numbers -- hence paging rather than one Query.
func queryAllByUser[T any](ctx context.Context, ddb *dynamodb.Client, table, index, userID string, out *[]T) error {
	p := dynamodb.NewQueryPaginator(ddb, &dynamodb.QueryInput{
		TableName:              aws.String(table),
		IndexName:              aws.String(index),
		KeyConditionExpression: aws.String("userId = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: userID},
		},
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return err
		}
		var items []T
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &items); err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
		*out = append(*out, items...)
	}
	return nil
}

func getItem(ctx context.Context, ddb *dynamodb.Client, table, keyName, keyValue string, out any) (bool, error) {
	res, err := ddb.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			keyName: &types.AttributeValueMemberS{Value: keyValue},
		},
	})
	if err != nil {
		return false, err
	}
	if res.Item == nil {
		return false, nil
	}
	if err := attributevalue.UnmarshalMap(res.Item, out); err != nil {
		return false, fmt.Errorf("unmarshal: %w", err)
	}
	return true, nil
}

func putItem(ctx context.Context, ddb *dynamodb.Client, table string, v any) error {
	item, err := attributevalue.MarshalMap(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	_, err = ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(table),
		Item:      item,
	})
	return err
}
