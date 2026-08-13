package store

import (
	"context"

	"github.com/finnsmithermay/mediaforge/pkg/models"
)

// JobStore defines the interface for job persistence operations.
type JobStore interface {
	Create(ctx context.Context, job *models.Job) error
	Get(ctx context.Context, id string) (*models.Job, error)
	Update(ctx context.Context, job *models.Job) error
	List(ctx context.Context, limit int) ([]models.Job, error)
}
