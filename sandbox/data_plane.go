package sandbox

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
)

const dataPlaneCredentialHeader = "x-tenki-session-cert"

var errDataPlaneEndpointUnavailable = errors.New("sandbox: data-plane endpoint unavailable")

// dataPlaneReadyBackoff returns the wait before the next readiness probe.
func dataPlaneReadyBackoff(attempt int) time.Duration {
	return min(time.Duration(50*(attempt+1))*time.Millisecond, 750*time.Millisecond)
}

// isEdgeNotReady reports whether err is the transient "edge route published but
// not yet serving" condition. The per-session data-plane route lives on the
// edge (Caddy); the engine publishes it synchronously but Caddy applies the
// config asynchronously after the admin API returns 200, so a request can
// arrive before the route serves and gets the edge's plain HTTP 404, which
// connect maps to CodeUnimplemented. A genuine node-agent Unimplemented
// (capability unavailable) is delivered with gRPC framing and lacks the
// HTTP-404 signature, so it is not retried.
func isEdgeNotReady(err error) bool {
	if err == nil {
		return false
	}
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		return false
	}
	return strings.Contains(err.Error(), "HTTP status 404")
}

func dataPlaneNotReadyError(err error) error {
	if err == nil {
		return &DataPlaneNotReadyError{}
	}
	return &DataPlaneNotReadyError{Message: "sandbox: data-plane edge route not ready", Err: err}
}

func terminalDataPlaneNotReadyError(err error) error {
	return &DataPlaneNotReadyError{Message: "sandbox: data-plane route verification failed", Err: err, Terminal: true}
}

func dataPlaneReadyContext(parent context.Context, budget time.Duration) (context.Context, context.CancelFunc) {
	if budget <= 0 {
		budget = defaultDataPlaneReadyTimeout
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), budget)
	done := make(chan struct{})
	var once sync.Once
	go func() {
		select {
		case <-parent.Done():
			if errors.Is(parent.Err(), context.Canceled) {
				cancel()
			}
		case <-done:
		}
	}()
	return ctx, func() {
		once.Do(func() {
			close(done)
			cancel()
		})
	}
}

func newDataPlaneHTTPClient(endpoint string) *http.Client {
	transport := &http2.Transport{
		ReadIdleTimeout: 30 * time.Second,
		PingTimeout:     5 * time.Second,
	}
	if strings.HasPrefix(endpoint, "http://") {
		transport.AllowHTTP = true
		transport.DialTLS = func(network, addr string, _ *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		}
	}
	return &http.Client{Transport: transport}
}

func (c *Client) dataPlaneClientOptions(session *Session) []connect.ClientOption {
	opts := []connect.ClientOption{
		connect.WithInterceptors(&dataPlaneCredentialInterceptor{session: session}),
		connect.WithGRPC(),
	}
	opts = append(opts, c.connectOpts...)
	return opts
}

type dataPlaneCredentialInterceptor struct {
	session *Session
}

func (i *dataPlaneCredentialInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if !req.Spec().IsClient {
			return next(ctx, req)
		}
		i.setHeaders(req.Header())
		resp, err := next(ctx, req)
		if err != nil && i.session.reauthOnUnauthenticated(ctx, err) {
			i.setHeaders(req.Header())
			return next(ctx, req)
		}
		return resp, err
	}
}

func (i *dataPlaneCredentialInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

func (i *dataPlaneCredentialInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		i.setHeaders(conn.RequestHeader())
		return conn
	}
}

func (i *dataPlaneCredentialInterceptor) setHeaders(header http.Header) {
	credential := strings.TrimSpace(i.session.currentDataPlaneCredential())
	if credential != "" {
		header.Set(dataPlaneCredentialHeader, credential)
	}
}
