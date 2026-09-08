package temporal

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/api/enums/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/converter"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestClientDecodesHistoryWithRemoteCodec(t *testing.T) {
	encoded := encodeTestPayloads(t, "decoded input")
	service := &codecTestWorkflowService{history: &workflowservice.GetWorkflowExecutionHistoryResponse{
		History: &historypb.History{Events: []*historypb.HistoryEvent{{
			EventId:   1,
			EventType: enums.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED,
			EventTime: timestamppb.Now(),
			Attributes: &historypb.HistoryEvent_WorkflowExecutionStartedEventAttributes{
				WorkflowExecutionStartedEventAttributes: &historypb.WorkflowExecutionStartedEventAttributes{
					Input: encoded,
				},
			},
		}}},
	}}
	address := startCodecTestTemporalServer(t, service)

	var requests []codecTestRequest
	var requestMu sync.Mutex
	codecHandler := converter.NewPayloadCodecHTTPHandler(
		converter.NewZlibCodec(converter.ZlibCodecOptions{AlwaysEncode: true}),
	)
	codecServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMu.Lock()
		requests = append(requests, codecTestRequest{
			method:        r.Method,
			path:          r.URL.Path,
			authorization: r.Header.Get("Authorization"),
			namespace:     r.Header.Get("X-Namespace"),
			customHeader:  r.Header.Get("X-Codec-Tenant"),
		})
		requestMu.Unlock()
		codecHandler.ServeHTTP(w, r)
	}))
	t.Cleanup(codecServer.Close)

	client, err := NewClient(t.Context(), ConnectionConfig{
		Address:       address,
		Namespace:     "profile-default",
		CodecEndpoint: codecServer.URL + "/{namespace}",
		CodecAuth:     "Bearer secret-token",
		CodecHeaders:  map[string]string{"X-Codec-Tenant": "payments"},
	})
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	events, err := client.GetEnhancedWorkflowHistory(t.Context(), "testing", "workflow-id", "run-id")
	if err != nil {
		t.Fatalf("get workflow history: %v", err)
	}
	if got, want := len(events), 1; got != want {
		t.Fatalf("history event count = %d, want %d", got, want)
	}
	if got, want := events[0].Input, `"decoded input"`; got != want {
		t.Fatalf("decoded input = %q, want %q", got, want)
	}

	requestMu.Lock()
	if got, want := len(requests), 1; got != want {
		requestMu.Unlock()
		t.Fatalf("codec request count = %d, want %d", got, want)
	}
	gotRequest := requests[0]
	requestMu.Unlock()
	if got, want := gotRequest.method, http.MethodPost; got != want {
		t.Fatalf("codec method = %q, want %q", got, want)
	}
	if got, want := gotRequest.path, "/testing/decode"; got != want {
		t.Fatalf("codec path = %q, want %q", got, want)
	}
	if got, want := gotRequest.authorization, "Bearer secret-token"; got != want {
		t.Fatalf("codec authorization = %q, want %q", got, want)
	}
	if got, want := gotRequest.namespace, "testing"; got != want {
		t.Fatalf("codec namespace = %q, want %q", got, want)
	}
	if got, want := gotRequest.customHeader, "payments"; got != want {
		t.Fatalf("custom codec header = %q, want %q", got, want)
	}
}

func TestClientEncodesSignalWithRemoteCodec(t *testing.T) {
	service := &codecTestWorkflowService{}
	address := startCodecTestTemporalServer(t, service)

	var requests []codecTestRequest
	var requestMu sync.Mutex
	codecHandler := converter.NewPayloadCodecHTTPHandler(
		converter.NewZlibCodec(converter.ZlibCodecOptions{AlwaysEncode: true}),
	)
	codecServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMu.Lock()
		requests = append(requests, codecTestRequest{
			method:       r.Method,
			path:         r.URL.Path,
			namespace:    r.Header.Get("X-Namespace"),
			customHeader: r.Header.Get("X-Codec-Tenant"),
		})
		requestMu.Unlock()
		codecHandler.ServeHTTP(w, r)
	}))
	t.Cleanup(codecServer.Close)

	client, err := NewClient(t.Context(), ConnectionConfig{
		Address:       address,
		Namespace:     "testing",
		CodecEndpoint: codecServer.URL + "/{namespace}",
		CodecHeaders:  map[string]string{"X-Codec-Tenant": "payments"},
	})
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	if err := client.SignalWorkflow(
		t.Context(), "testing", "workflow-id", "run-id", "approve", []byte(`{"approved":true}`),
	); err != nil {
		t.Fatalf("signal workflow: %v", err)
	}

	requestMu.Lock()
	if got, want := len(requests), 1; got != want {
		requestMu.Unlock()
		t.Fatalf("codec request count = %d, want %d", got, want)
	}
	gotRequest := requests[0]
	requestMu.Unlock()
	if got, want := gotRequest.path, "/testing/encode"; got != want {
		t.Fatalf("codec path = %q, want %q", got, want)
	}
	if got, want := gotRequest.customHeader, "payments"; got != want {
		t.Fatalf("custom codec header = %q, want %q", got, want)
	}
	if service.signal == nil || service.signal.GetInput() == nil || len(service.signal.GetInput().GetPayloads()) != 1 {
		t.Fatalf("Temporal service did not receive signal input: %#v", service.signal)
	}
	if got, want := string(service.signal.GetInput().GetPayloads()[0].GetMetadata()["encoding"]), "binary/zlib"; got != want {
		t.Fatalf("signal payload encoding = %q, want %q", got, want)
	}
}

func TestClientReconnectUsesNewCodecConfiguration(t *testing.T) {
	encoded := encodeTestPayloads(t, "decoded input")
	service := &codecTestWorkflowService{history: &workflowservice.GetWorkflowExecutionHistoryResponse{
		History: &historypb.History{Events: []*historypb.HistoryEvent{{
			EventId:   1,
			EventType: enums.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED,
			EventTime: timestamppb.Now(),
			Attributes: &historypb.HistoryEvent_WorkflowExecutionStartedEventAttributes{
				WorkflowExecutionStartedEventAttributes: &historypb.WorkflowExecutionStartedEventAttributes{
					Input: encoded,
				},
			},
		}}},
	}}
	address := startCodecTestTemporalServer(t, service)

	codecHandler := converter.NewPayloadCodecHTTPHandler(
		converter.NewZlibCodec(converter.ZlibCodecOptions{AlwaysEncode: true}),
	)
	var firstPath, secondPath string
	firstCodecServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstPath = r.URL.Path
		codecHandler.ServeHTTP(w, r)
	}))
	t.Cleanup(firstCodecServer.Close)
	secondCodecServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondPath = r.URL.Path
		codecHandler.ServeHTTP(w, r)
	}))
	t.Cleanup(secondCodecServer.Close)

	client, err := NewClient(t.Context(), ConnectionConfig{
		Address:       address,
		Namespace:     "first",
		CodecEndpoint: firstCodecServer.URL + "/{namespace}",
	})
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if _, err := client.GetEnhancedWorkflowHistory(t.Context(), "first", "workflow-id", "run-id"); err != nil {
		t.Fatalf("get workflow history before reconnect: %v", err)
	}

	if err := client.ReconnectWithConfig(t.Context(), ConnectionConfig{
		Address:       address,
		Namespace:     "second",
		CodecEndpoint: secondCodecServer.URL + "/{namespace}",
	}); err != nil {
		t.Fatalf("reconnect client: %v", err)
	}
	if _, err := client.GetEnhancedWorkflowHistory(t.Context(), "second", "workflow-id", "run-id"); err != nil {
		t.Fatalf("get workflow history after reconnect: %v", err)
	}

	if got, want := firstPath, "/first/decode"; got != want {
		t.Fatalf("first codec path = %q, want %q", got, want)
	}
	if got, want := secondPath, "/second/decode"; got != want {
		t.Fatalf("second codec path = %q, want %q", got, want)
	}
}

type codecTestRequest struct {
	method        string
	path          string
	authorization string
	namespace     string
	customHeader  string
}

type codecTestWorkflowService struct {
	workflowservice.UnimplementedWorkflowServiceServer
	history *workflowservice.GetWorkflowExecutionHistoryResponse
	signal  *workflowservice.SignalWorkflowExecutionRequest
}

func (s *codecTestWorkflowService) SignalWorkflowExecution(
	_ context.Context,
	req *workflowservice.SignalWorkflowExecutionRequest,
) (*workflowservice.SignalWorkflowExecutionResponse, error) {
	s.signal = req
	return &workflowservice.SignalWorkflowExecutionResponse{}, nil
}

func (s *codecTestWorkflowService) GetSystemInfo(
	context.Context,
	*workflowservice.GetSystemInfoRequest,
) (*workflowservice.GetSystemInfoResponse, error) {
	return &workflowservice.GetSystemInfoResponse{}, nil
}

func (s *codecTestWorkflowService) GetWorkflowExecutionHistory(
	context.Context,
	*workflowservice.GetWorkflowExecutionHistoryRequest,
) (*workflowservice.GetWorkflowExecutionHistoryResponse, error) {
	return s.history, nil
}

func startCodecTestTemporalServer(t *testing.T, service workflowservice.WorkflowServiceServer) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for Temporal test server: %v", err)
	}
	server := grpc.NewServer()
	workflowservice.RegisterWorkflowServiceServer(server, service)
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	return listener.Addr().String()
}

func encodeTestPayloads(t *testing.T, value string) *commonpb.Payloads {
	t.Helper()

	dataConverter := converter.NewCodecDataConverter(
		converter.GetDefaultDataConverter(),
		converter.NewZlibCodec(converter.ZlibCodecOptions{AlwaysEncode: true}),
	)
	payloads, err := dataConverter.ToPayloads(value)
	if err != nil {
		t.Fatalf("encode test payload: %v", err)
	}
	return payloads
}
