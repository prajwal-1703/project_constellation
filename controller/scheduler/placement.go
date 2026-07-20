package scheduler

import (
	"fmt"
	"log"

	"github.com/constellation/controller/state"
)

// PlacementResult represents the result of a scheduling decision.
type PlacementResult struct {
	TaskID   string
	NodeID   string
	NodeName string
	Score    float64
	Reason   string
}

// PlaceTask runs the full scheduling pipeline for a single task.
// Returns the best placement or an error if no suitable node is found.
func PlaceTask(task *state.Task, nodes []*state.Node) (*PlacementResult, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes available for scheduling")
	}

	// Phase 1: Filter — remove nodes that can't satisfy hard constraints
	var candidates []*state.Node
	var filterReasons []string
	for _, node := range nodes {
		if node.Status != "online" {
			filterReasons = append(filterReasons, fmt.Sprintf("  %s: status=%s (not online)", node.ID, node.Status))
			continue
		}
		if !satisfiesConstraints(task, node) {
			filterReasons = append(filterReasons, fmt.Sprintf("  %s: insufficient resources (need %d CPU, %d bytes mem)",
				node.ID, task.CPURequired, task.MemoryRequired))
			continue
		}
		candidates = append(candidates, node)
	}

	if len(candidates) == 0 {
		msg := fmt.Sprintf("no node satisfies task requirements (CPU: %d, Memory: %d bytes)", task.CPURequired, task.MemoryRequired)
		if len(filterReasons) > 0 {
			msg += "\nFilter reasons:\n"
			for _, r := range filterReasons {
				msg += r + "\n"
			}
		}
		return nil, fmt.Errorf("%s", msg)
	}

	// Phase 2: If target node specified, use it
	if task.TargetNode != "" {
		for _, node := range candidates {
			if node.ID == task.TargetNode {
				return &PlacementResult{
					TaskID:   task.ID,
					NodeID:   node.ID,
					NodeName: node.Hostname,
					Score:    1.0,
					Reason:   "pinned to target node",
				}, nil
			}
		}
		return nil, fmt.Errorf("target node %s is not available", task.TargetNode)
	}

	// Phase 3: Score — rank candidates
	var bestNode *state.Node
	bestScore := -1.0

	for _, node := range candidates {
		score := ScoreNode(task, node)
		log.Printf("scheduler: scoring node %s (%s): %.3f", node.ID, node.Hostname, score)
		if score > bestScore {
			bestScore = score
			bestNode = node
		}
	}

	if bestNode == nil {
		return nil, fmt.Errorf("scoring failed to find a suitable node")
	}

	return &PlacementResult{
		TaskID:   task.ID,
		NodeID:   bestNode.ID,
		NodeName: bestNode.Hostname,
		Score:    bestScore,
		Reason:   fmt.Sprintf("best score: %.3f (out of %d candidates)", bestScore, len(candidates)),
	}, nil
}
