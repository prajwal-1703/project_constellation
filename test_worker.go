//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const controller = "http://localhost:8080"
const token = "cst_ce7c8da3-d89a-4b" 

type JoinRequest struct {
	Hostname   string `json:"hostname"`
	Role       string `json:"role"`
	JoinToken  string `json:"join_token"`
	IPAddress  string `json:"ip_address"`
	CPUModel   string `json:"cpu_model"`
	CPUCores   int    `json:"cpu_cores"`
	MemoryTotal int   `json:"memory_total"`
}

func main() {
	fmt.Println("Starting mock worker node...")
	
	joinReq := JoinRequest{
		Hostname: "mock-worker-1",
		Role: "worker",
		JoinToken: token,
		IPAddress: "127.0.0.1",
		CPUModel: "Mock CPU",
		CPUCores: 4,
		MemoryTotal: 8 * 1024 * 1024 * 1024,
	}
	b, _ := json.Marshal(joinReq)
	
	resp, err := http.Post(controller+"/api/v1/nodes/join", "application/json", bytes.NewBuffer(b))
	if err != nil {
		fmt.Println("Failed to join:", err)
		return
	}
	defer resp.Body.Close()
	
	var joinResp struct{ NodeID string `json:"node_id"` }
	json.NewDecoder(resp.Body).Decode(&joinResp)
	fmt.Println("Joined as:", joinResp.NodeID)

	for {
		hb := map[string]interface{}{
			"status": "online",
			"metrics": map[string]float64{
				"cpu_usage": 15.0,
				"memory_usage": 45.0,
				"running_tasks": 0,
			},
		}
		hbBytes, _ := json.Marshal(hb)
		req, _ := http.NewRequest("PUT", controller+"/api/v1/nodes/"+joinResp.NodeID+"/status", bytes.NewBuffer(hbBytes))
		req.Header.Set("Content-Type", "application/json")
		http.DefaultClient.Do(req)

		fmt.Println("Heartbeat sent.")
		time.Sleep(3 * time.Second)
	}
}
