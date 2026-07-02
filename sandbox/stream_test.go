package sandbox

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	sandboxv1 "github.com/TenkiCloud/tenki-sdk-go/sandbox/internal/proto/tenki/sandbox/v1"
	"github.com/TenkiCloud/tenki-sdk-go/sandbox/internal/proto/tenki/sandbox/v1/sandboxv1connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type streamTestHandler struct {
	sandboxv1connect.UnimplementedSandboxServiceHandler

	pauseSessionFn  func(*connect.Request[sandboxv1.PauseSessionRequest]) (*connect.Response[sandboxv1.PauseSessionResponse], error)
	resumeSessionFn func(*connect.Request[sandboxv1.ResumeSessionRequest]) (*connect.Response[sandboxv1.ResumeSessionResponse], error)
}

func (h *streamTestHandler) PauseSession(_ context.Context, req *connect.Request[sandboxv1.PauseSessionRequest]) (*connect.Response[sandboxv1.PauseSessionResponse], error) {
	if h.pauseSessionFn != nil {
		return h.pauseSessionFn(req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *streamTestHandler) ResumeSession(_ context.Context, req *connect.Request[sandboxv1.ResumeSessionRequest]) (*connect.Response[sandboxv1.ResumeSessionResponse], error) {
	if h.resumeSessionFn != nil {
		return h.resumeSessionFn(req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

type dataPlaneRunTestHandler struct {
	sandboxv1connect.UnimplementedSandboxSessionDataPlaneServiceHandler
	t *testing.T

	starts    []*sandboxv1.RunStart
	statPaths []string
}

func (h *dataPlaneRunTestHandler) Stat(
	_ context.Context,
	req *connect.Request[sandboxv1.SandboxSessionDataPlaneServiceStatRequest],
) (*connect.Response[sandboxv1.SandboxSessionDataPlaneServiceStatResponse], error) {
	h.statPaths = append(h.statPaths, req.Msg.GetRequest().GetPath())
	return connect.NewResponse(&sandboxv1.SandboxSessionDataPlaneServiceStatResponse{
		Response: &sandboxv1.StatResponse{Exists: true, IsDir: true},
	}), nil
}

func (h *dataPlaneRunTestHandler) Run(
	ctx context.Context,
	stream *connect.BidiStream[sandboxv1.SandboxSessionDataPlaneServiceRunRequest, sandboxv1.SandboxSessionDataPlaneServiceRunResponse],
) error {
	first, err := stream.Receive()
	if err != nil {
		return err
	}
	start := first.GetFrame().GetStart()
	if start == nil {
		h.t.Fatal("first run frame must be start")
	}
	h.starts = append(h.starts, start)
	if err := stream.Send(&sandboxv1.SandboxSessionDataPlaneServiceRunResponse{Frame: &sandboxv1.RunResponse{
		Payload: &sandboxv1.RunResponse_Started{Started: &sandboxv1.RunStarted{Pid: 42}},
	}}); err != nil {
		return err
	}
	if err := stream.Send(&sandboxv1.SandboxSessionDataPlaneServiceRunResponse{Frame: &sandboxv1.RunResponse{
		Payload: &sandboxv1.RunResponse_Stdout{Stdout: []byte("out")},
	}}); err != nil {
		return err
	}
	if err := stream.Send(&sandboxv1.SandboxSessionDataPlaneServiceRunResponse{Frame: &sandboxv1.RunResponse{
		Payload: &sandboxv1.RunResponse_Stderr{Stderr: []byte("err")},
	}}); err != nil {
		return err
	}
	return stream.Send(&sandboxv1.SandboxSessionDataPlaneServiceRunResponse{Frame: &sandboxv1.RunResponse{
		Payload: &sandboxv1.RunResponse_Exit{Exit: &sandboxv1.RunExit{ExitCode: 0, DurationMs: 12}},
	}})
}

func newStreamTestClient(t *testing.T, h *streamTestHandler) *Client {
	t.Helper()

	mux := http.NewServeMux()
	path, svc := sandboxv1connect.NewSandboxServiceHandler(h)
	mux.Handle(path, svc)

	server := httptest.NewUnstartedServer(mux)
	server.EnableHTTP2 = true
	server.StartTLS()
	t.Cleanup(server.Close)

	client, err := New(WithAuthToken("tk_test_api_key"), WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func newDataPlaneRunTestServer(t *testing.T) (*dataPlaneRunTestHandler, string) {
	t.Helper()

	handler := &dataPlaneRunTestHandler{t: t}
	mux := http.NewServeMux()
	path, svc := sandboxv1connect.NewSandboxSessionDataPlaneServiceHandler(handler)
	mux.Handle(path, svc)
	server := httptest.NewServer(h2c.NewHandler(mux, &http2.Server{}))
	t.Cleanup(server.Close)
	return handler, server.URL
}

func newStreamTestSession(client *Client, endpoint string) *Session {
	session := &Session{client: client, ID: "session-1"}
	session.configureDataPlane(endpoint, &sandboxv1.SessionCredential{
		Credential: "test-session-cert",
		ExpiresAt:  timestamppb.New(time.Now().Add(time.Hour)),
	})
	return session
}

func TestSessionStreamUsesDataPlaneRun(t *testing.T) {
	t.Parallel()

	runHandler, endpoint := newDataPlaneRunTestServer(t)
	client := newStreamTestClient(t, &streamTestHandler{})
	stream, err := newStreamTestSession(client, endpoint).Stream(
		context.Background(),
		"echo",
		WithArgs("hello"),
		WithEnv("FOO", "bar"),
		WithTimeout(time.Second),
	)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	first, err := stream.Next()
	if err != nil {
		t.Fatalf("first Next: %v", err)
	}
	second, err := stream.Next()
	if err != nil {
		t.Fatalf("second Next: %v", err)
	}
	outputs := []Output{first, second}
	if !hasOutput(outputs, "out", false) || !hasOutput(outputs, "err", true) {
		t.Fatalf("unexpected outputs: %#v", outputs)
	}

	if _, err := stream.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}

	result, err := stream.Wait()
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if result.ExitCode != 0 || result.Duration != 12*time.Millisecond || result.Status != CommandStatusSucceeded {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(runHandler.statPaths) != 1 || runHandler.statPaths[0] != "/home/tenki" {
		t.Fatalf("expected readiness stat on /home/tenki, got %#v", runHandler.statPaths)
	}
	if len(runHandler.starts) != 1 {
		t.Fatalf("expected one data-plane run start, got %d", len(runHandler.starts))
	}
	start := runHandler.starts[0]
	if start.GetSessionId() != "session-1" || start.GetCmd()[0] != "echo" || start.GetCmd()[1] != "hello" {
		t.Fatalf("unexpected data-plane run start: %#v", start)
	}
	if start.GetEnv()["FOO"] != "bar" || start.GetTimeoutMs() == 0 {
		t.Fatalf("unexpected data-plane run options: %#v", start)
	}
}

func hasOutput(outputs []Output, data string, stderr bool) bool {
	for _, output := range outputs {
		if string(output.Data) == data && output.IsStderr == stderr && !output.IsFinal {
			return true
		}
	}
	return false
}

func TestSessionExecAggregatesDataPlaneRunOutput(t *testing.T) {
	t.Parallel()

	_, endpoint := newDataPlaneRunTestServer(t)
	client := newStreamTestClient(t, &streamTestHandler{})
	var callbackOutputs []Output
	result, err := newStreamTestSession(client, endpoint).Exec(context.Background(), "echo", WithOnOutput(func(output Output) {
		callbackOutputs = append(callbackOutputs, output)
	}))
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if string(result.Stdout) != "out" || string(result.Stderr) != "err" {
		t.Fatalf("unexpected aggregated output: %#v", result)
	}
	if len(result.Outputs) != 2 || len(callbackOutputs) != 2 {
		t.Fatalf("unexpected outputs: result=%#v callback=%#v", result.Outputs, callbackOutputs)
	}
}

func TestSessionStreamRejectsWithOnOutput(t *testing.T) {
	t.Parallel()

	_, endpoint := newDataPlaneRunTestServer(t)
	client := newStreamTestClient(t, &streamTestHandler{})
	_, err := newStreamTestSession(client, endpoint).Stream(context.Background(), "echo", WithOnOutput(func(Output) {}))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSessionPauseResume(t *testing.T) {
	t.Parallel()

	h := &streamTestHandler{}
	h.pauseSessionFn = func(req *connect.Request[sandboxv1.PauseSessionRequest]) (*connect.Response[sandboxv1.PauseSessionResponse], error) {
		if req.Msg.GetSessionId() != "session-1" {
			t.Fatalf("unexpected pause session id: %q", req.Msg.GetSessionId())
		}
		return connect.NewResponse(&sandboxv1.PauseSessionResponse{Session: &sandboxv1.SandboxSession{Id: "session-1", State: sandboxv1.SessionState_SESSION_STATE_PAUSED}}), nil
	}
	h.resumeSessionFn = func(req *connect.Request[sandboxv1.ResumeSessionRequest]) (*connect.Response[sandboxv1.ResumeSessionResponse], error) {
		if req.Msg.GetSessionId() != "session-1" {
			t.Fatalf("unexpected resume session id: %q", req.Msg.GetSessionId())
		}
		return connect.NewResponse(&sandboxv1.ResumeSessionResponse{Session: &sandboxv1.SandboxSession{Id: "session-1", State: sandboxv1.SessionState_SESSION_STATE_RUNNING}}), nil
	}

	client := newStreamTestClient(t, h)
	session := newStreamTestSession(client, "")
	if err := session.Pause(context.Background()); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if session.State != SessionStatePaused {
		t.Fatalf("unexpected paused state: %q", session.State)
	}
	if err := session.Resume(context.Background()); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if session.State != SessionStateRunning {
		t.Fatalf("unexpected resumed state: %q", session.State)
	}
}
