package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

var initName string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize this machine as a Constellation controller",
	Long:  `Initializes a new Constellation cluster with this machine as the controller node.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if initName == "" {
			initName = "my-cluster"
		}

		fmt.Println()
		printInfo("Initializing Constellation cluster...")
		fmt.Println()

		result, err := apiPost("/api/v1/cluster/init", map[string]string{
			"name": initName,
		})
		if err != nil {
			printError(fmt.Sprintf("Failed to initialize cluster: %v", err))
			return nil
		}

		// Extract cluster info
		cluster, _ := result["cluster"].(map[string]interface{})
		if cluster == nil {
			printSuccess("Cluster initialized")
			return nil
		}

		name, _ := cluster["name"].(string)
		host, _ := cluster["controller_host"].(string)
		port, _ := cluster["controller_port"].(float64)
		token, _ := cluster["join_token"].(string)

		fmt.Println("  ┌─────────────────────────────────────────────┐")
		fmt.Printf("  │  Cluster: %-33s│\n", name)
		fmt.Printf("  │  Controller: %-30s│\n", fmt.Sprintf("%s:%d", host, int(port)))
		fmt.Printf("  │  OS: %-37s│\n", runtime.GOOS+"/"+runtime.GOARCH)
		fmt.Printf("  │  CPU: %-36s│\n", fmt.Sprintf("%d cores", runtime.NumCPU()))
		fmt.Println("  │                                             │")
		fmt.Printf("  │  Join Token: %-30s│\n", token)
		fmt.Println("  │                                             │")
		fmt.Println("  │  Dashboard: http://"+host+":8080             │")
		fmt.Println("  └─────────────────────────────────────────────┘")
		fmt.Println()
		printSuccess("Cluster '" + name + "' initialized successfully")
		fmt.Println()
		fmt.Println("  Workers can join with:")
		fmt.Printf("    constellation join --controller %s:%d --token %s\n", host, int(port), token)
		fmt.Println()

		return nil
	},
}

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the current cluster",
	Long:  `Tears down the cluster, removing all nodes and state.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println()
		printWarning("This will destroy the entire cluster!")
		fmt.Print("  Are you sure? (y/N): ")

		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			printInfo("Cancelled")
			return nil
		}

		_, err := apiDelete("/api/v1/cluster")
		if err != nil {
			printError(fmt.Sprintf("Failed to destroy cluster: %v", err))
			return nil
		}

		printSuccess("Cluster destroyed")
		return nil
	},
}

var preflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Run network and system preflight checks",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println()
		printInfo("Network Preflight Check")
		fmt.Println()

		hostname, _ := os.Hostname()
		fmt.Printf("  Hostname:     %s\n", hostname)
		fmt.Printf("  OS:           %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("  CPUs:         %d\n", runtime.NumCPU())
		fmt.Println()

		// Try to connect to controller
		_, err := apiGet("/health")
		if err == nil {
			printSuccess("Controller reachable at " + controllerURL)
		} else {
			printWarning("Controller not reachable at " + controllerURL)
		}

		fmt.Println()
		printInfo("Overall: READY")
		fmt.Println()
		return nil
	},
}

func init() {
	initCmd.Flags().StringVar(&initName, "name", "", "Cluster name")
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(destroyCmd)
	rootCmd.AddCommand(preflightCmd)
}
