package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client defines the interface for object storage operations.
type Client interface {
	Upload(ctx context.Context, key string, body io.Reader, contentType string) (string, error)
	GetURL(ctx context.Context, key string) (string, error)
	Download(ctx context.Context, key string) (io.ReadCloser, error)
}

// S3Client implements Client using Amazon S3.

// *Finns Comments*
// It's a container that holds everything needed to interact with S3. Think of it as a toolbox
//
//	you fill it once at startup, then reach into it whenever you need to do something with storage.

type S3Client struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
	urlExpiry time.Duration
}

// Config holds S3 connection settings.
type Config struct {
	Bucket    string
	Region    string
	Endpoint  string // empty for real AWS, set for LocalStack
	URLExpiry time.Duration
}

// NewS3Client creates a new S3 storage client.
func NewS3Client(ctx context.Context, cfg Config) (*S3Client, error) {
	var opts []func(*config.LoadOptions) error
	opts = append(opts, config.WithRegion(cfg.Region))

	// If endpoint is set, we're using LocalStack or another S3-compatible service
	//*Finns Comments*
	//We are using local stack so this is not empty and will use out fake credentials
	if cfg.Endpoint != "" {
		opts = append(opts,
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
		)
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	var s3Opts []func(*s3.Options)
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true // Required for LocalStack
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)
	presigner := s3.NewPresignClient(client)

	expiry := cfg.URLExpiry
	if expiry == 0 {
		expiry = 15 * time.Minute
	}

	return &S3Client{
		client:    client,
		presigner: presigner,
		bucket:    cfg.Bucket,
		urlExpiry: expiry,
	}, nil
}

// Upload stores a file in S3 and returns the object key.
func (c *S3Client) Upload(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	}

	_, err := c.client.PutObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("uploading to S3 (key=%s): %w", key, err)
	}

	return key, nil
}

// GetURL generates a presigned URL for downloading an object.
func (c *S3Client) GetURL(ctx context.Context, key string) (string, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}

	presigned, err := c.presigner.PresignGetObject(ctx, input,
		s3.WithPresignExpires(c.urlExpiry),
	)
	if err != nil {
		return "", fmt.Errorf("generating presigned URL (key=%s): %w", key, err)
	}

	return presigned.URL, nil
}

// Download retrieves a file from S3.
func (c *S3Client) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}

	output, err := c.client.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("downloading from S3 (key=%s): %w", key, err)
	}

	return output.Body, nil
}
