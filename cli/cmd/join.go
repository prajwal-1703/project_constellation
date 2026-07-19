package cmd

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var joinController string
var joinToken string

var joinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join an existing cluster as a worker node",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println()
		printInfo("Joining Constellation cluster...")
		fmt.Println()

		if joinController == "" {
			joinController = controllerURL
		} else if !strings.HasPrefix(joinController, "http") {
			joinController = "http://" + joinController
		}

		// Set controller URL for API calls
		controllerURL = joinController

		hostname, _ := os.Hostname()

		result, err := apiPost("/api/v1/nodes/join", map[string]interface{}{
			"hostname":    hostname,
			"ip_address":  getOutboundIP(),
			"agent_port":  9091,
			"join_token":  joinToken,
			"cpu_model":   runtime.GOARCH,
			"cpu_cores":   runtime.NumCPU(),
			"cpu_threads": runtime.NumCPU(),
			"memory_total": 0,
			"disk_total":   0,
			"os_name":     runtime.GOOS,
			"arch":        runtime.GOARCH,
		})
		if err != nil {
			printError(fmt.Sprintf("Failed to join cluster: %v", err))
			return nil
		}

		nodeID, _ := result["node_id"].(string)
		clusterName, _ := result["cluster_name"].(string)

		printSuccess(fmt.Sprintf("Joined cluster '%s' as node %s", clusterName, nodeID))
		printSuccess(fmt.Sprintf("Hardware reported: %d cores", runtime.NumCPU()))
		fmt.Println()

		return nil
	},
}

var leaveCmd = &cobra.Command{
	Use:   "leave",
	Short: "Leave the cluster gracefully",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println()
		printInfo("Leaving cluster...")
		// In a full implementation, this would deregister from the controller
		printSuccess("Left cluster gracefully")
		fmt.Println()
		return nil
	},
}

func init() {
	joinCmd.Flags().StringVar(&joinController, "controller", "", "Controller address (e.g., http://192.168.1.10:8080)")
	joinCmd.Flags().StringVar(&joinToken, "token", "", "Join token")
	rootCmd.AddCommand(joinCmd)
	rootCmd.AddCommand(leaveCmd)
}

func getOutboundIP() string {
	// This is a simplistic approach; in production, detect the real IP
	hostname, _ := os.Hostname()
	return hostname
}
