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
		})
		requestMu.Unlock()
		codecHandler.ServeHTTP(w, r)
	}))
	t.Cleanup(codecServer.Close)

	client, err := NewClient(t.Context(), ConnectionConfig{
		Address:       address,
		Namespace:     "testing",
		CodecEndpoint: codecServer.URL + "/{namespace}",
		CodecAuth:     "Bearer secret-token",
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
}

type codecTestRequest struct {
	method        string
	path          string
	authorization string
	namespace     string
}

type codecTestWorkflowService struct {
	workflowservice.UnimplementedWorkflowServiceServer
	history *workflowservice.GetWorkflowExecutionHistoryResponse
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
