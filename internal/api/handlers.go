package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/finnsmithermay/mediaforge/internal/api/ws"
	"github.com/finnsmithermay/mediaforge/pkg/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	health := map[string]string{
		"service": "mediaforge",
		"status":  "healthy",
	}

	// Check DynamoDB connectivity
	_, err := s.store.List(ctx, 1)
	if err != nil {
		health["status"] = "degraded"
		health["dynamodb"] = "unreachable"
	} else {
		health["dynamodb"] = "ok"
	}

	if health["status"] == "healthy" {
		writeJSON(w, http.StatusOK, health)
	} else {
		writeJSON(w, http.StatusServiceUnavailable, health)
	}
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid form data")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing 'file' field in form data")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	mediaType, err := detectMediaType(contentType, header.Filename)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Generate a unique key for S3
	jobID := uuid.New().String()
	ext := filepath.Ext(header.Filename)
	s3Key := fmt.Sprintf("originals/%s%s", jobID, ext)

	// Upload to S3
	_, err = s.storage.Upload(r.Context(), s3Key, file, contentType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store file")
		log.Printf("S3 upload error: %v", err)
		return
	}

	// Generate a presigned URL for the original
	originalURL, err := s.storage.GetURL(r.Context(), s3Key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate file URL")
		log.Printf("presign error: %v", err)
		return
	}

	job := &models.Job{
		ID:          jobID,
		FileName:    header.Filename,
		MediaType:   mediaType,
		Status:      models.StatusPending,
		OriginalURL: originalURL,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Save to DynamoDB
	if err := s.store.Create(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create job record")
		log.Printf("DynamoDB create error: %v", err)
		return
	}

	// Push job to SQS for async processing
	if err := s.queue.Send(r.Context(), job.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to queue job for processing")
		log.Printf("SQS send error: %v", err)
		return
	}

	writeJSON(w, http.StatusAccepted, UploadResponse{
		JobID:   jobID,
		Status:  models.StatusPending,
		Message: "file accepted for processing",
	})
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	if jobID == "" {
		writeError(w, http.StatusBadRequest, "missing job ID")
		return
	}

	job, err := s.store.Get(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.store.List(r.Context(), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list jobs")
		log.Printf("DynamoDB list error: %v", err)
		return
	}

	writeJSON(w, http.StatusOK, JobListResponse{
		Jobs:       jobs,
		TotalCount: len(jobs),
	})
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	client, err := ws.NewClient(w, r)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	s.hub.Register(client)

	go client.WritePump()
	go client.ReadPump(s.hub)
}

// detectMediaType determines if a file is a video or image based on its
// content type and filename extension.
func detectMediaType(contentType, filename string) (models.MediaType, error) {
	// Check content type first
	switch {
	case strings.HasPrefix(contentType, "video/"):
		return models.MediaTypeVideo, nil
	case strings.HasPrefix(contentType, "image/"):
		return models.MediaTypeImage, nil
	}

	// Fall back to file extension
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".mp4"),
		strings.HasSuffix(lower, ".mov"),
		strings.HasSuffix(lower, ".avi"),
		strings.HasSuffix(lower, ".mkv"),
		strings.HasSuffix(lower, ".webm"):
		return models.MediaTypeVideo, nil
	case strings.HasSuffix(lower, ".jpg"),
		strings.HasSuffix(lower, ".jpeg"),
		strings.HasSuffix(lower, ".png"),
		strings.HasSuffix(lower, ".gif"),
		strings.HasSuffix(lower, ".webp"):
		return models.MediaTypeImage, nil
	}

	return "", fmt.Errorf("unsupported media type: %s", contentType)
}
