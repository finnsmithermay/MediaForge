package api

import (
	"github.com/finnsmithermay/mediaforge/internal/api/queue"
	"github.com/finnsmithermay/mediaforge/internal/api/storage"
	"github.com/finnsmithermay/mediaforge/internal/api/store"
	"github.com/finnsmithermay/mediaforge/internal/api/ws"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	router  chi.Router
	storage storage.Client
	store   store.JobStore
	queue   queue.Producer
	hub     *ws.Hub
}

func NewServer(storageClient storage.Client, jobStore store.JobStore, queueProducer queue.Producer, hub *ws.Hub) *Server {
	s := &Server{
		storage: storageClient,
		store:   jobStore,
		queue:   queueProducer,
		hub:     hub,
	}
	s.router = s.setupRoutes()
	return s
}

func (s *Server) Router() chi.Router {
	return s.router
}

func (s *Server) setupRoutes() chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	r.Get("/health", s.handleHealth)
	r.Get("/ws", s.handleWebSocket)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/upload", s.handleUpload)
		r.Get("/jobs", s.handleListJobs)
		r.Get("/jobs/{id}", s.handleGetJob)
	})

	return r
}
