package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var (
	controllerURL string
	apiClient     *http.Client
)

var rootCmd = &cobra.Command{
	Use:   "constellation",
	Short: "Constellation — Distributed Compute Platform",
	Long: `
  ✦ Constellation — Distributed Compute Platform ✦

  Turn any collection of Linux machines into a unified
  computation pool. Submit tasks, split workloads across
  nodes, and monitor everything from the CLI or dashboard.

  Get started:
    constellation init --name "my-cluster"
    constellation join --controller 192.168.1.10
    constellation status
    constellation run --cmd "python train.py" --cpu 4`,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&controllerURL, "controller-url", "http://localhost:8080", "Controller API URL")
	apiClient = &http.Client{Timeout: 30 * time.Second}
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func getToken() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	tokenBytes, err := os.ReadFile(filepath.Join(home, ".constellation", "token"))
	if err != nil {
		return ""
	}
	return string(tokenBytes)
}

func doRequest(method, path string, data interface{}) (map[string]interface{}, error) {
	var bodyReader io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, controllerURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if token := getToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := apiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to controller: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		// Attempting to parse as plain text if it's not JSON
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
		}
		return map[string]interface{}{"message": string(body)}, nil
	}

	if resp.StatusCode >= 400 {
		if errMsg, ok := result["error"]; ok {
			return nil, fmt.Errorf("API error: %s", errMsg)
		}
		return nil, fmt.Errorf("API error: HTTP %d", resp.StatusCode)
	}

	return result, nil
}

func apiGet(path string) (map[string]interface{}, error) {
	return doRequest("GET", path, nil)
}

func apiPost(path string, data interface{}) (map[string]interface{}, error) {
	return doRequest("POST", path, data)
}

func apiDelete(path string) (map[string]interface{}, error) {
	return doRequest("DELETE", path, nil)
}

func apiPut(path string, data interface{}) (map[string]interface{}, error) {
	return doRequest("PUT", path, data)
}

// ─── Formatting Helpers ──────────────────────────────────────────────────────

func printSuccess(msg string) {
	fmt.Printf("  ✓ %s\n", msg)
}

func printError(msg string) {
	fmt.Printf("  ✗ %s\n", msg)
}

func printWarning(msg string) {
	fmt.Printf("  ⚠ %s\n", msg)
}

func printInfo(msg string) {
	fmt.Printf("  ✦ %s\n", msg)
}

func formatBytes(b float64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%.0f B", b)
	}
	div := float64(unit)
	exp := 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", b/div, "KMGTPE"[exp])
}
