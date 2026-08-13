package processor

import (
	"context"

	"github.com/finnsmithermay/mediaforge/pkg/models"
)

// Processor defines the contract for media processing.
type Processor interface {
	Process(ctx context.Context, job *models.Job) (*models.ProcessResult, error)
	Supports(mediaType models.MediaType) bool
}
