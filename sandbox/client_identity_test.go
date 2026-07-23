package sandbox

import (
	"net/http"
	"testing"
)

func TestSetClientAuthHeadersAddsMigrationIdentity(t *testing.T) {
	t.Parallel()

	header := http.Header{}
	setClientAuthHeaders(header, "tk_test", defaultCookieName)

	if got := header.Get(headerClientFamily); got != "go_sdk" {
		t.Fatalf("client family = %q, want go_sdk", got)
	}
	if got := header.Get(headerClientGeneration); got != "workspace_v1" {
		t.Fatalf("client generation = %q, want workspace_v1", got)
	}
}
