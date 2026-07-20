package sandbox

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	sandboxv1 "github.com/TenkiCloud/tenki-sdk-go/sandbox/internal/proto/tenki/sandbox/v1"
	"github.com/TenkiCloud/tenki-sdk-go/sandbox/internal/proto/tenki/sandbox/v1/sandboxv1connect"
)

type sandboxListHandler struct {
	sandboxv1connect.UnimplementedSandboxServiceHandler

	listWorkspaceSandboxesFn func(*connect.Request[sandboxv1.ListWorkspaceSandboxesRequest]) (*connect.Response[sandboxv1.ListWorkspaceSandboxesResponse], error)
	listProjectSandboxesFn   func(*connect.Request[sandboxv1.ListProjectSandboxesRequest]) (*connect.Response[sandboxv1.ListProjectSandboxesResponse], error)
}

func (h *sandboxListHandler) ListWorkspaceSandboxes(_ context.Context, req *connect.Request[sandboxv1.ListWorkspaceSandboxesRequest]) (*connect.Response[sandboxv1.ListWorkspaceSandboxesResponse], error) {
	if h.listWorkspaceSandboxesFn != nil {
		return h.listWorkspaceSandboxesFn(req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (h *sandboxListHandler) ListProjectSandboxes(_ context.Context, req *connect.Request[sandboxv1.ListProjectSandboxesRequest]) (*connect.Response[sandboxv1.ListProjectSandboxesResponse], error) {
	if h.listProjectSandboxesFn != nil {
		return h.listProjectSandboxesFn(req)
	}
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func newSandboxListTestClient(t *testing.T, h *sandboxListHandler) *Client {
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

func TestListWorkspaceSandboxes(t *testing.T) {
	t.Parallel()

	h := &sandboxListHandler{}
	h.listWorkspaceSandboxesFn = func(req *connect.Request[sandboxv1.ListWorkspaceSandboxesRequest]) (*connect.Response[sandboxv1.ListWorkspaceSandboxesResponse], error) {
		if req.Msg.GetWorkspaceId() != "ws-001" {
			t.Fatalf("unexpected workspace_id: %q", req.Msg.GetWorkspaceId())
		}
		return connect.NewResponse(&sandboxv1.ListWorkspaceSandboxesResponse{
			Sessions: []*sandboxv1.SandboxSession{
				{Id: "sess-001", WorkspaceId: "ws-001", ProjectId: "proj-001"},
			},
		}), nil
	}

	client := newSandboxListTestClient(t, h)
	sessions, err := client.ListWorkspaceSandboxes(context.Background(), "ws-001")
	if err != nil {
		t.Fatalf("ListWorkspaceSandboxes: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "sess-001" || sessions[0].ProjectID != "proj-001" {
		t.Fatalf("unexpected sessions: %#v", sessions)
	}
}

func TestListProjectSandboxes(t *testing.T) {
	t.Parallel()

	h := &sandboxListHandler{}
	h.listProjectSandboxesFn = func(req *connect.Request[sandboxv1.ListProjectSandboxesRequest]) (*connect.Response[sandboxv1.ListProjectSandboxesResponse], error) {
		if req.Msg.GetProjectId() != "proj-001" {
			t.Fatalf("unexpected project_id: %q", req.Msg.GetProjectId())
		}
		return connect.NewResponse(&sandboxv1.ListProjectSandboxesResponse{
			Sessions: []*sandboxv1.SandboxSession{
				{Id: "sess-001", WorkspaceId: "ws-001", ProjectId: "proj-001"},
			},
		}), nil
	}

	client := newSandboxListTestClient(t, h)
	sessions, err := client.ListProjectSandboxes(context.Background(), "proj-001")
	if err != nil {
		t.Fatalf("ListProjectSandboxes: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "sess-001" || sessions[0].ProjectID != "proj-001" {
		t.Fatalf("unexpected sessions: %#v", sessions)
	}
}
