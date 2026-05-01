package agent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marang/franz-agent/internal/agent/tools"
	"github.com/stretchr/testify/require"
)

func TestSessionHeaderTransportAddsHeadersFromContext(t *testing.T) {
	t.Parallel()

	var gotSessionID string
	var gotConversationID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSessionID = r.Header.Get("session_id")
		gotConversationID = r.Header.Get("conversation_id")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client := &http.Client{Transport: wrapSessionHeaderTransport(nil)}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	req = req.WithContext(context.WithValue(req.Context(), tools.SessionIDContextKey, "sess-42"))

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	require.Equal(t, "sess-42", gotSessionID)
	require.Equal(t, "sess-42", gotConversationID)
}

func TestSessionHeaderTransportNoSessionInContext(t *testing.T) {
	t.Parallel()

	var gotSessionID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSessionID = r.Header.Get("session_id")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client := &http.Client{Transport: wrapSessionHeaderTransport(nil)}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	require.Equal(t, "", gotSessionID)
}
