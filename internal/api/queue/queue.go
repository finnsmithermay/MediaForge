package queue

import "context"

// Message represents a job message from the queue.
type Message struct {
	JobID         string
	ReceiptHandle string // needed to delete the message after processing
}

// Producer pushes job messages onto the queue.
type Producer interface {
	Send(ctx context.Context, jobID string) error
}

// Consumer polls for job messages from the queue.
type Consumer interface {
	Receive(ctx context.Context, maxMessages int) ([]Message, error)
	Delete(ctx context.Context, receiptHandle string) error
}
