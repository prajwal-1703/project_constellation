package api

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/constellation/controller/state"
	pb "github.com/constellation/controller/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type GRPCServer struct {
	pb.UnimplementedNodeAgentServer
	Store *state.Store
	WSHub *WebSocketHub
}

func NewGRPCServer(store *state.Store, wsHub *WebSocketHub) *GRPCServer {
	return &GRPCServer{
		Store: store,
		WSHub: wsHub,
	}
}

func (s *GRPCServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	nodeID := req.NodeId

	// Update node status to online
	if err := s.Store.UpdateNodeHeartbeat(nodeID); err != nil {
		log.Printf("Failed to update heartbeat for %s: %v", nodeID, err)
		return nil, err
	}

	// Broadcast
	s.WSHub.BroadcastEvent("node_status_changed", map[string]interface{}{
		"node_id": nodeID,
		"status":  "online",
	})

	return &pb.HeartbeatResponse{
		// agent.proto HeartbeatResponse has no `received` field
	}, nil
}

func StartGRPCServer(addr, certFile, keyFile string, store *state.Store, wsHub *WebSocketHub) error {
	var opts []grpc.ServerOption

	if certFile != "" && keyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS keys: %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}

	grpcServer := grpc.NewServer(opts...)
	srv := NewGRPCServer(store, wsHub)
	pb.RegisterNodeAgentServer(grpcServer, srv)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", addr, err)
	}

	log.Printf("gRPC server listening on %s (TLS: %v)", addr, certFile != "")
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()
	return nil
}
