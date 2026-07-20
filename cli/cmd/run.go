package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	runCmd_cmd      string
	runCmd_file     string
	runCmd_name     string
	runCmd_cpu      int
	runCmd_memory   string
	runCmd_priority string
	runCmd_runtime  string
	runCmd_docker   string
	runCmd_retry    int
	runCmd_timeout  int
	runCmd_node     string
	runCmd_split    string
	runCmd_gpus     int
	runCmd_worldSize int
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Submit a task for execution",
	Long: `Submit a task to the cluster for execution.

Examples:
  constellation run --cmd "python train.py" --cpu 4 --memory 8GB
  constellation run --cmd "make build" --priority high
  constellation run --file job.yaml --split chunk`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if runCmd_cmd == "" && runCmd_file == "" {
			printError("Either --cmd or --file is required")
			return nil
		}

		req := map[string]interface{}{
			"command":     runCmd_cmd,
			"cpu_required": runCmd_cpu,
			"priority":    runCmd_priority,
			"runtime":     runCmd_runtime,
			"retry_max":   runCmd_retry,
			"gpu_required": runCmd_gpus,
		}

		if runCmd_worldSize > 1 {
			req["is_distributed"] = true
			req["world_size"] = runCmd_worldSize
		}

		if runCmd_name != "" {
			req["name"] = runCmd_name
		}
		if runCmd_memory != "" {
			req["memory_required"] = runCmd_memory
		}
		if runCmd_docker != "" {
			req["docker_image"] = runCmd_docker
		}
		if runCmd_timeout > 0 {
			req["timeout_seconds"] = runCmd_timeout
		}
		if runCmd_node != "" {
			req["target_node"] = runCmd_node
		}
		if runCmd_split != "" {
			req["split_strategy"] = runCmd_split
		}

		fmt.Println()
		result, err := apiPost("/api/v1/tasks", req)
		if err != nil {
			printError(fmt.Sprintf("Failed to submit task: %v", err))
			return nil
		}

		taskID, _ := result["task_id"].(string)
		status, _ := result["status"].(string)

		printSuccess(fmt.Sprintf("Task %s submitted", taskID))
		printInfo(fmt.Sprintf("Status: %s", status))
		fmt.Println()

		return nil
	},
}

var tasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Manage tasks",
}

var tasksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := apiGet("/api/v1/tasks")
		if err != nil {
			printError(fmt.Sprintf("Failed: %v", err))
			return nil
		}

		tasks, _ := result["tasks"].([]interface{})
		if len(tasks) == 0 {
			printInfo("No tasks")
			return nil
		}

		fmt.Println()
		fmt.Printf("  %-14s %-20s %-12s %-10s %-14s\n",
			"TASK ID", "NAME", "STATUS", "PRIORITY", "NODE")
		fmt.Println("  " + "─────────────────────────────────────────────────────────────────────────")

		for _, t := range tasks {
			task, ok := t.(map[string]interface{})
			if !ok {
				continue
			}

			id, _ := task["id"].(string)
			name, _ := task["name"].(string)
			status, _ := task["status"].(string)
			priority, _ := task["priority"].(string)
			node, _ := task["assigned_node"].(string)

			if len(name) > 18 {
				name = name[:18] + ".."
			}
			if node == "" {
				node = "-"
			}

			statusIcon := "◻"
			switch status {
			case "running":
				statusIcon = "▶"
			case "completed":
				statusIcon = "✓"
			case "failed":
				statusIcon = "✗"
			case "queued":
				statusIcon = "◌"
			case "cancelled":
				statusIcon = "⊘"
			}

			fmt.Printf("  %-14s %-20s %s %-10s %-10s %-14s\n",
				id, name, statusIcon, status, priority, node)
		}
		fmt.Println()

		return nil
	},
}

var tasksInfoCmd = &cobra.Command{
	Use:   "info [task-id]",
	Short: "Show task details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := apiGet("/api/v1/tasks/" + args[0])
		if err != nil {
			printError(fmt.Sprintf("Task not found: %v", err))
			return nil
		}

		fmt.Println()
		fmt.Println("  ┌─ Task Details ────────────────────────────────┐")
		fields := []struct{ label, key string }{
			{"ID", "id"}, {"Name", "name"}, {"Status", "status"},
			{"Command", "command"}, {"Priority", "priority"},
			{"Runtime", "runtime"}, {"Node", "assigned_node"},
		}
		for _, f := range fields {
			val := fmt.Sprintf("%v", result[f.key])
			if len(val) > 34 {
				val = val[:34] + ".."
			}
			fmt.Printf("  │  %-12s: %-35s│\n", f.label, val)
		}
		fmt.Println("  └────────────────────────────────────────────────┘")
		fmt.Println()
		return nil
	},
}

var tasksCancelCmd = &cobra.Command{
	Use:   "cancel [task-id]",
	Short: "Cancel a running task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := apiDelete("/api/v1/tasks/" + args[0])
		if err != nil {
			printError(fmt.Sprintf("Failed: %v", err))
			return nil
		}
		printSuccess("Task " + args[0] + " cancelled")
		return nil
	},
}

var tasksRetryCmd = &cobra.Command{
	Use:   "retry [task-id]",
	Short: "Retry a failed task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := apiPost("/api/v1/tasks/"+args[0]+"/retry", nil)
		if err != nil {
			printError(fmt.Sprintf("Failed: %v", err))
			return nil
		}
		printSuccess("Task " + args[0] + " requeued for retry")
		return nil
	},
}

var tasksLogsCmd = &cobra.Command{
	Use:   "logs [task-id]",
	Short: "Stream task logs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := apiGet("/api/v1/tasks/" + args[0] + "/logs")
		if err != nil {
			printError(fmt.Sprintf("Failed: %v", err))
			return nil
		}

		logs, _ := result["logs"].([]interface{})
		for _, l := range logs {
			fmt.Println(l)
		}
		return nil
	},
}

func init() {
	runCmd.Flags().StringVar(&runCmd_cmd, "cmd", "", "Command to execute")
	runCmd.Flags().StringVar(&runCmd_file, "file", "", "Task definition file (YAML)")
	runCmd.Flags().StringVar(&runCmd_name, "name", "", "Task name")
	runCmd.Flags().IntVar(&runCmd_cpu, "cpu", 1, "CPU cores required")
	runCmd.Flags().StringVar(&runCmd_memory, "memory", "", "Memory required (e.g., 4GB)")
	runCmd.Flags().StringVar(&runCmd_priority, "priority", "normal", "Priority: critical, high, normal, low")
	runCmd.Flags().StringVar(&runCmd_runtime, "runtime", "bare", "Runtime: bare, docker")
	runCmd.Flags().StringVar(&runCmd_docker, "docker-image", "", "Docker image (if runtime=docker)")
	runCmd.Flags().IntVar(&runCmd_retry, "retry", 0, "Number of retries on failure")
	runCmd.Flags().IntVar(&runCmd_timeout, "timeout", 0, "Timeout in seconds")
	runCmd.Flags().StringVar(&runCmd_node, "node", "", "Pin to specific node")
	runCmd.Flags().StringVar(&runCmd_split, "split", "", "Split strategy: chunk, map-reduce, pipeline")
	runCmd.Flags().IntVar(&runCmd_gpus, "gpus", 0, "GPUs required per node")
	runCmd.Flags().IntVar(&runCmd_worldSize, "world-size", 1, "Number of nodes required (for Gang Scheduling)")

	tasksCmd.AddCommand(tasksListCmd)
	tasksCmd.AddCommand(tasksInfoCmd)
	tasksCmd.AddCommand(tasksCancelCmd)
	tasksCmd.AddCommand(tasksRetryCmd)
	tasksCmd.AddCommand(tasksLogsCmd)

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(tasksCmd)
}
