package sandbox

import "testing"

func TestNew_UsesCanonicalEnvFallbacks(t *testing.T) {
	t.Setenv(EnvAuthToken, "tk_env_auth")
	t.Setenv(EnvAPIEndpoint, "https://api.example.com")

	client, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = client.Close() }()

	if client.authToken != "tk_env_auth" {
		t.Fatalf("unexpected auth token: %q", client.authToken)
	}
	if client.baseURL != "https://api.example.com" {
		t.Fatalf("unexpected base url: %q", client.baseURL)
	}
}
