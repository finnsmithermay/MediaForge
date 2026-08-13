package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/finnsmithermay/mediaforge/internal/api"
	"github.com/finnsmithermay/mediaforge/internal/api/processor"
	"github.com/finnsmithermay/mediaforge/internal/api/queue"
	"github.com/finnsmithermay/mediaforge/internal/api/storage"
	"github.com/finnsmithermay/mediaforge/internal/api/store"
	"github.com/finnsmithermay/mediaforge/internal/api/worker"
	"github.com/finnsmithermay/mediaforge/internal/api/ws"
	"github.com/finnsmithermay/mediaforge/internal/config"
)

func main() {
	// Context that cancels on SIGINT or SIGTERM
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg := config.Load()

	// --- Initialise dependencies ---

	storageClient, err := storage.NewS3Client(ctx, storage.Config{
		Bucket:    cfg.S3.Bucket,
		Region:    cfg.S3.Region,
		Endpoint:  cfg.S3.Endpoint,
		URLExpiry: cfg.S3.URLExpiry,
	})
	if err != nil {
		log.Fatalf("failed to create S3 client: %v", err)
	}

	jobStore, err := store.NewDynamoDBStore(ctx, store.DynamoDBConfig{
		TableName: cfg.DynamoDB.TableName,
		Region:    cfg.DynamoDB.Region,
		Endpoint:  cfg.DynamoDB.Endpoint,
	})
	if err != nil {
		log.Fatalf("failed to create DynamoDB store: %v", err)
	}

	sqsClient, err := queue.NewSQSClient(ctx, queue.SQSConfig{
		QueueURL: cfg.SQS.QueueURL,
		Region:   cfg.SQS.Region,
		Endpoint: cfg.SQS.Endpoint,
	})
	if err != nil {
		log.Fatalf("failed to create SQS client: %v", err)
	}

	hub := ws.NewHub()

	videoProcessor := processor.NewVideoProcessor(storageClient)
	imageProcessor := processor.NewImageProcessor(storageClient)

	processors := []worker.Processor{
		videoProcessor,
		imageProcessor,
	}

	// --- Start worker pool ---

	pool := worker.NewPool(sqsClient, jobStore, processors, hub, worker.PoolConfig{
		Workers:    cfg.Worker.Count,
		BufferSize: cfg.Worker.BufferSize,
	})

	workerDone := make(chan struct{})
	go func() {
		pool.Start(ctx)
		close(workerDone)
	}()

	// --- Start HTTP server ---

	server := api.NewServer(storageClient, jobStore, sqsClient, hub)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      server.Router(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Run server in a goroutine so we can handle shutdown
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("MediaForge server starting on port %s", cfg.Port)
		if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	// --- Wait for shutdown signal ---

	select {
	case err := <-serverErr:
		log.Fatalf("server error: %v", err)
	case <-ctx.Done():
		log.Println("Shutdown signal received")
	}

	// --- Graceful shutdown ---

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop accepting new HTTP connections, finish in-flight requests
	log.Println("Shutting down HTTP server...")
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Wait for workers to finish (they'll stop because ctx is cancelled)
	log.Println("Waiting for workers to finish...")
	select {
	case <-workerDone:
		log.Println("Workers stopped")
	case <-shutdownCtx.Done():
		log.Println("Timed out waiting for workers")
	}

	log.Println("MediaForge shutdown complete")
}
