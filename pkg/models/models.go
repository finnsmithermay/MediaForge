package models

import "time"

// MediaType represents the type of uploaded media.
type MediaType string

const (
	MediaTypeVideo MediaType = "video"
	MediaTypeImage MediaType = "image"
)

// JobStatus represents the current state of a processing job
type JobStatus string

const (
	StatusPending    JobStatus = "pending"
	StatusProcessing JobStatus = "processing"
	StatusComplete   JobStatus = "complete"
	StatusFailed     JobStatus = "failed"
)

// Job represents a media processing job.
type Job struct {
	ID           string    `json:"id" dynamodbav:"id"`
	FileName     string    `json:"file_name" dynamodbav:"file_name"`
	MediaType    MediaType `json:"media_type" dynamodbav:"media_type"`
	Status       JobStatus `json:"status" dynamodbav:"status"`
	OriginalURL  string    `json:"original_url" dynamodbav:"original_url"`
	ThumbnailURL string    `json:"thumbnail_url,omitempty" dynamodbav:"thumbnail_url,omitempty"`
	Metadata     *Metadata `json:"metadata,omitempty" dynamodbav:"metadata,omitempty"`
	Error        string    `json:"error,omitempty" dynamodbav:"error,omitempty"`
	CreatedAt    time.Time `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" dynamodbav:"updated_at"`
}

// Metadata holds extracted information about a media file.
type Metadata struct {
	Width    int     `json:"width" dynamodbav:"width"`
	Height   int     `json:"height" dynamodbav:"height"`
	Duration float64 `json:"duration,omitempty" dynamodbav:"duration,omitempty"`
	Format   string  `json:"format" dynamodbav:"format"`
	Codec    string  `json:"codec,omitempty" dynamodbav:"codec,omitempty"`
	Size     int64   `json:"size_bytes" dynamodbav:"size_bytes"`
}

type ProcessResult struct {
	ThumbnailURL string
	Metadata     *Metadata
}
