package api

import "github.com/finnsmithermay/mediaforge/pkg/models"

// UploadResponse is returned after a sucsessful file upload
type UploadResponse struct {
	JobID   string           `json:"job_id"`
	Status  models.JobStatus `json:"status"`
	Message string           `json:"message"`
}

// JobListResponse wraps a list of jobs with pagination info
type JobListResponse struct {
	Jobs       []models.Job `json:"jobs"`
	TotalCount int          `json:"total_count"`
}

// ErrorResponse is the standard error format for all endpoints.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Details string `json:"details, omitempty"`
}
