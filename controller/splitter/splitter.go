package splitter

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/constellation/controller/state"
	"github.com/google/uuid"
)

// Splitter handles breaking large tasks into smaller parallel chunks.
type Splitter struct {
	store    *state.Store
	taskDir  string // Base directory for task files
}

// NewSplitter creates a new task splitter.
func NewSplitter(store *state.Store, taskDir string) *Splitter {
	return &Splitter{
		store:   store,
		taskDir: taskDir,
	}
}

// SplitTask analyzes a task and creates sub-chunks based on the split strategy.
func (sp *Splitter) SplitTask(task *state.Task) ([]state.TaskChunk, error) {
	switch task.SplitStrategy {
	case "chunk":
		return sp.chunkSplit(task)
	case "map-reduce":
		return sp.mapReduceSplit(task)
	case "pipeline":
		return sp.pipelineSplit(task)
	case "replicated":
		return sp.replicatedSplit(task)
	default:
		return nil, fmt.Errorf("unknown split strategy: %s", task.SplitStrategy)
	}
}

// ─── Chunk Split ─────────────────────────────────────────────────────────────
// Splits input file into N chunks by lines, files, or size.

func (sp *Splitter) chunkSplit(task *state.Task) ([]state.TaskChunk, error) {
	if task.InputFile == "" {
		return nil, fmt.Errorf("input_file is required for chunk split")
	}

	splitBy := task.SplitBy
	if splitBy == "" {
		splitBy = "lines"
	}

	chunkSize := task.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 10000 // Default: 10K lines per chunk
	}

	switch splitBy {
	case "lines":
		return sp.splitByLines(task, chunkSize)
	case "size":
		return sp.splitBySize(task, int64(chunkSize))
	default:
		return sp.splitByLines(task, chunkSize)
	}
}

func (sp *Splitter) splitByLines(task *state.Task, linesPerChunk int) ([]state.TaskChunk, error) {
	// Create task directory
	taskDir := filepath.Join(sp.taskDir, task.ID)
	inputDir := filepath.Join(taskDir, "input")
	outputDir := filepath.Join(taskDir, "output")
	os.MkdirAll(inputDir, 0755)
	os.MkdirAll(outputDir, 0755)

	// Count total lines
	file, err := os.Open(task.InputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open input file: %w", err)
	}
	defer file.Close()

	totalLines := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer
	for scanner.Scan() {
		totalLines++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to count lines: %w", err)
	}

	if totalLines == 0 {
		return nil, fmt.Errorf("input file is empty")
	}

	// Calculate number of chunks
	numChunks := (totalLines + linesPerChunk - 1) / linesPerChunk

	// Re-read and split
	file.Seek(0, 0)
	scanner = bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var chunks []state.TaskChunk
	lineNum := 0
	chunkIdx := 0

	var currentWriter *os.File
	var currentBufWriter *bufio.Writer

	for scanner.Scan() {
		line := scanner.Text()

		// Start new chunk if needed
		if lineNum%linesPerChunk == 0 {
			if currentBufWriter != nil {
				currentBufWriter.Flush()
				currentWriter.Close()
			}

			chunkFile := filepath.Join(inputDir, fmt.Sprintf("chunk_%04d.dat", chunkIdx))
			outputFile := filepath.Join(outputDir, fmt.Sprintf("result_%04d.dat", chunkIdx))

			currentWriter, err = os.Create(chunkFile)
			if err != nil {
				return nil, fmt.Errorf("failed to create chunk file: %w", err)
			}
			currentBufWriter = bufio.NewWriter(currentWriter)

			chunk := state.TaskChunk{
				ID:           fmt.Sprintf("chunk-%s-%d", task.ID, chunkIdx),
				ParentTaskID: task.ID,
				ChunkIndex:   chunkIdx,
				Status:       "queued",
				InputFile:    chunkFile,
				OutputFile:   outputFile,
			}
			chunks = append(chunks, chunk)
			chunkIdx++
		}

		currentBufWriter.WriteString(line + "\n")
		lineNum++
	}

	if currentBufWriter != nil {
		currentBufWriter.Flush()
		currentWriter.Close()
	}

	log.Printf("splitter: split %s into %d chunks (%d lines each, %d total lines)",
		task.ID, numChunks, linesPerChunk, totalLines)

	return chunks, scanner.Err()
}

func (sp *Splitter) splitBySize(task *state.Task, bytesPerChunk int64) ([]state.TaskChunk, error) {
	taskDir := filepath.Join(sp.taskDir, task.ID)
	inputDir := filepath.Join(taskDir, "input")
	outputDir := filepath.Join(taskDir, "output")
	os.MkdirAll(inputDir, 0755)
	os.MkdirAll(outputDir, 0755)

	info, err := os.Stat(task.InputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to stat input file: %w", err)
	}

	totalSize := info.Size()
	numChunks := int((totalSize + bytesPerChunk - 1) / bytesPerChunk)

	file, err := os.Open(task.InputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open input file: %w", err)
	}
	defer file.Close()

	var chunks []state.TaskChunk
	buf := make([]byte, 32*1024) // 32KB read buffer

	for i := 0; i < numChunks; i++ {
		chunkFile := filepath.Join(inputDir, fmt.Sprintf("chunk_%04d.dat", i))
		outputFile := filepath.Join(outputDir, fmt.Sprintf("result_%04d.dat", i))

		out, err := os.Create(chunkFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create chunk file: %w", err)
		}

		var written int64
		for written < bytesPerChunk {
			toRead := int64(len(buf))
			if remaining := bytesPerChunk - written; remaining < toRead {
				toRead = remaining
			}
			n, readErr := file.Read(buf[:toRead])
			if n > 0 {
				out.Write(buf[:n])
				written += int64(n)
			}
			if readErr != nil {
				break
			}
		}
		out.Close()

		chunk := state.TaskChunk{
			ID:           fmt.Sprintf("chunk-%s-%d", task.ID, i),
			ParentTaskID: task.ID,
			ChunkIndex:   i,
			Status:       "queued",
			InputFile:    chunkFile,
			OutputFile:   outputFile,
		}
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// ─── Map-Reduce Split ────────────────────────────────────────────────────────

func (sp *Splitter) mapReduceSplit(task *state.Task) ([]state.TaskChunk, error) {
	// Map-reduce uses chunk splitting for the map phase
	// The reduce step is tracked separately in the task's ReduceCommand
	chunks, err := sp.chunkSplit(task)
	if err != nil {
		return nil, err
	}

	// Mark these as map phase chunks
	for i := range chunks {
		chunks[i].ID = fmt.Sprintf("map-%s-%d", task.ID, i)
	}

	return chunks, nil
}

// ─── Pipeline Split ──────────────────────────────────────────────────────────

func (sp *Splitter) pipelineSplit(task *state.Task) ([]state.TaskChunk, error) {
	// Pipeline: each command separated by | runs as a separate stage
	commands := strings.Split(task.Command, "|")
	if len(commands) < 2 {
		return nil, fmt.Errorf("pipeline split requires at least 2 stages (separate with |)")
	}

	taskDir := filepath.Join(sp.taskDir, task.ID)
	os.MkdirAll(filepath.Join(taskDir, "pipeline"), 0755)

	var chunks []state.TaskChunk
	for i, cmd := range commands {
		cmd = strings.TrimSpace(cmd)
		var inputFile, outputFile string

		if i == 0 {
			inputFile = task.InputFile
		} else {
			inputFile = filepath.Join(taskDir, "pipeline", fmt.Sprintf("stage_%d_out.dat", i-1))
		}
		outputFile = filepath.Join(taskDir, "pipeline", fmt.Sprintf("stage_%d_out.dat", i))

		chunk := state.TaskChunk{
			ID:           fmt.Sprintf("pipe-%s-%d", task.ID, i),
			ParentTaskID: task.ID,
			ChunkIndex:   i,
			Status:       "queued",
			InputFile:    inputFile,
			OutputFile:   outputFile,
		}
		_ = cmd // Command stored in parent task, stage index determines which part
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// ─── Replicated Split ────────────────────────────────────────────────────────

func (sp *Splitter) replicatedSplit(task *state.Task) ([]state.TaskChunk, error) {
	// Run the same task on multiple nodes (for redundancy or parameter sweep)
	nodes, err := sp.store.GetOnlineWorkerNodes()
	if err != nil {
		return nil, fmt.Errorf("failed to get online nodes: %w", err)
	}

	numReplicas := len(nodes)
	if task.MaxNodes > 0 && task.MaxNodes < numReplicas {
		numReplicas = task.MaxNodes
	}

	if numReplicas == 0 {
		return nil, fmt.Errorf("no online worker nodes available for replication")
	}

	taskDir := filepath.Join(sp.taskDir, task.ID)
	outputDir := filepath.Join(taskDir, "output")
	os.MkdirAll(outputDir, 0755)

	var chunks []state.TaskChunk
	for i := 0; i < numReplicas; i++ {
		chunk := state.TaskChunk{
			ID:           fmt.Sprintf("replica-%s-%d", task.ID, i),
			ParentTaskID: task.ID,
			ChunkIndex:   i,
			Status:       "queued",
			InputFile:    task.InputFile,
			OutputFile:   filepath.Join(outputDir, fmt.Sprintf("replica_%d.dat", i)),
		}
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// ─── Result Aggregation ──────────────────────────────────────────────────────

// AggregateResults collects all chunk outputs and determines if the task is complete.
func (sp *Splitter) AggregateResults(task *state.Task) (bool, error) {
	if task.Type != "split" {
		return true, nil
	}

	allComplete := true
	anyFailed := false

	for _, chunk := range task.Chunks {
		if chunk.Status == "failed" {
			anyFailed = true
		}
		if chunk.Status != "completed" && chunk.Status != "failed" {
			allComplete = false
		}
	}

	if !allComplete {
		return false, nil
	}

	if anyFailed {
		return true, fmt.Errorf("one or more chunks failed")
	}

	return true, nil
}

// GenerateReduceTask creates the reduce step task after all chunks complete.
func (sp *Splitter) GenerateReduceTask(parentTask *state.Task) (*state.Task, error) {
	if parentTask.ReduceCommand == "" {
		return nil, nil // No reduce step needed
	}

	// Collect all output files
	var outputFiles []string
	for _, chunk := range parentTask.Chunks {
		outputFiles = append(outputFiles, chunk.OutputFile)
	}

	taskDir := filepath.Join(sp.taskDir, parentTask.ID)
	finalOutput := filepath.Join(taskDir, "output", "final_result.dat")

	// Replace template variables in reduce command
	cmd := parentTask.ReduceCommand
	cmd = strings.ReplaceAll(cmd, "{{.AllOutputs}}", strings.Join(outputFiles, " "))
	cmd = strings.ReplaceAll(cmd, "{{.AllOutputFiles}}", strings.Join(outputFiles, " "))
	cmd = strings.ReplaceAll(cmd, "{{.Output}}", finalOutput)

	reduceTask := &state.Task{
		ID:             "reduce-" + uuid.New().String()[:8],
		Name:           parentTask.Name + " (reduce)",
		Command:        cmd,
		Type:           "simple",
		Runtime:        parentTask.Runtime,
		DockerImage:    parentTask.DockerImage,
		CPURequired:    parentTask.CPURequired,
		MemoryRequired: parentTask.MemoryRequired,
		Priority:       parentTask.Priority,
		Status:         "queued",
		ExitCode:       -1,
	}

	return reduceTask, nil
}
