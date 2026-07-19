package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

var dashboardPort int

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Open the web dashboard in a browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		url := fmt.Sprintf("%s", controllerURL)
		fmt.Println()
		printInfo(fmt.Sprintf("Dashboard available at %s", url))
		printInfo("Opening in browser...")
		fmt.Println()

		// Open browser
		var openCmd *exec.Cmd
		switch runtime.GOOS {
		case "windows":
			openCmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		case "darwin":
			openCmd = exec.Command("open", url)
		default:
			openCmd = exec.Command("xdg-open", url)
		}
		openCmd.Start()

		return nil
	},
}

func init() {
	dashboardCmd.Flags().IntVar(&dashboardPort, "port", 8080, "Dashboard port")
	rootCmd.AddCommand(dashboardCmd)
}
