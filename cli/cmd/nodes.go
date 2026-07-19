package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var nodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "Manage cluster nodes",
}

var nodesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all cluster nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := apiGet("/api/v1/nodes")
		if err != nil {
			printError(fmt.Sprintf("Failed to list nodes: %v", err))
			return nil
		}

		nodes, _ := result["nodes"].([]interface{})
		if len(nodes) == 0 {
			printInfo("No nodes in cluster")
			return nil
		}

		fmt.Println()
		fmt.Printf("  %-14s %-16s %-10s %-8s %-12s %-10s\n",
			"NODE ID", "HOSTNAME", "STATUS", "ROLE", "CPU", "MEMORY")
		fmt.Println("  " + fmt.Sprintf("%s", "─────────────────────────────────────────────────────────────────────────"))

		for _, n := range nodes {
			node, ok := n.(map[string]interface{})
			if !ok {
				continue
			}

			id, _ := node["id"].(string)
			hostname, _ := node["hostname"].(string)
			status, _ := node["status"].(string)
			role, _ := node["role"].(string)
			cpuCores, _ := node["cpu_cores"].(float64)
			memTotal, _ := node["memory_total"].(float64)

			statusIcon := "●"
			if status == "offline" {
				statusIcon = "○"
			} else if status == "draining" {
				statusIcon = "◐"
			}

			fmt.Printf("  %-14s %-16s %s %-8s %-8s %-8d %-10s\n",
				id, hostname, statusIcon, status, role,
				int(cpuCores), formatBytes(memTotal))
		}
		fmt.Println()

		return nil
	},
}

var nodesInfoCmd = &cobra.Command{
	Use:   "info [node-id]",
	Short: "Show detailed node information",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeID := args[0]
		result, err := apiGet("/api/v1/nodes/" + nodeID)
		if err != nil {
			printError(fmt.Sprintf("Node not found: %v", err))
			return nil
		}

		fmt.Println()
		fmt.Println("  ┌─ Node Details ────────────────────────────────┐")

		fields := []struct{ label, key string }{
			{"ID", "id"}, {"Hostname", "hostname"}, {"IP", "ip_address"},
			{"Status", "status"}, {"Role", "role"}, {"OS", "os_name"},
			{"Arch", "arch"}, {"CPU Model", "cpu_model"},
		}
		for _, f := range fields {
			val := fmt.Sprintf("%v", result[f.key])
			fmt.Printf("  │  %-12s: %-35s│\n", f.label, val)
		}

		cpuCores, _ := result["cpu_cores"].(float64)
		cpuUsage, _ := result["cpu_usage"].(float64)
		memTotal, _ := result["memory_total"].(float64)
		memUsage, _ := result["memory_usage"].(float64)

		fmt.Printf("  │  %-12s: %-35s│\n", "CPU Cores", fmt.Sprintf("%d (%.1f%% used)", int(cpuCores), cpuUsage))
		fmt.Printf("  │  %-12s: %-35s│\n", "Memory", fmt.Sprintf("%s (%.1f%% used)", formatBytes(memTotal), memUsage))
		fmt.Println("  └────────────────────────────────────────────────┘")
		fmt.Println()

		return nil
	},
}

var nodesActivateCmd = &cobra.Command{
	Use:   "activate [node-id]",
	Short: "Activate a node for scheduling",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := apiPut("/api/v1/nodes/"+args[0]+"/status", map[string]string{"status": "online"})
		if err != nil {
			printError(fmt.Sprintf("Failed: %v", err))
			return nil
		}
		printSuccess("Node " + args[0] + " activated")
		return nil
	},
}

var nodesDeactivateCmd = &cobra.Command{
	Use:   "deactivate [node-id]",
	Short: "Deactivate a node (stop scheduling)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := apiPut("/api/v1/nodes/"+args[0]+"/status", map[string]string{"status": "maintenance"})
		if err != nil {
			printError(fmt.Sprintf("Failed: %v", err))
			return nil
		}
		printSuccess("Node " + args[0] + " deactivated")
		return nil
	},
}

var nodesDrainCmd = &cobra.Command{
	Use:   "drain [node-id]",
	Short: "Drain a node (finish tasks, then deactivate)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := apiPut("/api/v1/nodes/"+args[0]+"/status", map[string]string{"status": "draining"})
		if err != nil {
			printError(fmt.Sprintf("Failed: %v", err))
			return nil
		}
		printSuccess("Node " + args[0] + " draining")
		return nil
	},
}

var nodesRemoveCmd = &cobra.Command{
	Use:   "remove [node-id]",
	Short: "Remove a node from the cluster",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := apiDelete("/api/v1/nodes/" + args[0])
		if err != nil {
			printError(fmt.Sprintf("Failed: %v", err))
			return nil
		}
		printSuccess("Node " + args[0] + " removed")
		return nil
	},
}

func init() {
	nodesCmd.AddCommand(nodesListCmd)
	nodesCmd.AddCommand(nodesInfoCmd)
	nodesCmd.AddCommand(nodesActivateCmd)
	nodesCmd.AddCommand(nodesDeactivateCmd)
	nodesCmd.AddCommand(nodesDrainCmd)
	nodesCmd.AddCommand(nodesRemoveCmd)
	rootCmd.AddCommand(nodesCmd)
}
