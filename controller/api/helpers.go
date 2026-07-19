package api

import (
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/google/uuid"
)

// detectLocalIP returns the primary non-loopback IP address.
func detectLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

// getHostname returns the machine hostname.
func getHostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return name
}

// generateNodeID creates a short unique node ID.
func generateNodeID() string {
	return "node-" + uuid.New().String()[:8]
}

// hashPassword creates a SHA-256 hash of the password.
// In production, use bcrypt — this is for MVP simplicity.
func hashPassword(password string) string {
	h := sha256.Sum256([]byte(password))
	return fmt.Sprintf("%x", h)
}

// checkPassword verifies a password against its hash.
func checkPassword(password, hash string) bool {
	return hashPassword(password) == hash
}

// parseMemorySize parses sizes like "4GB", "512MB" into bytes.
func parseMemorySize(s string) int64 {
	s = strings.TrimSpace(strings.ToUpper(s))
	var value float64
	var unit string

	fmt.Sscanf(s, "%f%s", &value, &unit)

	switch unit {
	case "TB":
		return int64(value * 1024 * 1024 * 1024 * 1024)
	case "GB":
		return int64(value * 1024 * 1024 * 1024)
	case "MB":
		return int64(value * 1024 * 1024)
	case "KB":
		return int64(value * 1024)
	case "B", "":
		return int64(value)
	default:
		return int64(value)
	}
}

// formatBytes formats bytes into human-readable string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
