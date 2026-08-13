package worker

import (
	"context"
	"log"
	"sync"

	"github.com/finnsmithermay/mediaforge/internal/api/queue"
	"github.com/finnsmithermay/mediaforge/internal/api/store"
	"github.com/finnsmithermay/mediaforge/internal/api/ws"
	"github.com/finnsmithermay/mediaforge/pkg/models"
)

// Processor defines what a media processor must do.
type Processor interface {
	Process(ctx context.Context, job *models.Job) (*models.ProcessResult, error)
	Supports(mediaType models.MediaType) bool
}

// Pool manages a set of worker goroutines that process jobs from a queue.
type Pool struct {
	consumer   queue.Consumer
	store      store.JobStore
	processors []Processor
	hub        *ws.Hub
	workers    int
	messages   chan queue.Message
	wg         sync.WaitGroup
}

// PoolConfig holds worker pool settings.
type PoolConfig struct {
	Workers    int // number of concurrent workers
	BufferSize int // channel buffer size
}

// NewPool creates a new worker pool.
func NewPool(consumer queue.Consumer, jobStore store.JobStore, processors []Processor, hub *ws.Hub, cfg PoolConfig) *Pool {
	if cfg.Workers <= 0 {
		cfg.Workers = 3
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = cfg.Workers * 2
	}

	return &Pool{
		consumer:   consumer,
		store:      jobStore,
		processors: processors,
		hub:        hub,
		workers:    cfg.Workers,
		messages:   make(chan queue.Message, cfg.BufferSize),
	}
}

// Start launches the poller and worker goroutines.
// It blocks until ctx is cancelled.
func (p *Pool) Start(ctx context.Context) {
	// Start workers
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}

	// Start poller
	p.wg.Add(1)
	go p.poller(ctx)

	log.Printf("Worker pool started: %d workers", p.workers)

	// Wait for all goroutines to finish
	p.wg.Wait()
	log.Println("Worker pool stopped")
}

func (p *Pool) poller(ctx context.Context) {
	defer p.wg.Done()
	defer close(p.messages) // closing the channel signals workers to stop

	for {
		select {
		case <-ctx.Done():
			log.Println("Poller: context cancelled, stopping")
			return
		default:
		}

		msgs, err := p.consumer.Receive(ctx, p.workers)
		if err != nil {
			if ctx.Err() != nil {
				return // context was cancelled during receive
			}
			log.Printf("Poller: error receiving messages: %v", err)
			continue
		}

		for _, msg := range msgs {
			select {
			case p.messages <- msg:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (p *Pool) worker(ctx context.Context, id int) {
	defer p.wg.Done()
	log.Printf("Worker %d: started", id)

	for msg := range p.messages {
		p.processMessage(ctx, id, msg)
	}

	log.Printf("Worker %d: stopped", id)
}

func (p *Pool) processMessage(ctx context.Context, workerID int, msg queue.Message) {
	log.Printf("Worker %d: processing job %s", workerID, msg.JobID)

	job, err := p.store.Get(ctx, msg.JobID)
	if err != nil {
		log.Printf("Worker %d: failed to get job %s: %v", workerID, msg.JobID, err)
		return
	}

	job.Status = models.StatusProcessing
	if err := p.store.Update(ctx, job); err != nil {
		log.Printf("Worker %d: failed to update job status: %v", workerID, err)
		return
	}

	processor := p.findProcessor(job.MediaType)
	if processor == nil {
		log.Printf("Worker %d: no processor for media type %s", workerID, job.MediaType)
		job.Status = models.StatusFailed
		job.Error = "unsupported media type"
		p.store.Update(ctx, job)
		p.hub.BroadcastJobComplete(job.ID, string(job.Status), "", job.Error)
		return
	}

	result, err := processor.Process(ctx, job)
	if err != nil {
		log.Printf("Worker %d: processing failed for job %s: %v", workerID, msg.JobID, err)
		job.Status = models.StatusFailed
		job.Error = err.Error()
		p.store.Update(ctx, job)
		p.hub.BroadcastJobComplete(job.ID, string(job.Status), "", job.Error)
		return
	}

	job.Status = models.StatusComplete
	job.ThumbnailURL = result.ThumbnailURL
	job.Metadata = result.Metadata
	if err := p.store.Update(ctx, job); err != nil {
		log.Printf("Worker %d: failed to save results: %v", workerID, err)
		return
	}

	// Notify connected clients
	p.hub.BroadcastJobComplete(job.ID, string(job.Status), job.ThumbnailURL, "")

	if err := p.consumer.Delete(ctx, msg.ReceiptHandle); err != nil {
		log.Printf("Worker %d: failed to delete SQS message: %v", workerID, err)
	}

	log.Printf("Worker %d: job %s completed successfully", workerID, msg.JobID)
}

func (p *Pool) findProcessor(mediaType models.MediaType) Processor {
	for _, proc := range p.processors {
		if proc.Supports(mediaType) {
			return proc
		}
	}
	return nil
}
