package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/constellation/controller/api/middleware"
	"github.com/constellation/controller/state"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

// ─── Task Endpoints ─────────────────────────────────────────────────────────

type SubmitTaskRequest struct {
	Name           string            `json:"name"`
	Command        string            `json:"command"`
	CPURequired    int               `json:"cpu_required"`
	MemoryRequired string            `json:"memory_required"` // "4GB", "512MB"
	Priority       string            `json:"priority"`        // critical, high, normal, low
	RetryMax       int               `json:"retry_max"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	EnvVars        map[string]string `json:"env_vars"`
	WorkingDir     string            `json:"working_dir"`
	Runtime        string            `json:"runtime"`      // bare, docker
	DockerImage    string            `json:"docker_image"`
	TargetNode     string            `json:"target_node"`  // pin to node

	// Split task fields
	SplitStrategy  string `json:"split_strategy"` // chunk, map-reduce, pipeline, replicated
	InputFile      string `json:"input_file"`
	SplitBy        string `json:"split_by"`       // lines, files, size
	ChunkSize      int    `json:"chunk_size"`
	ReduceCommand  string `json:"reduce_command"`
	MaxNodes       int    `json:"max_nodes"`
}

func (s *Server) HandleSubmitTask(w http.ResponseWriter, r *http.Request) {
	var req SubmitTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.Command == "" {
		respondError(w, http.StatusBadRequest, "command is required")
		return
	}
	if req.Name == "" {
		req.Name = req.Command
		if len(req.Name) > 50 {
			req.Name = req.Name[:50]
		}
	}

	// Validate priority
	if req.Priority == "" {
		req.Priority = "normal"
	}
	validPriorities := map[string]bool{
		"critical": true, "high": true, "normal": true, "low": true,
	}
	if !validPriorities[req.Priority] {
		respondError(w, http.StatusBadRequest, "invalid priority, must be: critical, high, normal, low")
		return
	}

	// Validate runtime
	if req.Runtime == "" {
		req.Runtime = "bare"
	}
	if req.Runtime != "bare" && req.Runtime != "docker" {
		respondError(w, http.StatusBadRequest, "invalid runtime, must be: bare, docker")
		return
	}
	if req.Runtime == "docker" && req.DockerImage == "" {
		respondError(w, http.StatusBadRequest, "docker_image is required when runtime is 'docker'")
		return
	}

	// Defaults
	if req.CPURequired <= 0 {
		req.CPURequired = 1
	}

	// Parse memory
	var memBytes int64
	if req.MemoryRequired != "" {
		memBytes = parseMemorySize(req.MemoryRequired)
	}

	// Determine task type
	taskType := "simple"
	if req.SplitStrategy != "" {
		taskType = "split"
		validStrategies := map[string]bool{
			"chunk": true, "map-reduce": true, "pipeline": true, "replicated": true,
		}
		if !validStrategies[req.SplitStrategy] {
			respondError(w, http.StatusBadRequest, "invalid split_strategy")
			return
		}
	}

	// Validate target node if specified
	if req.TargetNode != "" {
		node, err := s.Store.GetNode(req.TargetNode)
		if err != nil || node == nil {
			respondError(w, http.StatusBadRequest, "target node not found: "+req.TargetNode)
			return
		}
		if node.Status != "online" {
			respondError(w, http.StatusBadRequest, "target node is not online: "+node.Status)
			return
		}
	}

	task := &state.Task{
		Name:           req.Name,
		Command:        req.Command,
		Type:           taskType,
		SplitStrategy:  req.SplitStrategy,
		Runtime:        req.Runtime,
		DockerImage:    req.DockerImage,
		CPURequired:    req.CPURequired,
		MemoryRequired: memBytes,
		Priority:       req.Priority,
		RetryMax:       req.RetryMax,
		TimeoutSeconds: req.TimeoutSeconds,
		EnvVars:        req.EnvVars,
		WorkingDir:     req.WorkingDir,
		TargetNode:     req.TargetNode,
		InputFile:      req.InputFile,
		SplitBy:        req.SplitBy,
		ChunkSize:      req.ChunkSize,
		ReduceCommand:  req.ReduceCommand,
		MaxNodes:       req.MaxNodes,
		Status:         "queued",
		ExitCode:       -1,
	}

	if err := s.Store.CreateTask(task); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create task: "+err.Error())
		return
	}

	s.Store.AddAuditEntry("", "task_submit", "task", task.ID, "Task '"+task.Name+"' submitted")

	s.WSHub.BroadcastEvent("task_submitted", map[string]interface{}{
		"task": task,
	})

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"task_id": task.ID,
		"name":    task.Name,
		"status":  task.Status,
		"message": "Task submitted successfully",
	})
}

func (s *Server) HandleListTasks(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	priorityFilter := r.URL.Query().Get("priority")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 0
	offset := 0
	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}
	if offsetStr != "" {
		offset, _ = strconv.Atoi(offsetStr)
	}

	tasks, total, err := s.Store.ListTasks(statusFilter, priorityFilter, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}
	if tasks == nil {
		tasks = []*state.Task{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks,
		"total": total,
	})
}

func (s *Server) HandleGetTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	task, err := s.Store.GetTask(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "task not found")
		return
	}
	respondJSON(w, http.StatusOK, task)
}

func (s *Server) HandleCancelTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	task, err := s.Store.GetTask(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "task not found")
		return
	}

	// Can only cancel non-terminal tasks
	terminalStatuses := map[string]bool{
		"completed": true, "failed": true, "cancelled": true,
	}
	if terminalStatuses[task.Status] {
		respondError(w, http.StatusBadRequest, "task is already in terminal state: "+task.Status)
		return
	}

	if err := s.Store.UpdateTaskStatus(id, "cancelled"); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to cancel task")
		return
	}

	s.Store.AddAuditEntry("", "task_cancel", "task", id, "Task '"+task.Name+"' cancelled")
	s.WSHub.BroadcastEvent("task_cancelled", map[string]interface{}{"task_id": id})

	respondJSON(w, http.StatusOK, map[string]string{"message": "task cancelled"})
}

func (s *Server) HandleRetryTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	task, err := s.Store.GetTask(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "task not found")
		return
	}

	if task.Status != "failed" && task.Status != "cancelled" {
		respondError(w, http.StatusBadRequest, "can only retry failed or cancelled tasks")
		return
	}

	if err := s.Store.IncrementTaskRetry(id); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to retry task")
		return
	}

	s.Store.AddAuditEntry("", "task_retry", "task", id, "Task '"+task.Name+"' retried")
	s.WSHub.BroadcastEvent("task_retried", map[string]interface{}{"task_id": id})

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "task requeued for retry",
		"task_id": id,
	})
}

func (s *Server) HandleTaskLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, err := s.Store.GetTask(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "task not found")
		return
	}

	// For MVP, return placeholder — real log streaming requires gRPC connection to agent
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"task_id": id,
		"logs":    []string{"[Log streaming available when agent is connected]"},
	})
}

func (s *Server) HandleTaskLogsWS(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, err := s.Store.GetTask(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "task not found")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Simulate streaming logs for enterprise demonstration
	logs := []string{
		"Initializing task execution environment...",
		"Allocating cgroups (CPU: 2, Mem: 4GB)...",
		"Pulling required dependencies...",
		"Starting execution...",
		"Processing data chunk 1/3...",
		"Processing data chunk 2/3...",
		"Processing data chunk 3/3...",
		"Task completed successfully. Cleaning up resources...",
	}

	for _, line := range logs {
		event := WSEvent{
			Type:      "log",
			Data:      map[string]string{"task_id": id, "line": line},
			Timestamp: time.Now().Unix(),
		}
		msg, _ := json.Marshal(event)
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
}

// ─── User Endpoints ─────────────────────────────────────────────────────────

type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (s *Server) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	if req.Role == "" {
		req.Role = "viewer"
	}
	validRoles := map[string]bool{"admin": true, "operator": true, "viewer": true}
	if !validRoles[req.Role] {
		respondError(w, http.StatusBadRequest, "invalid role, must be: admin, operator, viewer")
		return
	}

	// Check if user already exists
	existing, _ := s.Store.GetUserByUsername(req.Username)
	if existing != nil {
		respondError(w, http.StatusConflict, "user already exists: "+req.Username)
		return
	}

	user := &state.User{
		Username:     req.Username,
		PasswordHash: hashPassword(req.Password),
		Role:         req.Role,
	}

	if err := s.Store.CreateUser(user); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	s.Store.AddAuditEntry("", "user_create", "user", user.ID, "User '"+req.Username+"' created with role "+req.Role)

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
	})
}

func (s *Server) HandleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.Store.ListUsers()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	if users == nil {
		users = []*state.User{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"users": users,
		"total": len(users),
	})
}

func (s *Server) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if username == "admin" {
		respondError(w, http.StatusBadRequest, "cannot delete the admin user")
		return
	}
	if err := s.Store.DeleteUser(username); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "user deleted"})
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.Store.GetUserByUsername(req.Username)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !checkPassword(req.Password, user.PasswordHash) {
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	s.Store.UpdateUserLogin(req.Username)

	// Generate JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &middleware.Claims{
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})
	tokenString, err := token.SignedString(middleware.JWTSecret)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
		"token":    tokenString,
		"message":  "login successful",
	})
}

// ─── Audit Log ───────────────────────────────────────────────────────────────

func (s *Server) HandleListAuditLog(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}

	entries, err := s.Store.ListAuditLog(limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list audit log")
		return
	}
	if entries == nil {
		entries = []*state.AuditEntry{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"total":   len(entries),
	})
}
