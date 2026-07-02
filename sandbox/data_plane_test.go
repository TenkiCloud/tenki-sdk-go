package sandbox

import (
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
)

func TestIsEdgeNotReady(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		// Caddy returns a plain HTTP 404 before the per-session route applies;
		// connect maps it to Unimplemented and carries the HTTP status text.
		{"edge 404", connect.NewError(connect.CodeUnimplemented, errors.New("HTTP status 404 Not Found")), true},
		// A genuine node-agent capability-unavailable is Unimplemented without a 404.
		{"real unimplemented", connect.NewError(connect.CodeUnimplemented, errors.New("run is not implemented")), false},
		// Other transport errors must not be treated as edge-not-ready.
		{"unavailable", connect.NewError(connect.CodeUnavailable, errors.New("connection refused")), false},
		{"not found 404 wrong code", connect.NewError(connect.CodeNotFound, errors.New("HTTP status 404")), false},
		{"unimplemented unrelated digits", connect.NewError(connect.CodeUnimplemented, errors.New("capability version 4040 missing")), false},
		{"plain error", errors.New("HTTP status 404 Not Found"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isEdgeNotReady(tc.err); got != tc.want {
				t.Fatalf("isEdgeNotReady(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestDataPlaneReadyBackoff(t *testing.T) {
	if got := dataPlaneReadyBackoff(0); got != 50*time.Millisecond {
		t.Fatalf("attempt 0 = %v, want 50ms", got)
	}
	// Capped at 750ms regardless of attempt.
	if got := dataPlaneReadyBackoff(100); got != 750*time.Millisecond {
		t.Fatalf("attempt 100 = %v, want 750ms (cap)", got)
	}
	// Monotonic non-decreasing up to the cap.
	prev := time.Duration(0)
	for a := range 32 {
		d := dataPlaneReadyBackoff(a)
		if d < prev {
			t.Fatalf("backoff decreased at attempt %d: %v < %v", a, d, prev)
		}
		prev = d
	}
}

func TestDataPlaneNotReadyErrorContract(t *testing.T) {
	err := dataPlaneNotReadyError(connect.NewError(connect.CodeUnimplemented, errors.New("HTTP status 404 Not Found")))
	if !IsDataPlaneNotReady(err) {
		t.Fatal("IsDataPlaneNotReady should match DataPlaneNotReadyError")
	}
	if IsCapabilityUnavailable(err) {
		t.Fatal("edge 404 readiness errors must not look like capability unavailable")
	}
	var retryable interface{ IsRetryable() bool }
	if !errors.As(err, &retryable) || !retryable.IsRetryable() {
		t.Fatal("DataPlaneNotReadyError should be marked retryable")
	}
}

func TestMapErrorMapsEdge404ToDataPlaneNotReady(t *testing.T) {
	err := mapError(connect.NewError(connect.CodeUnimplemented, errors.New("HTTP status 404 Not Found")))
	if !IsDataPlaneNotReady(err) {
		t.Fatalf("mapError should return DataPlaneNotReadyError, got %T: %v", err, err)
	}
}
