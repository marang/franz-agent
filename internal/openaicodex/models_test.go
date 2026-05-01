package openaicodex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchModels(t *testing.T) {
	t.Parallel()

	var gotPath, gotClientVersion, gotAuth, gotAccount string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotClientVersion = r.URL.Query().Get("client_version")
		gotAuth = r.Header.Get("Authorization")
		gotAccount = r.Header.Get("chatgpt-account-id")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{
				{
					"slug":                    "gpt-test",
					"display_name":            "GPT Test",
					"context_window":          123000,
					"default_reasoning_level": "medium",
					"supported_reasoning_levels": []map[string]any{
						{"effort": "low"},
						{"effort": "medium"},
					},
					"input_modalities": []string{"text", "image"},
					"visibility":       "list",
					"supported_in_api": true,
				},
				{
					"slug":       "hidden",
					"visibility": "hidden",
				},
			},
		}))
	}))
	defer server.Close()

	models, err := FetchModels(context.Background(), ModelsRequest{
		BaseURL:       server.URL + "/backend-api/codex",
		AccessToken:   "token",
		AccountID:     "acc",
		ClientVersion: "1.2.3",
	})
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "gpt-test", models[0].ID)
	require.Equal(t, "GPT Test", models[0].Name)
	require.Equal(t, int64(123000), models[0].ContextWindow)
	require.Equal(t, []string{"low", "medium"}, models[0].ReasoningLevels)
	require.True(t, models[0].SupportsImages)
	require.Equal(t, "/backend-api/codex/models", gotPath)
	require.Equal(t, "1.2.3", gotClientVersion)
	require.Equal(t, "Bearer token", gotAuth)
	require.Equal(t, "acc", gotAccount)
}

func TestFetchModelsRejectsEmptyPayload(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer server.Close()

	_, err := FetchModels(context.Background(), ModelsRequest{
		BaseURL:     server.URL,
		AccessToken: "token",
		AccountID:   "acc",
	})
	require.ErrorContains(t, err, "no usable models")
}
