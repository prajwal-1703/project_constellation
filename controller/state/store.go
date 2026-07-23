package state

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ─── Models ──────────────────────────────────────────────────────────────────

type Cluster struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	ControllerHost string    `json:"controller_host"`
	ControllerPort int       `json:"controller_port"`
	GRPCPort       int       `json:"grpc_port"`
	JoinToken      string    `json:"join_token"`
	CreatedAt      time.Time `json:"created_at"`
}

type Node struct {
	ID              string    `json:"id"`
	Hostname        string    `json:"hostname"`
	IPAddress       string    `json:"ip_address"`
	MACAddress      string    `json:"mac_address"`
	AgentPort       int       `json:"agent_port"`
	Role            string    `json:"role"` // controller, worker
	Status          string    `json:"status"` // online, offline, draining, maintenance, discovered
	SSHUser         string    `json:"ssh_user,omitempty"`
	SSHPort         int       `json:"ssh_port,omitempty"`
	CPUModel        string    `json:"cpu_model"`
	CPUCores        int       `json:"cpu_cores"`
	CPUThreads      int       `json:"cpu_threads"`
	CPUFreqMHz      float64   `json:"cpu_freq_mhz"`
	MemoryTotal     int64     `json:"memory_total"`
	DiskTotal       int64     `json:"disk_total"`
	GPUCount        int       `json:"gpu_count"`
	GPUMemory       int64     `json:"gpu_memory"`
	OSName          string    `json:"os_name"`
	KernelVersion   string    `json:"kernel_version"`
	Arch            string    `json:"arch"`
	ClusterID       string    `json:"cluster_id"`
	JoinedAt        time.Time `json:"joined_at"`
	LastHeartbeat   time.Time `json:"last_heartbeat"`
	LastIPChange    time.Time `json:"last_ip_change,omitempty"`
	Labels          []string  `json:"labels,omitempty"`

	// Live metrics (not persisted, updated by heartbeats)
	CPUUsage        float64   `json:"cpu_usage"`
	MemoryUsage     float64   `json:"memory_usage"`
	DiskUsage       float64   `json:"disk_usage"`
	LoadAvg1        float64   `json:"load_avg_1"`
	LoadAvg5        float64   `json:"load_avg_5"`
	LoadAvg15       float64   `json:"load_avg_15"`
	RunningTasks    int       `json:"running_tasks"`
	UptimeSeconds   int64     `json:"uptime_seconds"`
	CPUTemperature  float64   `json:"cpu_temperature"`
}

type Task struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Status          string    `json:"status"`
	Type            string    `json:"type"` // simple, split
	SplitStrategy   string    `json:"split_strategy,omitempty"`
	Command         string    `json:"command"`
	Runtime         string    `json:"runtime"` // bare, docker
	DockerImage     string    `json:"docker_image,omitempty"`
	CPURequired     int       `json:"cpu_required"`
	MemoryRequired  int64     `json:"memory_required"`
	GPURequired     int       `json:"gpu_required"`
	GPUMemoryRequired int64     `json:"gpu_memory_required"`
	Priority        string    `json:"priority"`
	RetryMax        int       `json:"retry_max"`
	RetryCount      int       `json:"retry_count"`
	TimeoutSeconds  int       `json:"timeout_seconds"`
	SubmittedBy     string    `json:"submitted_by"`
	AssignedNode    string    `json:"assigned_node,omitempty"`
	TargetNode      string    `json:"target_node,omitempty"`
	ExitCode        int       `json:"exit_code"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	StartedAt       time.Time `json:"started_at,omitempty"`
	CompletedAt     time.Time `json:"completed_at,omitempty"`
	EnvVars         map[string]string `json:"env_vars,omitempty"`
	WorkingDir      string    `json:"working_dir,omitempty"`
	InputFile       string    `json:"input_file,omitempty"`
	OutputDir       string    `json:"output_dir,omitempty"`
	ReduceCommand   string    `json:"reduce_command,omitempty"`
	MaxNodes        int       `json:"max_nodes,omitempty"`
	ChunkSize       int       `json:"chunk_size,omitempty"`
	SplitBy         string    `json:"split_by,omitempty"`
	Chunks          []TaskChunk `json:"chunks,omitempty"`
	IsDistributed   bool      `json:"is_distributed"`
	WorldSize       int       `json:"world_size"`
	GangID          string    `json:"gang_id,omitempty"`
}

type TaskChunk struct {
	ID            string    `json:"id"`
	ParentTaskID  string    `json:"parent_task_id"`
	ChunkIndex    int       `json:"chunk_index"`
	AssignedNode  string    `json:"assigned_node,omitempty"`
	Status        string    `json:"status"`
	InputFile     string    `json:"input_file"`
	OutputFile    string    `json:"output_file"`
	ExitCode      int       `json:"exit_code"`
	StartedAt     time.Time `json:"started_at,omitempty"`
	CompletedAt   time.Time `json:"completed_at,omitempty"`
}

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"` // admin, operator, viewer
	CreatedAt    time.Time `json:"created_at"`
	LastLogin    time.Time `json:"last_login,omitempty"`
}

type AuditEntry struct {
	ID         int64     `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	UserID     string    `json:"user_id"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	Details    string    `json:"details"`
}

// ─── Store ───────────────────────────────────────────────────────────────────

type Store struct {
	db    *sql.DB
	mu    sync.RWMutex

	// In-memory live metrics for nodes (updated by heartbeats)
	liveMetrics map[string]*NodeLiveMetrics
	metricsMu   sync.RWMutex
}

type NodeLiveMetrics struct {
	CPUUsage       float64
	MemoryUsage    float64
	DiskUsage      float64
	LoadAvg1       float64
	LoadAvg5       float64
	LoadAvg15      float64
	RunningTasks   int
	UptimeSeconds  int64
	CPUTemperature float64
	LastUpdate     time.Time
}

func NewStore(db *sql.DB) (*Store, error) {
	s := &Store{
		db:          db,
		liveMetrics: make(map[string]*NodeLiveMetrics),
	}

	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS cluster (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		controller_host TEXT NOT NULL,
		controller_port INTEGER NOT NULL DEFAULT 8080,
		grpc_port INTEGER NOT NULL DEFAULT 9090,
		join_token TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY,
		hostname TEXT NOT NULL,
		ip_address TEXT NOT NULL,
		mac_address TEXT DEFAULT '',
		agent_port INTEGER NOT NULL DEFAULT 9091,
		role TEXT NOT NULL CHECK(role IN ('controller','worker')),
		status TEXT NOT NULL DEFAULT 'online' CHECK(status IN ('online','offline','draining','maintenance','discovered')),
		ssh_user TEXT DEFAULT '',
		ssh_port INTEGER DEFAULT 22,
		cpu_model TEXT DEFAULT '',
		cpu_cores INTEGER DEFAULT 0,
		cpu_threads INTEGER DEFAULT 0,
		cpu_freq_mhz REAL DEFAULT 0,
		memory_total INTEGER DEFAULT 0,
		disk_total INTEGER DEFAULT 0,
		gpu_count INTEGER DEFAULT 0,
		gpu_memory INTEGER DEFAULT 0,
		os_name TEXT DEFAULT '',
		kernel_version TEXT DEFAULT '',
		arch TEXT DEFAULT '',
		cluster_id TEXT REFERENCES cluster(id),
		joined_at DATETIME NOT NULL DEFAULT (datetime('now')),
		last_heartbeat DATETIME NOT NULL DEFAULT (datetime('now')),
		last_ip_change DATETIME,
		labels TEXT DEFAULT '[]'
	);

	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'submitted' CHECK(status IN ('submitted','queued','splitting','scheduled','transferring','running','completed','failed','cancelled')),
		type TEXT NOT NULL DEFAULT 'simple' CHECK(type IN ('simple','split')),
		split_strategy TEXT DEFAULT '',
		command TEXT NOT NULL,
		runtime TEXT NOT NULL DEFAULT 'bare' CHECK(runtime IN ('bare','docker')),
		docker_image TEXT DEFAULT '',
		cpu_required INTEGER DEFAULT 1,
		memory_required INTEGER DEFAULT 0,
		gpu_required INTEGER DEFAULT 0,
		gpu_memory_required INTEGER DEFAULT 0,
		priority TEXT NOT NULL DEFAULT 'normal' CHECK(priority IN ('critical','high','normal','low')),
		retry_max INTEGER DEFAULT 0,
		retry_count INTEGER DEFAULT 0,
		timeout_seconds INTEGER DEFAULT 0,
		submitted_by TEXT DEFAULT '',
		assigned_node TEXT DEFAULT '' REFERENCES nodes(id),
		target_node TEXT DEFAULT '',
		exit_code INTEGER DEFAULT -1,
		error_message TEXT DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		started_at DATETIME,
		completed_at DATETIME,
		env_vars TEXT DEFAULT '{}',
		working_dir TEXT DEFAULT '',
		input_file TEXT DEFAULT '',
		output_dir TEXT DEFAULT '',
		reduce_command TEXT DEFAULT '',
		max_nodes INTEGER DEFAULT 0,
		chunk_size INTEGER DEFAULT 0,
		split_by TEXT DEFAULT '',
		is_distributed BOOLEAN DEFAULT 0,
		world_size INTEGER DEFAULT 1,
		gang_id TEXT DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS task_chunks (
		id TEXT PRIMARY KEY,
		parent_task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		chunk_index INTEGER NOT NULL,
		assigned_node TEXT DEFAULT '' REFERENCES nodes(id),
		status TEXT NOT NULL DEFAULT 'queued',
		input_file TEXT DEFAULT '',
		output_file TEXT DEFAULT '',
		exit_code INTEGER DEFAULT -1,
		started_at DATETIME,
		completed_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'viewer' CHECK(role IN ('admin','operator','viewer')),
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		last_login DATETIME
	);

	CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL DEFAULT (datetime('now')),
		user_id TEXT DEFAULT '',
		action TEXT NOT NULL,
		target_type TEXT DEFAULT '',
		target_id TEXT DEFAULT '',
		details TEXT DEFAULT '{}'
	);

	CREATE INDEX IF NOT EXISTS idx_nodes_cluster ON nodes(cluster_id);
	CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes(status);
	CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
	CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority);
	CREATE INDEX IF NOT EXISTS idx_tasks_assigned ON tasks(assigned_node);
	CREATE INDEX IF NOT EXISTS idx_task_chunks_parent ON task_chunks(parent_task_id);
	CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp);
	`

	_, err := s.db.Exec(schema)
	return err
}

// ─── Cluster Operations ─────────────────────────────────────────────────────

func (s *Store) CreateCluster(name, host string, port, grpcPort int) (*Cluster, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cluster := &Cluster{
		ID:             uuid.New().String()[:12],
		Name:           name,
		ControllerHost: host,
		ControllerPort: port,
		GRPCPort:       grpcPort,
		JoinToken:      "cst_" + uuid.New().String()[:16],
		CreatedAt:      time.Now(),
	}

	_, err := s.db.Exec(
		`INSERT INTO cluster (id, name, controller_host, controller_port, grpc_port, join_token, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		cluster.ID, cluster.Name, cluster.ControllerHost, cluster.ControllerPort,
		cluster.GRPCPort, cluster.JoinToken, cluster.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster: %w", err)
	}

	return cluster, nil
}

func (s *Store) GetCluster() (*Cluster, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.db.QueryRow(`SELECT id, name, controller_host, controller_port, grpc_port, join_token, created_at FROM cluster LIMIT 1`)
	c := &Cluster{}
	err := row.Scan(&c.ID, &c.Name, &c.ControllerHost, &c.ControllerPort, &c.GRPCPort, &c.JoinToken, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}
	return c, nil
}

func (s *Store) DeleteCluster() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM cluster`)
	return err
}

// ─── Node Operations ─────────────────────────────────────────────────────────

func (s *Store) RegisterNode(node *Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	labelsJSON, _ := json.Marshal(node.Labels)

	_, err := s.db.Exec(
		`INSERT INTO nodes (id, hostname, ip_address, mac_address, agent_port, role, status,
			cpu_model, cpu_cores, cpu_threads, cpu_freq_mhz, memory_total, disk_total, gpu_count, gpu_memory,
			os_name, kernel_version, arch, cluster_id, joined_at, last_heartbeat, labels)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		node.ID, node.Hostname, node.IPAddress, node.MACAddress, node.AgentPort,
		node.Role, node.Status, node.CPUModel, node.CPUCores, node.CPUThreads,
		node.CPUFreqMHz, node.MemoryTotal, node.DiskTotal, node.GPUCount, node.GPUMemory, node.OSName,
		node.KernelVersion, node.Arch, node.ClusterID, node.JoinedAt,
		node.LastHeartbeat, string(labelsJSON),
	)
	return err
}

func (s *Store) GetNode(id string) (*Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getNodeUnsafe(id)
}

func (s *Store) getNodeUnsafe(id string) (*Node, error) {
	row := s.db.QueryRow(
		`SELECT id, hostname, ip_address, mac_address, agent_port, role, status,
			ssh_user, ssh_port, cpu_model, cpu_cores, cpu_threads, cpu_freq_mhz,
			memory_total, disk_total, gpu_count, gpu_memory, os_name, kernel_version, arch, cluster_id,
			joined_at, last_heartbeat, labels
		 FROM nodes WHERE id = ?`, id,
	)
	return s.scanNode(row)
}

func (s *Store) ListNodes() ([]*Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, hostname, ip_address, mac_address, agent_port, role, status,
			ssh_user, ssh_port, cpu_model, cpu_cores, cpu_threads, cpu_freq_mhz,
			memory_total, disk_total, gpu_count, gpu_memory, os_name, kernel_version, arch, cluster_id,
			joined_at, last_heartbeat, labels
		 FROM nodes ORDER BY joined_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*Node
	for rows.Next() {
		n, err := s.scanNodeRow(rows)
		if err != nil {
			return nil, err
		}
		// Attach live metrics
		s.metricsMu.RLock()
		if m, ok := s.liveMetrics[n.ID]; ok {
			n.CPUUsage = m.CPUUsage
			n.MemoryUsage = m.MemoryUsage
			n.DiskUsage = m.DiskUsage
			n.LoadAvg1 = m.LoadAvg1
			n.LoadAvg5 = m.LoadAvg5
			n.LoadAvg15 = m.LoadAvg15
			n.RunningTasks = m.RunningTasks
			n.UptimeSeconds = m.UptimeSeconds
			n.CPUTemperature = m.CPUTemperature
		}
		s.metricsMu.RUnlock()
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func (s *Store) UpdateNodeStatus(id, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`UPDATE nodes SET status = ? WHERE id = ?`, status, id)
	return err
}

func (s *Store) UpdateNodeHeartbeat(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`UPDATE nodes SET last_heartbeat = datetime('now'), status = 'online' WHERE id = ?`, id)
	return err
}

func (s *Store) UpdateNodeIP(id, newIP string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`UPDATE nodes SET ip_address = ?, last_ip_change = datetime('now') WHERE id = ?`,
		newIP, id,
	)
	return err
}

func (s *Store) UpdateNodeLiveMetrics(nodeID string, metrics *NodeLiveMetrics) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	metrics.LastUpdate = time.Now()
	s.liveMetrics[nodeID] = metrics
}

func (s *Store) DeleteNode(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM nodes WHERE id = ?`, id)
	return err
}

func (s *Store) GetOnlineWorkerNodes() ([]*Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, hostname, ip_address, mac_address, agent_port, role, status,
			ssh_user, ssh_port, cpu_model, cpu_cores, cpu_threads, cpu_freq_mhz,
			memory_total, disk_total, gpu_count, gpu_memory, os_name, kernel_version, arch, cluster_id,
			joined_at, last_heartbeat, labels
		 FROM nodes WHERE status = 'online' AND role = 'worker'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*Node
	for rows.Next() {
		n, err := s.scanNodeRow(rows)
		if err != nil {
			return nil, err
		}
		s.metricsMu.RLock()
		if m, ok := s.liveMetrics[n.ID]; ok {
			n.CPUUsage = m.CPUUsage
			n.MemoryUsage = m.MemoryUsage
			n.DiskUsage = m.DiskUsage
			n.LoadAvg1 = m.LoadAvg1
			n.RunningTasks = m.RunningTasks
		}
		s.metricsMu.RUnlock()
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// ─── Task Operations ─────────────────────────────────────────────────────────

func (s *Store) CreateTask(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task.ID == "" {
		task.ID = "task-" + uuid.New().String()[:8]
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	if task.Status == "" {
		task.Status = "submitted"
	}
	if task.Priority == "" {
		task.Priority = "normal"
	}
	if task.Runtime == "" {
		task.Runtime = "bare"
	}
	if task.Type == "" {
		task.Type = "simple"
	}
	if task.WorldSize == 0 {
		task.WorldSize = 1
	}

	envJSON, _ := json.Marshal(task.EnvVars)

	_, err := s.db.Exec(
		`INSERT INTO tasks (id, name, status, type, split_strategy, command, runtime,
			docker_image, cpu_required, memory_required, gpu_required, gpu_memory_required, priority, retry_max, retry_count,
			timeout_seconds, submitted_by, assigned_node, target_node, exit_code,
			error_message, created_at, env_vars, working_dir, input_file, output_dir,
			reduce_command, max_nodes, chunk_size, split_by, is_distributed, world_size, gang_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.Name, task.Status, task.Type, task.SplitStrategy,
		task.Command, task.Runtime, task.DockerImage, task.CPURequired,
		task.MemoryRequired, task.GPURequired, task.GPUMemoryRequired, task.Priority, task.RetryMax, task.RetryCount,
		task.TimeoutSeconds, task.SubmittedBy, nullIfEmpty(task.AssignedNode), task.TargetNode,
		task.ExitCode, task.ErrorMessage, task.CreatedAt, string(envJSON),
		task.WorkingDir, task.InputFile, task.OutputDir, task.ReduceCommand,
		task.MaxNodes, task.ChunkSize, task.SplitBy, task.IsDistributed, task.WorldSize, task.GangID,
	)
	return err
}

func (s *Store) GetTask(id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getTaskUnsafe(id)
}

func (s *Store) getTaskUnsafe(id string) (*Task, error) {
	row := s.db.QueryRow(
		`SELECT id, name, status, type, split_strategy, command, runtime,
			docker_image, cpu_required, memory_required, gpu_required, gpu_memory_required, priority, retry_max,
			retry_count, timeout_seconds, submitted_by, assigned_node, target_node,
			exit_code, error_message, created_at, started_at, completed_at,
			env_vars, working_dir, input_file, output_dir, reduce_command,
			max_nodes, chunk_size, split_by, is_distributed, world_size, gang_id
		 FROM tasks WHERE id = ?`, id,
	)
	task, err := s.scanTask(row)
	if err != nil {
		return nil, err
	}

	// Load chunks if it's a split task
	if task.Type == "split" {
		chunks, err := s.getTaskChunks(task.ID)
		if err != nil {
			return nil, err
		}
		task.Chunks = chunks
	}

	return task, nil
}

func (s *Store) ListTasks(statusFilter, priorityFilter string, limit, offset int) ([]*Task, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, name, status, type, split_strategy, command, runtime,
		docker_image, cpu_required, memory_required, gpu_required, gpu_memory_required, priority, retry_max,
		retry_count, timeout_seconds, submitted_by, assigned_node, target_node,
		exit_code, error_message, created_at, started_at, completed_at,
		env_vars, working_dir, input_file, output_dir, reduce_command,
		max_nodes, chunk_size, split_by, is_distributed, world_size, gang_id
	 FROM tasks WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM tasks WHERE 1=1`
	var args []interface{}

	if statusFilter != "" {
		query += ` AND status = ?`
		countQuery += ` AND status = ?`
		args = append(args, statusFilter)
	}
	if priorityFilter != "" {
		query += ` AND priority = ?`
		countQuery += ` AND priority = ?`
		args = append(args, priorityFilter)
	}

	// Get total count
	var total int
	err := s.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query += ` ORDER BY created_at DESC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d OFFSET %d`, limit, offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		t, err := s.scanTaskRow(rows)
		if err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, t)
	}
	return tasks, total, rows.Err()
}

func (s *Store) UpdateTaskStatus(id, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	switch status {
	case "running":
		_, err := s.db.Exec(`UPDATE tasks SET status = ?, started_at = ? WHERE id = ?`, status, now, id)
		return err
	case "completed", "failed", "cancelled":
		_, err := s.db.Exec(`UPDATE tasks SET status = ?, completed_at = ? WHERE id = ?`, status, now, id)
		return err
	default:
		_, err := s.db.Exec(`UPDATE tasks SET status = ? WHERE id = ?`, status, id)
		return err
	}
}

func (s *Store) UpdateTaskAssignment(id, nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`UPDATE tasks SET assigned_node = ?, status = 'scheduled' WHERE id = ?`, nodeID, id)
	return err
}

func (s *Store) UpdateTaskResult(id string, exitCode int, errorMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := "completed"
	if exitCode != 0 {
		status = "failed"
	}

	_, err := s.db.Exec(
		`UPDATE tasks SET status = ?, exit_code = ?, error_message = ?, completed_at = datetime('now') WHERE id = ?`,
		status, exitCode, errorMsg, id,
	)
	return err
}

func (s *Store) IncrementTaskRetry(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`UPDATE tasks SET retry_count = retry_count + 1, status = 'queued', assigned_node = '' WHERE id = ?`, id)
	return err
}

func (s *Store) GetQueuedTasks() ([]*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, name, status, type, split_strategy, command, runtime,
			docker_image, cpu_required, memory_required, gpu_required, gpu_memory_required, priority, retry_max,
			retry_count, timeout_seconds, submitted_by, assigned_node, target_node,
			exit_code, error_message, created_at, started_at, completed_at,
			env_vars, working_dir, input_file, output_dir, reduce_command,
			max_nodes, chunk_size, split_by, is_distributed, world_size, gang_id
		 FROM tasks WHERE status IN ('submitted', 'queued')
		 ORDER BY
			CASE priority
				WHEN 'critical' THEN 0
				WHEN 'high' THEN 1
				WHEN 'normal' THEN 2
				WHEN 'low' THEN 3
			END,
			created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		t, err := s.scanTaskRow(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *Store) GetQueuedChunks() ([]*TaskChunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, parent_task_id, chunk_index, assigned_node, status,
			input_file, output_file, exit_code, started_at, completed_at
		 FROM task_chunks WHERE status = 'queued'`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []*TaskChunk
	for rows.Next() {
		var c TaskChunk
		var startedAt, completedAt sql.NullTime
		err := rows.Scan(&c.ID, &c.ParentTaskID, &c.ChunkIndex, &c.AssignedNode,
			&c.Status, &c.InputFile, &c.OutputFile, &c.ExitCode, &startedAt, &completedAt)
		if err != nil {
			return nil, err
		}
		if startedAt.Valid {
			c.StartedAt = startedAt.Time
		}
		if completedAt.Valid {
			c.CompletedAt = completedAt.Time
		}
		chunks = append(chunks, &c)
	}
	return chunks, rows.Err()
}

func (s *Store) GetPendingTasksForNode(nodeID string) ([]*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, name, status, type, split_strategy, command, runtime,
			docker_image, cpu_required, memory_required, gpu_required, gpu_memory_required, priority, retry_max,
			retry_count, timeout_seconds, submitted_by, assigned_node, target_node,
			exit_code, error_message, created_at, started_at, completed_at,
			env_vars, working_dir, input_file, output_dir, reduce_command,
			max_nodes, chunk_size, split_by, is_distributed, world_size, gang_id
		 FROM tasks WHERE assigned_node = ? AND status = 'scheduled'`, nodeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		t, err := s.scanTaskRow(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ─── Task Chunks ─────────────────────────────────────────────────────────────

func (s *Store) CreateTaskChunk(chunk *TaskChunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if chunk.ID == "" {
		chunk.ID = fmt.Sprintf("chunk-%s-%d", chunk.ParentTaskID, chunk.ChunkIndex)
	}
	if chunk.Status == "" {
		chunk.Status = "queued"
	}

	var assignedNode interface{} = chunk.AssignedNode
	if chunk.AssignedNode == "" {
		assignedNode = nil
	}

	_, err := s.db.Exec(
		`INSERT INTO task_chunks (id, parent_task_id, chunk_index, assigned_node, status, input_file, output_file)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		chunk.ID, chunk.ParentTaskID, chunk.ChunkIndex, assignedNode,
		chunk.Status, chunk.InputFile, chunk.OutputFile,
	)
	return err
}

func (s *Store) getTaskChunks(taskID string) ([]TaskChunk, error) {
	rows, err := s.db.Query(
		`SELECT id, parent_task_id, chunk_index, assigned_node, status,
			input_file, output_file, exit_code, started_at, completed_at
		 FROM task_chunks WHERE parent_task_id = ? ORDER BY chunk_index`, taskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []TaskChunk
	for rows.Next() {
		var c TaskChunk
		var startedAt, completedAt sql.NullTime
		err := rows.Scan(&c.ID, &c.ParentTaskID, &c.ChunkIndex, &c.AssignedNode,
			&c.Status, &c.InputFile, &c.OutputFile, &c.ExitCode, &startedAt, &completedAt)
		if err != nil {
			return nil, err
		}
		if startedAt.Valid {
			c.StartedAt = startedAt.Time
		}
		if completedAt.Valid {
			c.CompletedAt = completedAt.Time
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

func (s *Store) GetPendingChunksForNode(nodeID string) ([]TaskChunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT id, parent_task_id, chunk_index, assigned_node, status,
			input_file, output_file, exit_code, started_at, completed_at
		 FROM task_chunks WHERE assigned_node = ? AND status = 'scheduled'`, nodeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []TaskChunk
	for rows.Next() {
		var c TaskChunk
		var startedAt, completedAt sql.NullTime
		err := rows.Scan(&c.ID, &c.ParentTaskID, &c.ChunkIndex, &c.AssignedNode,
			&c.Status, &c.InputFile, &c.OutputFile, &c.ExitCode, &startedAt, &completedAt)
		if err != nil {
			return nil, err
		}
		if startedAt.Valid {
			c.StartedAt = startedAt.Time
		}
		if completedAt.Valid {
			c.CompletedAt = completedAt.Time
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

func (s *Store) AreAllChunksCompleted(parentTaskID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var total, completed int
	err := s.db.QueryRow(`SELECT count(*) FROM task_chunks WHERE parent_task_id = ?`, parentTaskID).Scan(&total)
	if err != nil {
		return false, err
	}

	err = s.db.QueryRow(`SELECT count(*) FROM task_chunks WHERE parent_task_id = ? AND status IN ('completed', 'failed')`, parentTaskID).Scan(&completed)
	if err != nil {
		return false, err
	}

	return total > 0 && total == completed, nil
}

func (s *Store) UpdateChunkStatus(id, status string, exitCode int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	switch status {
	case "running":
		_, err := s.db.Exec(`UPDATE task_chunks SET status = ?, started_at = ? WHERE id = ?`, status, now, id)
		return err
	case "completed", "failed":
		_, err := s.db.Exec(`UPDATE task_chunks SET status = ?, exit_code = ?, completed_at = ? WHERE id = ?`, status, exitCode, now, id)
		return err
	default:
		_, err := s.db.Exec(`UPDATE task_chunks SET status = ? WHERE id = ?`, status, id)
		return err
	}
}

func (s *Store) UpdateChunkAssignment(id, nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`UPDATE task_chunks SET assigned_node = ?, status = 'scheduled' WHERE id = ?`, nodeID, id)
	return err
}

// ─── User Operations ─────────────────────────────────────────────────────────

func (s *Store) CreateUser(user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if user.ID == "" {
		user.ID = uuid.New().String()[:12]
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}

	_, err := s.db.Exec(
		`INSERT INTO users (id, username, password_hash, role, created_at) VALUES (?, ?, ?, ?, ?)`,
		user.ID, user.Username, user.PasswordHash, user.Role, user.CreatedAt,
	)
	return err
}

func (s *Store) GetUserByUsername(username string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.db.QueryRow(`SELECT id, username, password_hash, role, created_at, last_login FROM users WHERE username = ?`, username)
	u := &User{}
	var lastLogin sql.NullTime
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &lastLogin)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastLogin.Valid {
		u.LastLogin = lastLogin.Time
	}
	return u, nil
}

func (s *Store) ListUsers() ([]*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT id, username, password_hash, role, created_at, last_login FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		var lastLogin sql.NullTime
		err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &lastLogin)
		if err != nil {
			return nil, err
		}
		if lastLogin.Valid {
			u.LastLogin = lastLogin.Time
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) DeleteUser(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM users WHERE username = ?`, username)
	return err
}

func (s *Store) UpdateUserLogin(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`UPDATE users SET last_login = datetime('now') WHERE username = ?`, username)
	return err
}

// ─── Audit Log ───────────────────────────────────────────────────────────────

func (s *Store) AddAuditEntry(userID, action, targetType, targetID, details string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`INSERT INTO audit_log (user_id, action, target_type, target_id, details) VALUES (?, ?, ?, ?, ?)`,
		userID, action, targetType, targetID, details,
	)
	return err
}

func (s *Store) ListAuditLog(limit int) ([]*AuditEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, timestamp, user_id, action, target_type, target_id, details
		 FROM audit_log ORDER BY timestamp DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*AuditEntry
	for rows.Next() {
		e := &AuditEntry{}
		err := rows.Scan(&e.ID, &e.Timestamp, &e.UserID, &e.Action, &e.TargetType, &e.TargetID, &e.Details)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ─── Computation Pool ────────────────────────────────────────────────────────

type ComputationPool struct {
	TotalCPU       int     `json:"total_cpu_cores"`
	AvailableCPU   int     `json:"available_cpu_cores"`
	TotalMemory    int64   `json:"total_memory_bytes"`
	AvailableMemory int64  `json:"available_memory_bytes"`
	TotalDisk      int64   `json:"total_disk_bytes"`
	AvailableDisk  int64   `json:"available_disk_bytes"`
	TotalNodes     int     `json:"total_nodes"`
	OnlineNodes    int     `json:"online_nodes"`
}

func (s *Store) GetComputationPool() (*ComputationPool, error) {
	nodes, err := s.ListNodes()
	if err != nil {
		return nil, err
	}

	pool := &ComputationPool{}
	for _, n := range nodes {
		pool.TotalNodes++
		pool.TotalCPU += n.CPUCores
		pool.TotalMemory += n.MemoryTotal
		pool.TotalDisk += n.DiskTotal

		if n.Status == "online" {
			pool.OnlineNodes++
			// Reserve ~15% for system
			pool.AvailableCPU += int(float64(n.CPUCores) * 0.85)
			pool.AvailableMemory += int64(float64(n.MemoryTotal) * (1.0 - n.MemoryUsage/100.0))
			pool.AvailableDisk += int64(float64(n.DiskTotal) * (1.0 - n.DiskUsage/100.0))
		}
	}

	return pool, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

type scannable interface {
	Scan(dest ...interface{}) error
}

func (s *Store) scanNode(row scannable) (*Node, error) {
	n := &Node{}
	var labelsJSON string
	var lastIPChange sql.NullTime
	err := row.Scan(
		&n.ID, &n.Hostname, &n.IPAddress, &n.MACAddress, &n.AgentPort,
		&n.Role, &n.Status, &n.SSHUser, &n.SSHPort, &n.CPUModel,
		&n.CPUCores, &n.CPUThreads, &n.CPUFreqMHz, &n.MemoryTotal,
		&n.DiskTotal, &n.GPUCount, &n.GPUMemory, &n.OSName, &n.KernelVersion, &n.Arch,
		&n.ClusterID, &n.JoinedAt, &n.LastHeartbeat, &labelsJSON,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("node not found")
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(labelsJSON), &n.Labels)
	if lastIPChange.Valid {
		n.LastIPChange = lastIPChange.Time
	}

	// Attach live metrics
	s.metricsMu.RLock()
	if m, ok := s.liveMetrics[n.ID]; ok {
		n.CPUUsage = m.CPUUsage
		n.MemoryUsage = m.MemoryUsage
		n.DiskUsage = m.DiskUsage
		n.LoadAvg1 = m.LoadAvg1
		n.LoadAvg5 = m.LoadAvg5
		n.LoadAvg15 = m.LoadAvg15
		n.RunningTasks = m.RunningTasks
		n.UptimeSeconds = m.UptimeSeconds
		n.CPUTemperature = m.CPUTemperature
	}
	s.metricsMu.RUnlock()

	return n, nil
}

func (s *Store) scanNodeRow(rows *sql.Rows) (*Node, error) {
	return s.scanNode(rows)
}

func (s *Store) scanTask(row scannable) (*Task, error) {
	t := &Task{}
	var startedAt, completedAt sql.NullTime
	var envJSON string
	var assignedNode sql.NullString
	err := row.Scan(
		&t.ID, &t.Name, &t.Status, &t.Type, &t.SplitStrategy, &t.Command,
		&t.Runtime, &t.DockerImage, &t.CPURequired, &t.MemoryRequired, &t.GPURequired, &t.GPUMemoryRequired,
		&t.Priority, &t.RetryMax, &t.RetryCount, &t.TimeoutSeconds,
		&t.SubmittedBy, &assignedNode, &t.TargetNode, &t.ExitCode,
		&t.ErrorMessage, &t.CreatedAt, &startedAt, &completedAt,
		&envJSON, &t.WorkingDir, &t.InputFile, &t.OutputDir,
		&t.ReduceCommand, &t.MaxNodes, &t.ChunkSize, &t.SplitBy, &t.IsDistributed, &t.WorldSize, &t.GangID,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("task not found")
	}
	if err != nil {
		return nil, err
	}
	if assignedNode.Valid {
		t.AssignedNode = assignedNode.String
	}
	if startedAt.Valid {
		t.StartedAt = startedAt.Time
	}
	if completedAt.Valid {
		t.CompletedAt = completedAt.Time
	}
	if envJSON != "" {
		_ = json.Unmarshal([]byte(envJSON), &t.EnvVars)
	}
	return t, nil
}

func (s *Store) scanTaskRow(rows *sql.Rows) (*Task, error) {
	return s.scanTask(rows)
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
