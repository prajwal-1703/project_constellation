package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

	logDir := "/var/lib/constellation/logs"
	var filesToRead []string

	if _, err := os.Stat(filepath.Join(logDir, id+".log")); err == nil {
		filesToRead = append(filesToRead, filepath.Join(logDir, id+".log"))
	}

	entries, _ := os.ReadDir(logDir)
	for _, e := range entries {
		if e.Name() == id+".log" {
			continue
		}
		if strings.Contains(e.Name(), id+"-") && strings.HasSuffix(e.Name(), ".log") {
			filesToRead = append(filesToRead, filepath.Join(logDir, e.Name()))
		}
	}

	if len(filesToRead) == 0 {
		event := WSEvent{
			Type:      "log",
			Data:      map[string]string{"task_id": id, "line": "No logs found or task is still running on agent..."},
			Timestamp: time.Now().Unix(),
		}
		msg, _ := json.Marshal(event)
		conn.WriteMessage(websocket.TextMessage, msg)
		
		// Gracefully close
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		time.Sleep(100 * time.Millisecond)
		return
	}

	for _, file := range filesToRead {
		content, err := os.ReadFile(file)
		if err == nil {
			event := WSEvent{
				Type:      "log",
				Data:      map[string]string{"task_id": id, "line": "--- Logs from " + filepath.Base(file) + " ---"},
				Timestamp: time.Now().Unix(),
			}
			msg, _ := json.Marshal(event)
			conn.WriteMessage(websocket.TextMessage, msg)

			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) == "" {
					continue
				}
				event := WSEvent{
					Type:      "log",
					Data:      map[string]string{"task_id": id, "line": line},
					Timestamp: time.Now().Unix(),
				}
				msg, _ := json.Marshal(event)
				conn.WriteMessage(websocket.TextMessage, msg)
				time.Sleep(2 * time.Millisecond) // Prevent buffer overflow
			}
		}
	}

	// Gracefully close at the end
	conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	time.Sleep(100 * time.Millisecond)
}

// ─── Agent Task Execution Endpoints ──────────────────────────────────────────

type ExecutableTask struct {
	ID             string            `json:"id"`
	Command        string            `json:"command"`
	WorkingDir     string            `json:"working_dir"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	EnvVars        map[string]string `json:"env_vars"`
	InputFile      string            `json:"input_file"`
	OutputFile     string            `json:"output_file"`
}

func (s *Server) HandleGetPendingTasks(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "id")
	
	tasks, err := s.Store.GetPendingTasksForNode(nodeID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get pending tasks")
		return
	}

	chunks, err := s.Store.GetPendingChunksForNode(nodeID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get pending chunks")
		return
	}

	executables := make([]ExecutableTask, 0)

	for _, t := range tasks {
		executables = append(executables, ExecutableTask{
			ID:             t.ID,
			Command:        t.Command,
			WorkingDir:     t.WorkingDir,
			TimeoutSeconds: t.TimeoutSeconds,
			EnvVars:        t.EnvVars,
			InputFile:      t.InputFile,
			OutputFile:     "",
		})
		s.Store.UpdateTaskStatus(t.ID, "running")
	}

	for _, c := range chunks {
		// Need parent task for command
		parent, err := s.Store.GetTask(c.ParentTaskID)
		if err != nil {
			continue
		}
		executables = append(executables, ExecutableTask{
			ID:             c.ID,
			Command:        parent.Command,
			WorkingDir:     parent.WorkingDir,
			TimeoutSeconds: parent.TimeoutSeconds,
			EnvVars:        parent.EnvVars,
			InputFile:      c.InputFile,
			OutputFile:     c.OutputFile,
		})
		s.Store.UpdateChunkStatus(c.ID, "running", 0)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": executables,
	})
}

type TaskResultRequest struct {
	ExitCode int      `json:"exit_code"`
	Logs     []string `json:"logs"`
	Status   string   `json:"status"` // completed, failed
}

func (s *Server) HandleTaskResult(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	
	var req TaskResultRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Write logs to disk
	os.MkdirAll("/var/lib/constellation/logs", 0755)
	logPath := filepath.Join("/var/lib/constellation/logs", taskID+".log")
	if len(req.Logs) > 0 {
		file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			for _, line := range req.Logs {
				file.WriteString(line + "\n")
			}
			file.Close()
		} else {
			log.Printf("Failed to write logs for %s: %v", taskID, err)
		}
	}

	// Is it a normal task or a chunk?
	if len(taskID) > 6 && taskID[:6] == "chunk-" || taskID[:7] == "replica" || taskID[:4] == "map-" || taskID[:5] == "pipe-" {
		// It's a chunk
		if err := s.Store.UpdateChunkStatus(taskID, req.Status, req.ExitCode); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to update chunk result")
			return
		}

		s.WSHub.BroadcastEvent("chunk_completed", map[string]interface{}{
			"chunk_id": taskID,
			"status":   req.Status,
		})

		// Aggregate logic: Check if all chunks for this parent task are completed
		// Extract parent ID by string manipulation
		// "chunk-task-2f6...-0" -> parent is "task-2f6..."
		parts := strings.Split(taskID, "-")
		if len(parts) >= 4 && parts[1] == "task" {
			// e.g. ["replica", "task", "2f6...", "0"]
			parentID := parts[1] + "-" + parts[2]
			
			// Check if all chunks completed
			allDone, _ := s.Store.AreAllChunksCompleted(parentID)
			if allDone {
				s.Store.UpdateTaskResult(parentID, 0, "")
				s.WSHub.BroadcastEvent("task_completed", map[string]interface{}{
					"task_id": parentID,
					"status":  "completed",
				})
			}
		}

	} else {
		// Normal task
		errorMsg := ""
		if req.ExitCode != 0 {
			errorMsg = "Task failed with exit code"
		}
		
		if err := s.Store.UpdateTaskResult(taskID, req.ExitCode, errorMsg); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to update task result")
			return
		}

		s.WSHub.BroadcastEvent("task_completed", map[string]interface{}{
			"task_id": taskID,
			"status":  req.Status,
		})
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "result recorded"})
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
