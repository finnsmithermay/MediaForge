package store

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/finnsmithermay/mediaforge/pkg/models"
)

// DynamoDBStore implements JobStore using Amazon DynamoDB.
type DynamoDBStore struct {
	client    *dynamodb.Client
	tableName string
}

// DynamoDBConfig holds connection settings for DynamoDB.
type DynamoDBConfig struct {
	TableName string
	Region    string
	Endpoint  string // empty for real AWS, set for LocalStack
}

// NewDynamoDBStore creates a new DynamoDB-backed job store.
func NewDynamoDBStore(ctx context.Context, cfg DynamoDBConfig) (*DynamoDBStore, error) {
	var opts []func(*config.LoadOptions) error
	opts = append(opts, config.WithRegion(cfg.Region))

	if cfg.Endpoint != "" {
		opts = append(opts,
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
		)
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	var clientOpts []func(*dynamodb.Options)
	if cfg.Endpoint != "" {
		clientOpts = append(clientOpts, func(o *dynamodb.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}

	client := dynamodb.NewFromConfig(awsCfg, clientOpts...)

	return &DynamoDBStore{
		client:    client,
		tableName: cfg.TableName,
	}, nil
}

// Create stores a new job in DynamoDB.
func (s *DynamoDBStore) Create(ctx context.Context, job *models.Job) error {
	item, err := attributevalue.MarshalMap(job)
	if err != nil {
		return fmt.Errorf("marshaling job: %w", err)
	}

	input := &dynamodb.PutItemInput{
		TableName:           aws.String(s.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(id)"),
	}

	_, err = s.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("putting item (id=%s): %w", job.ID, err)
	}

	return nil
}

// Get retrieves a job by ID.
func (s *DynamoDBStore) Get(ctx context.Context, id string) (*models.Job, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
	}

	output, err := s.client.GetItem(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("getting item (id=%s): %w", id, err)
	}

	if output.Item == nil {
		return nil, fmt.Errorf("job not found: %s", id)
	}

	var job models.Job
	if err := attributevalue.UnmarshalMap(output.Item, &job); err != nil {
		return nil, fmt.Errorf("unmarshaling item: %w", err)
	}

	return &job, nil
}

// Update replaces a job's data in DynamoDB.
func (s *DynamoDBStore) Update(ctx context.Context, job *models.Job) error {
	job.UpdatedAt = time.Now()

	item, err := attributevalue.MarshalMap(job)
	if err != nil {
		return fmt.Errorf("marshaling job: %w", err)
	}

	input := &dynamodb.PutItemInput{
		TableName:           aws.String(s.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_exists(id)"),
	}

	_, err = s.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("updating item (id=%s): %w", job.ID, err)
	}

	return nil
}

// List retrieves the most recent jobs, up to the given limit.
func (s *DynamoDBStore) List(ctx context.Context, limit int) ([]models.Job, error) {
	input := &dynamodb.ScanInput{
		TableName: aws.String(s.tableName),
		Limit:     aws.Int32(int32(limit)),
	}

	output, err := s.client.Scan(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("scanning jobs table: %w", err)
	}

	jobs := make([]models.Job, 0, len(output.Items))
	for _, item := range output.Items {
		var job models.Job
		if err := attributevalue.UnmarshalMap(item, &job); err != nil {
			return nil, fmt.Errorf("unmarshaling item: %w", err)
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}
