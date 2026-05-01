package agent

import (
	"net/http"

	"github.com/marang/franz-agent/internal/agent/tools"
)

type sessionHeaderTransport struct {
	base http.RoundTripper
}

func wrapSessionHeaderTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &sessionHeaderTransport{base: base}
}

func (t *sessionHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	sessionID, _ := req.Context().Value(tools.SessionIDContextKey).(string)
	if sessionID == "" {
		return t.base.RoundTrip(req)
	}

	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	clone.Header.Set("session_id", sessionID)
	clone.Header.Set("conversation_id", sessionID)
	return t.base.RoundTrip(clone)
}
