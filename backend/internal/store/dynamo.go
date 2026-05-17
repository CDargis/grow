package store

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Clients struct {
	DDB *dynamodb.Client
	S3  *s3.Client
}

func NewClients(ctx context.Context) (*Clients, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &Clients{
		DDB: dynamodb.NewFromConfig(cfg),
		S3:  s3.NewFromConfig(cfg),
	}, nil
}
