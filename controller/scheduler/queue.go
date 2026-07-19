package scheduler

import (
	"container/heap"
	"sync"

	"github.com/constellation/controller/state"
)

// ─── Priority Queue ──────────────────────────────────────────────────────────
// Thread-safe priority queue for tasks.
// Priority order: Critical (4) > High (3) > Normal (2) > Low (1)
// Within same priority, FIFO by creation time.

type PriorityQueue struct {
	items taskHeap
	mu    sync.Mutex
}

func NewPriorityQueue() *PriorityQueue {
	pq := &PriorityQueue{}
	heap.Init(&pq.items)
	return pq
}

// Enqueue adds a task to the priority queue.
func (pq *PriorityQueue) Enqueue(task *state.Task) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	item := &queueItem{
		task:     task,
		priority: priorityToInt(task.Priority),
	}
	heap.Push(&pq.items, item)
}

// Dequeue removes and returns the highest priority task.
func (pq *PriorityQueue) Dequeue() *state.Task {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.items.Len() == 0 {
		return nil
	}

	item := heap.Pop(&pq.items).(*queueItem)
	return item.task
}

// Peek returns the highest priority task without removing it.
func (pq *PriorityQueue) Peek() *state.Task {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.items.Len() == 0 {
		return nil
	}

	return pq.items[0].task
}

// Len returns the number of tasks in the queue.
func (pq *PriorityQueue) Len() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return pq.items.Len()
}

// Remove removes a specific task from the queue by ID.
func (pq *PriorityQueue) Remove(taskID string) bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	for i, item := range pq.items {
		if item.task.ID == taskID {
			heap.Remove(&pq.items, i)
			return true
		}
	}
	return false
}

// ─── Internal Heap Implementation ────────────────────────────────────────────

type queueItem struct {
	task     *state.Task
	priority int
	index    int
}

type taskHeap []*queueItem

func (h taskHeap) Len() int { return len(h) }

func (h taskHeap) Less(i, j int) bool {
	// Higher priority number = higher priority
	if h[i].priority != h[j].priority {
		return h[i].priority > h[j].priority
	}
	// Same priority: FIFO by creation time
	return h[i].task.CreatedAt.Before(h[j].task.CreatedAt)
}

func (h taskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *taskHeap) Push(x interface{}) {
	n := len(*h)
	item := x.(*queueItem)
	item.index = n
	*h = append(*h, item)
}

func (h *taskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // GC
	item.index = -1
	*h = old[:n-1]
	return item
}

// priorityToInt converts priority string to integer for comparison.
func priorityToInt(p string) int {
	switch p {
	case "critical":
		return 4
	case "high":
		return 3
	case "normal":
		return 2
	case "low":
		return 1
	default:
		return 2 // default to normal
	}
}
