package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/constellation/controller/api"
	"github.com/constellation/controller/discovery"
	"github.com/constellation/controller/scheduler"
	"github.com/constellation/controller/state"
)

func main() {
	// ── Flags ────────────────────────────────────────────────────────────
	httpAddr := flag.String("http", ":8080", "HTTP API listen address")
	grpcAddr := flag.String("grpc", ":9090", "gRPC listen address")
	dataDir := flag.String("data", getDefaultDataDir(), "Data directory path")
	flag.Parse()

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("╔══════════════════════════════════════════════╗")
	log.Println("║       ✦ Constellation Controller ✦          ║")
	log.Println("║       Distributed Compute Platform           ║")
	log.Println("╚══════════════════════════════════════════════╝")

	// ── Data Directory ───────────────────────────────────────────────────
	os.MkdirAll(*dataDir, 0755)
	os.MkdirAll(filepath.Join(*dataDir, "tasks"), 0755)
	os.MkdirAll(filepath.Join(*dataDir, "agent-binaries"), 0755)

	// ── State Store (SQLite) ─────────────────────────────────────────────
	dbPath := filepath.Join(*dataDir, "constellation.db")
	store, err := state.NewStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize state store: %v", err)
	}
	defer store.Close()
	log.Printf("State store initialized: %s", dbPath)

	// ── API Server ───────────────────────────────────────────────────────
	server := api.NewServer(store)

	// ── Scheduler ────────────────────────────────────────────────────────
	sched := scheduler.NewScheduler(store, server.WSHub)
	sched.Start()
	defer sched.Stop()
	log.Println("Scheduler started")

	// ── mDNS Discovery ───────────────────────────────────────────────────
	cluster, _ := store.GetCluster()
	if cluster != nil {
		mdnsSvc := discovery.NewMDNSService(
			cluster.ID, cluster.Name,
			cluster.ControllerHost, cluster.ControllerPort,
		)
		if err := mdnsSvc.Advertise(); err != nil {
			log.Printf("Warning: mDNS advertisement failed: %v", err)
		} else {
			defer mdnsSvc.Stop()
		}
	}

	// ── Graceful Shutdown ────────────────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down...")
		sched.Stop()
		store.Close()
		os.Exit(0)
	}()

	// ── Start HTTP Server ────────────────────────────────────────────────
	log.Printf("HTTP API: http://0.0.0.0%s", *httpAddr)
	log.Printf("gRPC:     %s", *grpcAddr)
	log.Printf("Data dir: %s", *dataDir)
	log.Println("─────────────────────────────────────────────")

	if err := api.StartGRPCServer(*grpcAddr, "../certs/server.crt", "../certs/server.key", store, server.WSHub); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}

	if err := server.Start(*httpAddr, "", ""); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

func getDefaultDataDir() string {
	// On Windows, use user's app data; on Linux, use /var/lib
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".constellation")
}
