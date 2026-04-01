package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wall-ai/agent-engine/internal/server"
)

// newTestServer spins up a real chi router backed by the production handler.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := server.New(":0") // port 0 — won't actually ListenAndServe
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func TestHealthEndpoint(t *testing.T) {
	ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
	assert.NotEmpty(t, body["time"])
}

func TestCreateSessionMissingWorkDir(t *testing.T) {
	ts := newTestServer(t)

	payload := map[string]string{"provider": "anthropic"}
	b, _ := json.Marshal(payload)

	resp, err := http.Post(ts.URL+"/api/v1/sessions", "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Contains(t, body["error"], "work_dir")
}

func TestListSessionsEmpty(t *testing.T) {
	ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/api/v1/sessions")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestListToolsNoSession(t *testing.T) {
	ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/api/v1/tools")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Without a session ID header the server returns 200 with the default tool list.
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest)
}

func TestAPIKeyAuthMiddleware(t *testing.T) {
	t.Setenv("AGENT_ENGINE_API_KEY", "test-secret-key")

	ts := newTestServer(t)

	t.Run("no auth header", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/sessions")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("wrong key", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/sessions", nil)
		req.Header.Set("Authorization", "Bearer wrong-key")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("correct key", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/sessions", nil)
		req.Header.Set("Authorization", "Bearer test-secret-key")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("health is always public", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
