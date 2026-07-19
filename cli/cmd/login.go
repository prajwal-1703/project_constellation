package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	loginUsername string
	loginPassword string
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to the Constellation cluster",
	Run: func(cmd *cobra.Command, args []string) {
		if loginUsername == "" || loginPassword == "" {
			fmt.Println("Error: username and password are required")
			os.Exit(1)
		}

		req := map[string]string{
			"username": loginUsername,
			"password": loginPassword,
		}

		resp, err := apiPost("/api/v1/users/login", req)
		if err != nil {
			fmt.Println("Login failed:", err)
			os.Exit(1)
		}

		token, ok := resp["token"].(string)
		if !ok {
			fmt.Println("Login failed: no token in response")
			os.Exit(1)
		}

		home, _ := os.UserHomeDir()
		configDir := filepath.Join(home, ".constellation")
		os.MkdirAll(configDir, 0755)
		
		err = os.WriteFile(filepath.Join(configDir, "token"), []byte(token), 0600)
		if err != nil {
			fmt.Println("Failed to save token:", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Logged in as %s\n", resp["username"])
	},
}

func init() {
	loginCmd.Flags().StringVarP(&loginUsername, "username", "u", "", "Username")
	loginCmd.Flags().StringVarP(&loginPassword, "password", "p", "", "Password")
	rootCmd.AddCommand(loginCmd)
}
