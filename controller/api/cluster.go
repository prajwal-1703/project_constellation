package api

import (
	"net/http"
	"runtime"

	"github.com/constellation/controller/state"
	auth_middleware "github.com/constellation/controller/api/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

// ─── Cluster Endpoints ──────────────────────────────────────────────────────

type ClusterInitRequest struct {
	Name string `json:"name"`
}

func (s *Server) HandleClusterInit(w http.ResponseWriter, r *http.Request) {
	var req ClusterInitRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "cluster name is required")
		return
	}

	// Check if cluster already exists
	existing, err := s.Store.GetCluster()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check existing cluster")
		return
	}
	if existing != nil {
		respondError(w, http.StatusConflict, "cluster already initialized: "+existing.Name)
		return
	}

	// Detect host IP
	host := detectLocalIP()

	cluster, err := s.Store.CreateCluster(req.Name, host, 8080, 9090)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create cluster: "+err.Error())
		return
	}

	// Register this machine as controller node
	controllerNode := &state.Node{
		ID:          "ctrl-" + cluster.ID,
		Hostname:    getHostname(),
		IPAddress:   host,
		Role:        "controller",
		Status:      "online",
		CPUModel:    runtime.GOARCH,
		CPUCores:    runtime.NumCPU(),
		CPUThreads:  runtime.NumCPU(),
		AgentPort:   9091,
		ClusterID:   cluster.ID,
	}
	if err := s.Store.RegisterNode(controllerNode); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to register controller node: "+err.Error())
		return
	}

	// Create default admin user
	adminUser := &state.User{
		Username:     "admin",
		PasswordHash: hashPassword("admin"), // default password
		Role:         "admin",
	}
	_ = s.Store.CreateUser(adminUser)

	s.Store.AddAuditEntry("", "cluster_init", "cluster", cluster.ID, "Cluster '"+req.Name+"' initialized")

	s.WSHub.BroadcastEvent("cluster_init", map[string]interface{}{
		"cluster": cluster,
	})

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"cluster":    cluster,
		"controller": controllerNode,
		"message":    "Cluster '" + req.Name + "' initialized successfully",
	})
}

func (s *Server) HandleClusterStatus(w http.ResponseWriter, r *http.Request) {
	cluster, err := s.Store.GetCluster()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get cluster")
		return
	}
	if cluster == nil {
		respondError(w, http.StatusNotFound, "no cluster initialized")
		return
	}

	nodes, err := s.Store.ListNodes()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list nodes")
		return
	}

	pool, err := s.Store.GetComputationPool()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get pool")
		return
	}

	tasks, _, err := s.Store.ListTasks("", "", 0, 0)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}

	// Count task statuses
	taskStats := map[string]int{
		"running":   0,
		"queued":    0,
		"completed": 0,
		"failed":    0,
	}
	for _, t := range tasks {
		taskStats[t.Status]++
	}

	// Determine health
	onlineCount := 0
	for _, n := range nodes {
		if n.Status == "online" {
			onlineCount++
		}
	}
	health := "healthy"
	if onlineCount == 0 {
		health = "critical"
	} else if onlineCount < len(nodes) {
		health = "degraded"
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"cluster":     cluster,
		"health":      health,
		"nodes":       nodes,
		"pool":        pool,
		"task_stats":  taskStats,
		"total_tasks": len(tasks),
	})
}

func (s *Server) HandleClusterPool(w http.ResponseWriter, r *http.Request) {
	pool, err := s.Store.GetComputationPool()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get pool")
		return
	}
	respondJSON(w, http.StatusOK, pool)
}

func (s *Server) HandleClusterDestroy(w http.ResponseWriter, r *http.Request) {
	cluster, err := s.Store.GetCluster()
	if err != nil || cluster == nil {
		respondError(w, http.StatusNotFound, "no cluster to destroy")
		return
	}

	// Remove all nodes first
	nodes, _ := s.Store.ListNodes()
	for _, n := range nodes {
		s.Store.DeleteNode(n.ID)
	}

	if err := s.Store.DeleteCluster(); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to destroy cluster")
		return
	}

	s.Store.AddAuditEntry("", "cluster_destroy", "cluster", cluster.ID, "Cluster destroyed")
	s.WSHub.BroadcastEvent("cluster_destroy", nil)

	respondJSON(w, http.StatusOK, map[string]string{"message": "cluster destroyed"})
}

// ─── Node Endpoints ─────────────────────────────────────────────────────────

type NodeJoinRequest struct {
	Hostname   string `json:"hostname"`
	IPAddress  string `json:"ip_address"`
	MACAddress string `json:"mac_address"`
	AgentPort  int    `json:"agent_port"`
	JoinToken  string `json:"join_token"`
	CPUModel   string `json:"cpu_model"`
	CPUCores   int    `json:"cpu_cores"`
	CPUThreads int    `json:"cpu_threads"`
	CPUFreqMHz float64 `json:"cpu_freq_mhz"`
	MemoryTotal int64  `json:"memory_total"`
	DiskTotal  int64  `json:"disk_total"`
	OSName     string `json:"os_name"`
	KernelVer  string `json:"kernel_version"`
	Arch       string `json:"arch"`
}

func (s *Server) HandleNodeJoin(w http.ResponseWriter, r *http.Request) {
	var req NodeJoinRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	cluster, err := s.Store.GetCluster()
	if err != nil || cluster == nil {
		respondError(w, http.StatusNotFound, "no cluster initialized")
		return
	}

	// Validate join token
	if req.JoinToken != cluster.JoinToken {
		respondError(w, http.StatusUnauthorized, "invalid join token")
		return
	}

	if req.Hostname == "" || req.IPAddress == "" {
		respondError(w, http.StatusBadRequest, "hostname and ip_address are required")
		return
	}

	if req.AgentPort == 0 {
		req.AgentPort = 9091
	}

	node := &state.Node{
		ID:            generateNodeID(),
		Hostname:      req.Hostname,
		IPAddress:     req.IPAddress,
		MACAddress:    req.MACAddress,
		AgentPort:     req.AgentPort,
		Role:          "worker",
		Status:        "online",
		CPUModel:      req.CPUModel,
		CPUCores:      req.CPUCores,
		CPUThreads:    req.CPUThreads,
		CPUFreqMHz:    req.CPUFreqMHz,
		MemoryTotal:   req.MemoryTotal,
		DiskTotal:     req.DiskTotal,
		OSName:        req.OSName,
		KernelVersion: req.KernelVer,
		Arch:          req.Arch,
		ClusterID:     cluster.ID,
	}

	if err := s.Store.RegisterNode(node); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to register node: "+err.Error())
		return
	}

	s.Store.AddAuditEntry("", "node_join", "node", node.ID, "Node "+req.Hostname+" joined")

	s.WSHub.BroadcastEvent("node_joined", map[string]interface{}{
		"node": node,
	})

	// Generate JWT for the node
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, auth_middleware.Claims{
		Username: node.ID,
		Role:     "node",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(365 * 24 * time.Hour)),
		},
	})
	tokenString, _ := token.SignedString(auth_middleware.JWTSecret)

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"node_id":      node.ID,
		"cluster_id":   cluster.ID,
		"cluster_name": cluster.Name,
		"token":        tokenString,
		"message":      "Successfully joined cluster '" + cluster.Name + "'",
	})
}

func (s *Server) HandleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.Store.ListNodes()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list nodes")
		return
	}
	if nodes == nil {
		nodes = []*state.Node{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"nodes": nodes,
		"total": len(nodes),
	})
}

func (s *Server) HandleGetNode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	node, err := s.Store.GetNode(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "node not found")
		return
	}
	respondJSON(w, http.StatusOK, node)
}

func (s *Server) HandleDeleteNode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	node, err := s.Store.GetNode(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "node not found")
		return
	}

	if node.Role == "controller" {
		respondError(w, http.StatusBadRequest, "cannot remove controller node")
		return
	}

	if err := s.Store.DeleteNode(id); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to remove node")
		return
	}

	s.Store.AddAuditEntry("", "node_remove", "node", id, "Node "+node.Hostname+" removed")
	s.WSHub.BroadcastEvent("node_removed", map[string]interface{}{"node_id": id})

	respondJSON(w, http.StatusOK, map[string]string{"message": "node removed"})
}

type UpdateNodeStatusRequest struct {
	Status  string                 `json:"status"` // online, offline, draining, maintenance
	Metrics map[string]interface{} `json:"metrics,omitempty"`
}

func (s *Server) HandleUpdateNodeStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateNodeStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request")
		return
	}

	validStatuses := map[string]bool{
		"online": true, "offline": true, "draining": true, "maintenance": true,
	}
	if !validStatuses[req.Status] {
		respondError(w, http.StatusBadRequest, "invalid status, must be: online, offline, draining, maintenance")
		return
	}

	if req.Status == "online" {
		if err := s.Store.UpdateNodeHeartbeat(id); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to update heartbeat")
			return
		}
	} else {
		if err := s.Store.UpdateNodeStatus(id, req.Status); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to update node status")
			return
		}
	}

	if req.Metrics != nil {
		metrics := &state.NodeLiveMetrics{}
		if v, ok := req.Metrics["cpu_usage"].(float64); ok { metrics.CPUUsage = v }
		if v, ok := req.Metrics["memory_usage"].(float64); ok { metrics.MemoryUsage = v }
		if v, ok := req.Metrics["disk_usage"].(float64); ok { metrics.DiskUsage = v }
		if v, ok := req.Metrics["load_avg_1"].(float64); ok { metrics.LoadAvg1 = v }
		if v, ok := req.Metrics["load_avg_5"].(float64); ok { metrics.LoadAvg5 = v }
		if v, ok := req.Metrics["load_avg_15"].(float64); ok { metrics.LoadAvg15 = v }
		if v, ok := req.Metrics["running_tasks"].(float64); ok { metrics.RunningTasks = int(v) }
		if v, ok := req.Metrics["uptime_seconds"].(float64); ok { metrics.UptimeSeconds = int64(v) }
		if v, ok := req.Metrics["cpu_temperature"].(float64); ok { metrics.CPUTemperature = v }
		
		s.Store.UpdateNodeLiveMetrics(id, metrics)
	}

	s.WSHub.BroadcastEvent("node_status_changed", map[string]interface{}{
		"node_id": id,
		"status":  req.Status,
	})

	respondJSON(w, http.StatusOK, map[string]string{"message": "status updated to " + req.Status})
}

func (s *Server) HandleReserveNode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// For MVP, reservation is handled by setting status to maintenance
	if err := s.Store.UpdateNodeStatus(id, "maintenance"); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to reserve node")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "node reserved"})
}

func (s *Server) HandleUnreserveNode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.Store.UpdateNodeStatus(id, "online"); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to unreserve node")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "node unreserved"})
}
