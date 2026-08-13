package queue

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// SQSClient implements both Producer and Consumer using Amazon SQS.
type SQSClient struct {
	client   *sqs.Client
	queueURL string
}

// SQSConfig holds SQS connection settings.
type SQSConfig struct {
	QueueURL string
	Region   string
	Endpoint string
}

// NewSQSClient creates a new SQS client.
func NewSQSClient(ctx context.Context, cfg SQSConfig) (*SQSClient, error) {
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

	var clientOpts []func(*sqs.Options)
	if cfg.Endpoint != "" {
		clientOpts = append(clientOpts, func(o *sqs.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}

	client := sqs.NewFromConfig(awsCfg, clientOpts...)

	return &SQSClient{
		client:   client,
		queueURL: cfg.QueueURL,
	}, nil
}

// Send pushes a job ID onto the queue.
func (c *SQSClient) Send(ctx context.Context, jobID string) error {
	input := &sqs.SendMessageInput{
		QueueUrl:    aws.String(c.queueURL),
		MessageBody: aws.String(jobID),
	}

	_, err := c.client.SendMessage(ctx, input)
	if err != nil {
		return fmt.Errorf("sending message to SQS (jobID=%s): %w", jobID, err)
	}

	return nil
}

// Receive polls SQS for available messages.
func (c *SQSClient) Receive(ctx context.Context, maxMessages int) ([]Message, error) {
	input := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(c.queueURL),
		MaxNumberOfMessages: int32(maxMessages),
		WaitTimeSeconds:     20, // Long polling — waits up to 20s for messages
	}

	output, err := c.client.ReceiveMessage(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("receiving messages from SQS: %w", err)
	}

	messages := make([]Message, 0, len(output.Messages))
	for _, msg := range output.Messages {
		messages = append(messages, Message{
			JobID:         aws.ToString(msg.Body),
			ReceiptHandle: aws.ToString(msg.ReceiptHandle),
		})
	}

	return messages, nil
}

// Delete removes a message from the queue after successful processing.
func (c *SQSClient) Delete(ctx context.Context, receiptHandle string) error {
	input := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	}

	_, err := c.client.DeleteMessage(ctx, input)
	if err != nil {
		return fmt.Errorf("deleting message from SQS: %w", err)
	}

	return nil
}
