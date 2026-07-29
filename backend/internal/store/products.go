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

type ProductStore struct {
	ddb       *dynamodb.Client
	tableName string
	indexName string
}

func NewProductStore(ddb *dynamodb.Client, tableName string) *ProductStore {
	return &ProductStore{ddb: ddb, tableName: tableName, indexName: "user-index"}
}

func (s *ProductStore) List(ctx context.Context, userID string) ([]model.Product, error) {
	out, err := s.ddb.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		IndexName:              aws.String(s.indexName),
		KeyConditionExpression: aws.String("userId = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: userID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	products := make([]model.Product, 0, len(out.Items))
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &products); err != nil {
		return nil, fmt.Errorf("unmarshal products: %w", err)
	}
	return products, nil
}

func (s *ProductStore) Get(ctx context.Context, productID string) (*model.Product, error) {
	out, err := s.ddb.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"productId": &types.AttributeValueMemberS{Value: productID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}
	if out.Item == nil {
		return nil, nil
	}
	product := &model.Product{}
	if err := attributevalue.UnmarshalMap(out.Item, product); err != nil {
		return nil, fmt.Errorf("unmarshal product: %w", err)
	}
	return product, nil
}

func (s *ProductStore) Create(ctx context.Context, userID string, req model.CreateProductRequest) (*model.Product, error) {
	now := time.Now().UTC()
	product := model.Product{
		ProductID:     ulid.Make().String(),
		UserID:        userID,
		Name:          req.Name,
		Brand:         req.Brand,
		Form:          req.Form,
		NPK:           req.NPK,
		ReferenceDose: req.ReferenceDose,
		StockQty:      req.StockQty,
		StockUnit:     req.StockUnit,
		Notes:         req.Notes,
		CreatedAt:     now.Format(time.RFC3339),
	}
	item, err := attributevalue.MarshalMap(product)
	if err != nil {
		return nil, fmt.Errorf("marshal product: %w", err)
	}
	if _, err := s.ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	}); err != nil {
		return nil, fmt.Errorf("create product: %w", err)
	}
	return &product, nil
}

func (s *ProductStore) Update(ctx context.Context, productID string, req model.CreateProductRequest) (*model.Product, error) {
	existing, err := s.Get(ctx, productID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("product not found: %s", productID)
	}
	existing.Name = req.Name
	existing.Brand = req.Brand
	existing.Form = req.Form
	existing.NPK = req.NPK
	existing.ReferenceDose = req.ReferenceDose
	existing.StockQty = req.StockQty
	existing.StockUnit = req.StockUnit
	existing.Notes = req.Notes
	item, err := attributevalue.MarshalMap(existing)
	if err != nil {
		return nil, fmt.Errorf("marshal product: %w", err)
	}
	if _, err := s.ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	}); err != nil {
		return nil, fmt.Errorf("update product: %w", err)
	}
	return existing, nil
}

func (s *ProductStore) Delete(ctx context.Context, productID string) error {
	if _, err := s.ddb.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"productId": &types.AttributeValueMemberS{Value: productID},
		},
	}); err != nil {
		return fmt.Errorf("delete product: %w", err)
	}
	return nil
}
