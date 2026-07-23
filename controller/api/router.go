package api

import (
	"encoding/json"
	"log"
	"net/http"

	auth_middleware "github.com/constellation/controller/api/middleware"
	"github.com/constellation/controller/state"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Server holds all API dependencies.
type Server struct {
	Store    *state.Store
	WSHub    *WebSocketHub
	Router   chi.Router
}

// NewServer creates a configured API server.
func NewServer(store *state.Store) *Server {
	s := &Server{
		Store: store,
		WSHub: NewWebSocketHub(),
	}

	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Public Endpoints
		r.Post("/cluster/init", s.HandleClusterInit)
		r.Post("/nodes/join", s.HandleNodeJoin)
		r.Post("/users/login", s.HandleLogin)

		// Protected Endpoints
		r.Group(func(r chi.Router) {
			r.Use(auth_middleware.RequireAuth)

			// Cluster
			r.Get("/cluster/status", s.HandleClusterStatus)
			r.Get("/cluster/pool", s.HandleClusterPool)
			r.Delete("/cluster", s.HandleClusterDestroy)

			// Nodes
			r.Get("/nodes", s.HandleListNodes)
			r.Get("/nodes/{id}", s.HandleGetNode)
			r.Delete("/nodes/{id}", s.HandleDeleteNode)
			r.Put("/nodes/{id}/status", s.HandleUpdateNodeStatus)
			r.Post("/nodes/{id}/reserve", s.HandleReserveNode)
			r.Delete("/nodes/{id}/reserve", s.HandleUnreserveNode)
			r.Get("/nodes/{id}/tasks/pending", s.HandleGetPendingTasks)

			// Tasks
			r.Post("/tasks", s.HandleSubmitTask)
			r.Get("/tasks", s.HandleListTasks)
			r.Get("/tasks/{id}", s.HandleGetTask)
			r.Delete("/tasks/{id}", s.HandleCancelTask)
			r.Post("/tasks/{id}/retry", s.HandleRetryTask)
			r.Post("/tasks/{id}/result", s.HandleTaskResult)
			r.Get("/tasks/{id}/logs", s.HandleTaskLogs)
			r.Get("/tasks/{id}/logs/ws", s.HandleTaskLogsWS)

			// Users
			r.Get("/users", s.HandleListUsers)
			r.Post("/users", s.HandleCreateUser)
			r.Delete("/users/{username}", s.HandleDeleteUser)

			// Audit & Analytics
			r.Get("/audit", s.HandleListAuditLog)
			r.Get("/analytics", s.HandleAnalytics)
		})

	})

	// WebSocket
	r.Get("/ws", s.HandleWebSocket)

	// Serve dashboard static files
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "dashboard/dist/index.html")
	})
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir("dashboard/dist/assets"))))

	// Catch-all for SPA routing (React/Vue router)
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "dashboard/dist/index.html")
	})

	s.Router = r
	return s
}

// Start begins the HTTP server.
func (s *Server) Start(addr, certFile, keyFile string) error {
	// Start WebSocket hub
	go s.WSHub.Run()

	if certFile != "" && keyFile != "" {
		log.Printf("API server starting (HTTPS) on %s", addr)
		return http.ListenAndServeTLS(addr, certFile, keyFile, s.Router)
	}

	log.Printf("API server starting (HTTP) on %s", addr)
	return http.ListenAndServe(addr, s.Router)
}

// ─── JSON Helpers ────────────────────────────────────────────────────────────

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("failed to encode response: %v", err)
		}
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func decodeJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
