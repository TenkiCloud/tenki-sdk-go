package sandbox

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	sandboxv1 "github.com/TenkiCloud/tenki-sdk-go/sandbox/internal/proto/tenki/sandbox/v1"
	"github.com/TenkiCloud/tenki-sdk-go/sandbox/internal/proto/tenki/sandbox/v1/sandboxv1connect"
)

type waitSessionHandler struct {
	sandboxv1connect.UnimplementedSandboxServiceHandler
	getCalls      atomic.Int32
	waitErr       error
	terminalError string
}

func (h *waitSessionHandler) CreateSession(context.Context, *connect.Request[sandboxv1.CreateSessionRequest]) (*connect.Response[sandboxv1.CreateSessionResponse], error) {
	return connect.NewResponse(&sandboxv1.CreateSessionResponse{Session: &sandboxv1.SandboxSession{
		Id:        "019e84bc-6df8-765f-8507-2734f87156c7",
		State:     sandboxv1.SessionState_SESSION_STATE_CREATING,
		OwnerType: "SERVICE",
		OwnerId:   "self",
	}}), nil
}

func (h *waitSessionHandler) GetSession(context.Context, *connect.Request[sandboxv1.GetSessionRequest]) (*connect.Response[sandboxv1.GetSessionResponse], error) {
	h.getCalls.Add(1)
	return connect.NewResponse(&sandboxv1.GetSessionResponse{Session: &sandboxv1.SandboxSession{
		Id:        "019e84bc-6df8-765f-8507-2734f87156c7",
		State:     sandboxv1.SessionState_SESSION_STATE_RUNNING,
		OwnerType: "SERVICE",
		OwnerId:   "self",
	}}), nil
}

func (h *waitSessionHandler) WaitSession(_ context.Context, req *connect.Request[sandboxv1.WaitSessionRequest], stream *connect.ServerStream[sandboxv1.WaitSessionResponse]) error {
	if err := stream.Send(&sandboxv1.WaitSessionResponse{Session: &sandboxv1.SandboxSession{
		Id:        req.Msg.SessionId,
		State:     sandboxv1.SessionState_SESSION_STATE_CREATING,
		OwnerType: "SERVICE",
		OwnerId:   "self",
	}}); err != nil {
		return err
	}
	if h.waitErr != nil {
		return h.waitErr
	}
	if h.terminalError != "" {
		return stream.Send(&sandboxv1.WaitSessionResponse{Session: &sandboxv1.SandboxSession{
			Id:            req.Msg.SessionId,
			State:         sandboxv1.SessionState_SESSION_STATE_TERMINATED,
			OwnerType:     "SERVICE",
			OwnerId:       "self",
			TerminalError: h.terminalError,
		}})
	}
	return stream.Send(&sandboxv1.WaitSessionResponse{Session: &sandboxv1.SandboxSession{
		Id:        req.Msg.SessionId,
		State:     sandboxv1.SessionState_SESSION_STATE_RUNNING,
		OwnerType: "SERVICE",
		OwnerId:   "self",
	}})
}

func TestCreateAndWaitUsesWaitSessionStream(t *testing.T) {
	t.Parallel()

	handler := &waitSessionHandler{}
	server, client := newWaitSessionTestServer(t, handler)
	defer server.Close()

	session, err := client.CreateAndWait(context.Background(), time.Second, WithName("wait-stream"))
	if err != nil {
		t.Fatalf("CreateAndWait: %v", err)
	}
	if session.State != SessionStateRunning {
		t.Fatalf("state = %s, want %s", session.State, SessionStateRunning)
	}
	if got := handler.getCalls.Load(); got != 0 {
		t.Fatalf("GetSession calls = %d, want 0", got)
	}
}

func TestCreateAndWaitIncludesTerminalError(t *testing.T) {
	t.Parallel()

	handler := &waitSessionHandler{terminalError: "sandbox provisioned but readiness check failed"}
	server, client := newWaitSessionTestServer(t, handler)
	defer server.Close()

	_, err := client.CreateAndWait(context.Background(), time.Second, WithName("wait-stream"))
	if err == nil {
		t.Fatal("CreateAndWait returned nil error")
	}
	if !strings.Contains(err.Error(), "sandbox provisioned but readiness check failed") {
		t.Fatalf("error = %q, want terminal_error", err)
	}
}

func TestCreateAndWaitFallsBackToPollOnRetryableWaitStreamError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{name: "unavailable", err: connect.NewError(connect.CodeUnavailable, errors.New("transport closing"))},
		{name: "deadline", err: connect.NewError(connect.CodeDeadlineExceeded, context.DeadlineExceeded)},
		{name: "internal stream reset", err: connect.NewError(connect.CodeInternal, errors.New("stream reset: INTERNAL_ERROR"))},
		{name: "internal timeout", err: connect.NewError(connect.CodeInternal, errors.New("upstream request timeout"))},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := &waitSessionHandler{waitErr: tc.err}
			server, client := newWaitSessionTestServer(t, handler)
			defer server.Close()

			session, err := client.CreateAndWait(context.Background(), time.Second, WithName("wait-stream"))
			if err != nil {
				t.Fatalf("CreateAndWait: %v", err)
			}
			if session.State != SessionStateRunning {
				t.Fatalf("state = %s, want %s", session.State, SessionStateRunning)
			}
			if got := handler.getCalls.Load(); got == 0 {
				t.Fatalf("GetSession calls = %d, want fallback polling", got)
			}
		})
	}
}

func newWaitSessionTestServer(t *testing.T, h sandboxv1connect.SandboxServiceHandler) (*httptest.Server, *Client) {
	t.Helper()
	mux := http.NewServeMux()
	path, handler := sandboxv1connect.NewSandboxServiceHandler(h)
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	client, err := New(WithAuthToken("tk_test_api_key"), WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		server.Close()
		t.Fatalf("New client: %v", err)
	}
	return server, client
}
