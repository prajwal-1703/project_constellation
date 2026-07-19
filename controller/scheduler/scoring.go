package scheduler

import (
	"math"

	"github.com/constellation/controller/state"
)

// ─── Scoring Weights ─────────────────────────────────────────────────────────
// Inspired by Kubernetes scheduler scoring and Linux CFS fairness concepts.

const (
	WeightCPU      = 0.35
	WeightMemory   = 0.30
	WeightLoad     = 0.25
	WeightAffinity = 0.10
)

// ScoreNode calculates a weighted score for how suitable a node is for a task.
// Higher score = better fit. Range: 0.0 to 1.0.
func ScoreNode(task *state.Task, node *state.Node) float64 {
	cpuScore := scoreCPU(task, node)
	memScore := scoreMemory(task, node)
	loadScore := scoreLoad(node)
	affinityScore := scoreAffinity(task, node)

	total := WeightCPU*cpuScore +
		WeightMemory*memScore +
		WeightLoad*loadScore +
		WeightAffinity*affinityScore

	// Penalty for nodes already running many tasks (spread scheduling)
	taskPenalty := 1.0 - math.Min(float64(node.RunningTasks)*0.1, 0.5)
	total *= taskPenalty

	return clamp(total, 0.0, 1.0)
}

// scoreCPU: how much CPU headroom does the node have after running this task?
// Prefers nodes with more spare capacity (spread scheduling).
func scoreCPU(task *state.Task, node *state.Node) float64 {
	if node.CPUCores == 0 {
		return 0
	}

	// Available cores considering current usage
	availableCores := float64(node.CPUCores) * (1.0 - node.CPUUsage/100.0)

	// Score = remaining capacity after task / total capacity
	remaining := availableCores - float64(task.CPURequired)
	if remaining < 0 {
		return 0
	}

	return remaining / float64(node.CPUCores)
}

// scoreMemory: how much memory headroom does the node have?
func scoreMemory(task *state.Task, node *state.Node) float64 {
	if node.MemoryTotal == 0 {
		return 0.5 // No memory info, neutral score
	}

	if task.MemoryRequired == 0 {
		// No memory requirement specified, score based on current usage
		return 1.0 - node.MemoryUsage/100.0
	}

	availableMemory := float64(node.MemoryTotal) * (1.0 - node.MemoryUsage/100.0)
	remaining := availableMemory - float64(task.MemoryRequired)
	if remaining < 0 {
		return 0
	}

	return remaining / float64(node.MemoryTotal)
}

// scoreLoad: prefer nodes with lower load averages.
func scoreLoad(node *state.Node) float64 {
	if node.CPUCores == 0 {
		return 0.5
	}

	// Load average normalized by CPU count
	// Load of 1.0 per core means fully utilized
	normalizedLoad := node.LoadAvg1 / float64(node.CPUCores)

	// Score inversely proportional to load
	score := 1.0 - math.Min(normalizedLoad, 1.0)
	return score
}

// scoreAffinity: bonus for label matching.
func scoreAffinity(task *state.Task, node *state.Node) float64 {
	// For MVP, no label matching implemented — all nodes get neutral score
	// In the future, this would check task labels against node labels
	_ = task
	_ = node
	return 0.5
}

// clamp restricts a value to [min, max].
func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
