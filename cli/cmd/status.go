package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cluster status and computation pool",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := apiGet("/api/v1/cluster/status")
		if err != nil {
			printError(fmt.Sprintf("Failed to get cluster status: %v", err))
			return nil
		}

		cluster, _ := result["cluster"].(map[string]interface{})
		health, _ := result["health"].(string)
		pool, _ := result["pool"].(map[string]interface{})
		taskStats, _ := result["task_stats"].(map[string]interface{})

		clusterName := ""
		if cluster != nil {
			clusterName, _ = cluster["name"].(string)
		}

		// Health indicator
		healthIcon := "✓"
		if health == "degraded" {
			healthIcon = "⚠"
		} else if health == "critical" {
			healthIcon = "✗"
		}

		fmt.Println()
		fmt.Println("  ┌─────────────────────────────────────────────┐")
		fmt.Printf("  │  Cluster: %-33s│\n", clusterName)
		fmt.Printf("  │  Status:  %s %-31s│\n", healthIcon, health)

		if pool != nil {
			totalNodes, _ := pool["total_nodes"].(float64)
			onlineNodes, _ := pool["online_nodes"].(float64)
			totalCPU, _ := pool["total_cpu_cores"].(float64)
			availCPU, _ := pool["available_cpu_cores"].(float64)
			totalMem, _ := pool["total_memory_bytes"].(float64)
			availMem, _ := pool["available_memory_bytes"].(float64)
			totalDisk, _ := pool["total_disk_bytes"].(float64)
			availDisk, _ := pool["available_disk_bytes"].(float64)

			fmt.Printf("  │  Nodes:   %d online / %d total %-14s│\n", int(onlineNodes), int(totalNodes), "")
			fmt.Println("  │  ── Computation Pool ──                     │")
			fmt.Printf("  │  CPU:     %d cores (%d available) %-10s│\n", int(totalCPU), int(availCPU), "")
			fmt.Printf("  │  Memory:  %s (%s available) %-4s│\n",
				formatBytes(totalMem), formatBytes(availMem), "")
			fmt.Printf("  │  Storage: %s (%s available) %-2s│\n",
				formatBytes(totalDisk), formatBytes(availDisk), "")
		}

		if taskStats != nil {
			running, _ := taskStats["running"].(float64)
			queued, _ := taskStats["queued"].(float64)
			completed, _ := taskStats["completed"].(float64)
			failed, _ := taskStats["failed"].(float64)

			fmt.Println("  │  ── Tasks ──                                │")
			fmt.Printf("  │  Running: %-5d Queued: %-18d│\n", int(running), int(queued))
			fmt.Printf("  │  Done: %-8d Failed: %-17d│\n", int(completed), int(failed))
		}

		fmt.Println("  └─────────────────────────────────────────────┘")
		fmt.Println()

		return nil
	},
}

var poolCmd = &cobra.Command{
	Use:   "pool",
	Short: "Show the aggregated computation pool",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := apiGet("/api/v1/cluster/pool")
		if err != nil {
			printError(fmt.Sprintf("Failed to get pool: %v", err))
			return nil
		}

		totalCPU, _ := result["total_cpu_cores"].(float64)
		availCPU, _ := result["available_cpu_cores"].(float64)
		totalMem, _ := result["total_memory_bytes"].(float64)
		availMem, _ := result["available_memory_bytes"].(float64)
		totalDisk, _ := result["total_disk_bytes"].(float64)
		availDisk, _ := result["available_disk_bytes"].(float64)
		totalNodes, _ := result["total_nodes"].(float64)
		onlineNodes, _ := result["online_nodes"].(float64)

		fmt.Println()
		fmt.Println("  ┌─────────────────────────────────────────────┐")
		fmt.Println("  │  ✦ Computation Pool                         │")
		fmt.Printf("  │  Nodes:   %d online / %d total %-14s│\n", int(onlineNodes), int(totalNodes), "")
		fmt.Printf("  │  CPU:     %d cores (%d available) %-10s│\n", int(totalCPU), int(availCPU), "")
		fmt.Printf("  │  Memory:  %s (%s avail) %-8s│\n", formatBytes(totalMem), formatBytes(availMem), "")
		fmt.Printf("  │  Storage: %s (%s avail) %-6s│\n", formatBytes(totalDisk), formatBytes(availDisk), "")
		fmt.Println("  └─────────────────────────────────────────────┘")
		fmt.Println()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(poolCmd)
}
