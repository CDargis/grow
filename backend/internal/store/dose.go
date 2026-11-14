package store

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/cdargis/grow/internal/model"
)

// Dose stores are plain persistence; the rules live in internal/dose.

type DoseVersionStore struct {
	ddb       *dynamodb.Client
	tableName string
	indexName string
}

func NewDoseVersionStore(ddb *dynamodb.Client, tableName string) *DoseVersionStore {
	return &DoseVersionStore{ddb: ddb, tableName: tableName, indexName: "user-index"}
}

func (s *DoseVersionStore) List(ctx context.Context, userID string) ([]model.DoseVersion, error) {
	out := []model.DoseVersion{}
	if err := queryAllByUser(ctx, s.ddb, s.tableName, s.indexName, userID, &out); err != nil {
		return nil, fmt.Errorf("list dose versions: %w", err)
	}
	return out, nil
}

func (s *DoseVersionStore) Put(ctx context.Context, v model.DoseVersion) error {
	if err := putItem(ctx, s.ddb, s.tableName, v); err != nil {
		return fmt.Errorf("put dose version: %w", err)
	}
	return nil
}

func (s *DoseVersionStore) Delete(ctx context.Context, versionID string) error {
	if err := deleteItem(ctx, s.ddb, s.tableName, "versionId", versionID); err != nil {
		return fmt.Errorf("delete dose version: %w", err)
	}
	return nil
}

type ShelfStore struct {
	ddb       *dynamodb.Client
	tableName string
	indexName string
}

func NewShelfStore(ddb *dynamodb.Client, tableName string) *ShelfStore {
	return &ShelfStore{ddb: ddb, tableName: tableName, indexName: "user-index"}
}

func (s *ShelfStore) List(ctx context.Context, userID string) ([]model.ShelfItem, error) {
	out := []model.ShelfItem{}
	if err := queryAllByUser(ctx, s.ddb, s.tableName, s.indexName, userID, &out); err != nil {
		return nil, fmt.Errorf("list shelf: %w", err)
	}
	return out, nil
}

func (s *ShelfStore) Put(ctx context.Context, item model.ShelfItem) error {
	if err := putItem(ctx, s.ddb, s.tableName, item); err != nil {
		return fmt.Errorf("put shelf item: %w", err)
	}
	return nil
}

func (s *ShelfStore) Delete(ctx context.Context, shelfItemID string) error {
	if err := deleteItem(ctx, s.ddb, s.tableName, "shelfItemId", shelfItemID); err != nil {
		return fmt.Errorf("delete shelf item: %w", err)
	}
	return nil
}
