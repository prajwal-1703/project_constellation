package discovery

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/hashicorp/mdns"
)

// MDNSService handles mDNS advertisement and scanning.
type MDNSService struct {
	server    *mdns.Server
	clusterID string
	clusterName string
	host      string
	port      int
}

// NewMDNSService creates a new mDNS discovery service.
func NewMDNSService(clusterID, clusterName, host string, port int) *MDNSService {
	return &MDNSService{
		clusterID:   clusterID,
		clusterName: clusterName,
		host:        host,
		port:        port,
	}
}

// Advertise starts broadcasting this controller on the local network.
func (m *MDNSService) Advertise() error {
	hostname, _ := os.Hostname()
	info := []string{
		fmt.Sprintf("cluster_id=%s", m.clusterID),
		fmt.Sprintf("cluster_name=%s", m.clusterName),
		fmt.Sprintf("host=%s", m.host),
		fmt.Sprintf("port=%d", m.port),
	}

	service, err := mdns.NewMDNSService(
		hostname,
		"_constellation-ctrl._tcp",
		"",
		"",
		m.port,
		[]net.IP{net.ParseIP(m.host)},
		info,
	)
	if err != nil {
		return fmt.Errorf("failed to create mDNS service: %w", err)
	}

	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return fmt.Errorf("failed to start mDNS server: %w", err)
	}

	m.server = server
	log.Printf("mDNS: advertising cluster '%s' on %s:%d", m.clusterName, m.host, m.port)
	return nil
}

// ScanForControllers looks for Constellation controllers on the local network.
func ScanForControllers() ([]DiscoveredController, error) {
	entriesCh := make(chan *mdns.ServiceEntry, 10)
	var controllers []DiscoveredController

	go func() {
		for entry := range entriesCh {
			ctrl := DiscoveredController{
				Host:      entry.AddrV4.String(),
				Port:      entry.Port,
				Hostname:  entry.Host,
			}

			// Parse TXT records
			for _, txt := range entry.InfoFields {
				parts := strings.SplitN(txt, "=", 2)
				if len(parts) == 2 {
					switch parts[0] {
					case "cluster_id":
						ctrl.ClusterID = parts[1]
					case "cluster_name":
						ctrl.ClusterName = parts[1]
					}
				}
			}

			controllers = append(controllers, ctrl)
		}
	}()

	params := mdns.DefaultParams("_constellation-ctrl._tcp")
	params.Entries = entriesCh
	params.Timeout = 3 * 1e9 // 3 seconds

	if err := mdns.Query(params); err != nil {
		return nil, fmt.Errorf("mDNS query failed: %w", err)
	}
	close(entriesCh)

	return controllers, nil
}

// Stop shuts down the mDNS service.
func (m *MDNSService) Stop() {
	if m.server != nil {
		m.server.Shutdown()
		log.Println("mDNS: service stopped")
	}
}

// DiscoveredController represents a controller found on the network.
type DiscoveredController struct {
	ClusterID   string `json:"cluster_id"`
	ClusterName string `json:"cluster_name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Hostname    string `json:"hostname"`
}
