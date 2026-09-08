package temporal

import (
	"context"
	"net"
	"testing"

	"go.temporal.io/api/workflowservice/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

const temporalWorkflowServiceName = "temporal.api.workflowservice.v1.WorkflowService"

func TestCheckConnectionSucceedsWhenHealthIsServingAndNamespaceListingIsForbidden(t *testing.T) {
	healthServer := health.NewServer()
	healthServer.SetServingStatus(temporalWorkflowServiceName, healthpb.HealthCheckResponse_SERVING)
	address := startConnectionTestTemporalServer(t, &connectionTestWorkflowService{
		listNamespacesErr: status.Error(codes.PermissionDenied, "request unauthorized"),
	}, healthServer)

	client, err := NewClient(t.Context(), ConnectionConfig{
		Address:   address,
		Namespace: "namespace-scoped",
	})
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if err := client.CheckConnection(t.Context()); err != nil {
		t.Fatalf("check connection: %v", err)
	}
	if !client.IsConnected() {
		t.Fatal("client should remain connected when health reports serving")
	}
}

func TestCheckConnectionFailsWhenHealthIsNotServingAndNamespaceListingSucceeds(t *testing.T) {
	healthServer := health.NewServer()
	healthServer.SetServingStatus(temporalWorkflowServiceName, healthpb.HealthCheckResponse_SERVING)
	address := startConnectionTestTemporalServer(t, &connectionTestWorkflowService{}, healthServer)

	client, err := NewClient(t.Context(), ConnectionConfig{
		Address:   address,
		Namespace: "namespace-scoped",
	})
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	healthServer.SetServingStatus(temporalWorkflowServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
	if err := client.CheckConnection(t.Context()); err == nil {
		t.Fatal("check connection should fail when health reports not serving")
	}
	if client.IsConnected() {
		t.Fatal("client should be disconnected when health reports not serving")
	}
}

type connectionTestWorkflowService struct {
	workflowservice.UnimplementedWorkflowServiceServer
	listNamespacesErr error
}

func (s *connectionTestWorkflowService) GetSystemInfo(
	context.Context,
	*workflowservice.GetSystemInfoRequest,
) (*workflowservice.GetSystemInfoResponse, error) {
	return &workflowservice.GetSystemInfoResponse{}, nil
}

func (s *connectionTestWorkflowService) ListNamespaces(
	context.Context,
	*workflowservice.ListNamespacesRequest,
) (*workflowservice.ListNamespacesResponse, error) {
	if s.listNamespacesErr != nil {
		return nil, s.listNamespacesErr
	}
	return &workflowservice.ListNamespacesResponse{}, nil
}

func startConnectionTestTemporalServer(
	t *testing.T,
	workflowService workflowservice.WorkflowServiceServer,
	healthService *health.Server,
) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for Temporal test server: %v", err)
	}
	server := grpc.NewServer()
	workflowservice.RegisterWorkflowServiceServer(server, workflowService)
	healthpb.RegisterHealthServer(server, healthService)
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	return listener.Addr().String()
}
