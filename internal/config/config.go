package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration.
type Config struct {
	Port     string
	S3       S3Config
	DynamoDB DynamoDBConfig
	SQS      SQSConfig
	Worker   WorkerConfig
}

type S3Config struct {
	Bucket    string
	Region    string
	Endpoint  string
	URLExpiry time.Duration
}

type DynamoDBConfig struct {
	TableName string
	Region    string
	Endpoint  string
}

type SQSConfig struct {
	QueueURL string
	Region   string
	Endpoint string
}

type WorkerConfig struct {
	Count      int
	BufferSize int
}

// Load reads configuration from environment variables with defaults.
func Load() *Config {
	return &Config{
		Port: envOrDefault("PORT", "8080"),
		S3: S3Config{
			Bucket:    envOrDefault("S3_BUCKET", "mediaforge-uploads"),
			Region:    envOrDefault("AWS_REGION", "us-east-1"),
			Endpoint:  os.Getenv("AWS_ENDPOINT"),
			URLExpiry: 15 * time.Minute,
		},
		DynamoDB: DynamoDBConfig{
			TableName: envOrDefault("DYNAMODB_TABLE", "mediaforge-jobs"),
			Region:    envOrDefault("AWS_REGION", "us-east-1"),
			Endpoint:  os.Getenv("AWS_ENDPOINT"),
		},
		SQS: SQSConfig{
			QueueURL: envOrDefault("SQS_QUEUE_URL", "http://localhost:4566/000000000000/mediaforge-jobs"),
			Region:   envOrDefault("AWS_REGION", "us-east-1"),
			Endpoint: os.Getenv("AWS_ENDPOINT"),
		},
		Worker: WorkerConfig{
			Count:      envOrDefaultInt("WORKER_COUNT", 3),
			BufferSize: envOrDefaultInt("WORKER_BUFFER", 10),
		},
	}
}

func envOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return n
}
