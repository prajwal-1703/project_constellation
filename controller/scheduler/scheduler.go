package scheduler

import (
	"log"
	"sync"
	"time"

	"github.com/constellation/controller/api"
	"github.com/constellation/controller/state"
)

// Scheduler is the core scheduling loop that matches tasks to nodes.
type Scheduler struct {
	store     *state.Store
	wsHub     *api.WebSocketHub
	queue     *PriorityQueue
	ticker    *time.Ticker
	stopCh    chan struct{}
	mu        sync.Mutex
	isRunning bool
}

// NewScheduler creates a new scheduler instance.
func NewScheduler(store *state.Store, wsHub *api.WebSocketHub) *Scheduler {
	return &Scheduler{
		store:  store,
		wsHub:  wsHub,
		queue:  NewPriorityQueue(),
		stopCh: make(chan struct{}),
	}
}

// Start begins the scheduling loop.
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = true
	s.mu.Unlock()

	s.ticker = time.NewTicker(500 * time.Millisecond) // Schedule every 500ms
	log.Println("Scheduler started (interval: 500ms)")

	go s.run()
	go s.heartbeatMonitor()
}

// Stop halts the scheduling loop.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}
	s.isRunning = false
	close(s.stopCh)
	if s.ticker != nil {
		s.ticker.Stop()
	}
	log.Println("Scheduler stopped")
}

func (s *Scheduler) run() {
	for {
		select {
		case <-s.stopCh:
			return
		case <-s.ticker.C:
			s.scheduleCycle()
		}
	}
}

// scheduleCycle runs one iteration of the scheduling loop.
func (s *Scheduler) scheduleCycle() {
	// 1. Get all queued tasks
	tasks, err := s.store.GetQueuedTasks()
	if err != nil {
		log.Printf("scheduler: failed to get queued tasks: %v", err)
		return
	}

	if len(tasks) == 0 {
		return
	}

	// 2. Get available nodes
	nodes, err := s.store.GetOnlineWorkerNodes()
	if err != nil {
		log.Printf("scheduler: failed to get online nodes: %v", err)
		return
	}

	if len(nodes) == 0 {
		return // No nodes available, wait for next cycle
	}

	// 3. For each task, find the best node
	for _, task := range tasks {
		// Skip split tasks — they go through the splitter
		if task.Type == "split" && task.SplitStrategy != "" {
			continue
		}

		if task.IsDistributed && task.WorldSize > 1 {
			gangNodes := s.findGangNodes(task, nodes)
			if len(gangNodes) < task.WorldSize {
				continue // Not enough resources
			}
			
			masterAddr := gangNodes[0].IPAddress
			if masterAddr == "" {
				masterAddr = "127.0.0.1"
			}
			masterPort := "29500"

			for i, node := range gangNodes {
				chunk := &state.TaskChunk{
					ParentTaskID: task.ID,
					ChunkIndex:   i,
					AssignedNode: node.ID,
					Status:       "scheduled",
				}
				s.store.CreateTaskChunk(chunk)

				s.wsHub.BroadcastEvent("task_scheduled", map[string]interface{}{
					"task_id":     task.ID,
					"chunk_id":    chunk.ID,
					"node_id":     node.ID,
					"node_name":   node.Hostname,
					"world_size":  task.WorldSize,
					"rank":        i,
					"master_addr": masterAddr,
					"master_port": masterPort,
				})

				s.store.UpdateNodeLiveMetrics(node.ID, &state.NodeLiveMetrics{
					RunningTasks: node.RunningTasks + 1,
					CPUUsage:     node.CPUUsage,
					MemoryUsage:  node.MemoryUsage,
					DiskUsage:    node.DiskUsage,
					LoadAvg1:     node.LoadAvg1,
				})
			}
			s.store.UpdateTaskStatus(task.ID, "scheduled")
			continue
		}

		bestNode := s.findBestNode(task, nodes)
		if bestNode == nil {
			// No suitable node found — task stays in queue
			continue
		}

		// 4. Assign task to node
		if err := s.store.UpdateTaskAssignment(task.ID, bestNode.ID); err != nil {
			log.Printf("scheduler: failed to assign task %s to node %s: %v", task.ID, bestNode.ID, err)
			continue
		}

		log.Printf("scheduler: assigned task %s (%s) to node %s (%s)",
			task.ID, task.Name, bestNode.ID, bestNode.Hostname)

		// 5. Update task status to running (in real system, agent would confirm)
		if err := s.store.UpdateTaskStatus(task.ID, "running"); err != nil {
			log.Printf("scheduler: failed to update task status: %v", err)
		}

		// 6. Broadcast event
		s.wsHub.BroadcastEvent("task_scheduled", map[string]interface{}{
			"task_id":   task.ID,
			"task_name": task.Name,
			"node_id":   bestNode.ID,
			"node_name": bestNode.Hostname,
		})

		// Update node's running task count in live metrics
		s.store.UpdateNodeLiveMetrics(bestNode.ID, &state.NodeLiveMetrics{
			RunningTasks: bestNode.RunningTasks + 1,
			CPUUsage:     bestNode.CPUUsage,
			MemoryUsage:  bestNode.MemoryUsage,
			DiskUsage:    bestNode.DiskUsage,
			LoadAvg1:     bestNode.LoadAvg1,
		})
	}
}

// findBestNode uses weighted scoring to find the optimal node for a task.
func (s *Scheduler) findBestNode(task *state.Task, nodes []*state.Node) *state.Node {
	var bestNode *state.Node
	bestScore := -1.0

	for _, node := range nodes {
		// Skip nodes in non-schedulable states
		if node.Status != "online" {
			continue
		}

		// Hard constraints: does the node have enough resources?
		if !satisfiesConstraints(task, node) {
			continue
		}

		// If task is pinned to a specific node
		if task.TargetNode != "" {
			if node.ID == task.TargetNode {
				return node
			}
			continue
		}

		// Score the node
		score := ScoreNode(task, node)
		if score > bestScore {
			bestScore = score
			bestNode = node
		}
	}

	return bestNode
}

// satisfiesConstraints checks if a node meets the hard requirements of a task.
func satisfiesConstraints(task *state.Task, node *state.Node) bool {
	// CPU check: node must have enough available cores
	availableCores := float64(node.CPUCores) * (1.0 - node.CPUUsage/100.0)
	if int(availableCores) < task.CPURequired {
		return false
	}

	// Memory check: node must have enough available memory
	if task.MemoryRequired > 0 {
		availableMemory := float64(node.MemoryTotal) * (1.0 - node.MemoryUsage/100.0)
		if int64(availableMemory) < task.MemoryRequired {
			return false
		}
	}

	// GPU check
	if task.GPURequired > 0 {
		if node.GPUCount < task.GPURequired {
			return false
		}
	}

	return true
}

func (s *Scheduler) findGangNodes(task *state.Task, nodes []*state.Node) []*state.Node {
	var candidates []*state.Node
	for _, node := range nodes {
		if node.Status == "online" && satisfiesConstraints(task, node) {
			candidates = append(candidates, node)
		}
	}
	if len(candidates) >= task.WorldSize {
		return candidates[:task.WorldSize]
	}
	return nil
}

// heartbeatMonitor checks for nodes that haven't sent heartbeats.
func (s *Scheduler) heartbeatMonitor() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkHeartbeats()
		}
	}
}

// checkHeartbeats marks nodes offline if they haven't sent a heartbeat recently.
func (s *Scheduler) checkHeartbeats() {
	nodes, err := s.store.ListNodes()
	if err != nil {
		return
	}

	timeout := 30 * time.Second
	for _, node := range nodes {
		if node.Role == "controller" {
			continue
		}
		if node.Status == "online" && time.Since(node.LastHeartbeat) > timeout {
			log.Printf("scheduler: node %s (%s) heartbeat timeout, marking offline", node.ID, node.Hostname)
			s.store.UpdateNodeStatus(node.ID, "offline")
			s.wsHub.BroadcastEvent("node_offline", map[string]interface{}{
				"node_id":   node.ID,
				"node_name": node.Hostname,
				"reason":    "heartbeat timeout",
			})

			// Reschedule tasks that were running on this node
			s.rescheduleNodeTasks(node.ID)
		}
	}
}

// rescheduleNodeTasks moves tasks from a failed node back to the queue.
func (s *Scheduler) rescheduleNodeTasks(nodeID string) {
	tasks, _, err := s.store.ListTasks("running", "", 0, 0)
	if err != nil {
		return
	}

	for _, task := range tasks {
		if task.AssignedNode != nodeID {
			continue
		}

		if task.RetryCount < task.RetryMax {
			log.Printf("scheduler: rescheduling task %s from failed node %s (retry %d/%d)",
				task.ID, nodeID, task.RetryCount+1, task.RetryMax)
			s.store.IncrementTaskRetry(task.ID)
			s.wsHub.BroadcastEvent("task_rescheduled", map[string]interface{}{
				"task_id":    task.ID,
				"from_node":  nodeID,
				"retry_count": task.RetryCount + 1,
			})
		} else {
			log.Printf("scheduler: task %s failed on node %s, max retries exceeded", task.ID, nodeID)
			s.store.UpdateTaskResult(task.ID, 1, "node went offline, max retries exceeded")
			s.wsHub.BroadcastEvent("task_failed", map[string]interface{}{
				"task_id": task.ID,
				"reason":  "node offline, retries exhausted",
			})
		}
	}
}
