package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/marang/franz-agent/internal/oauth/openai_codex"
	"github.com/marang/franz-agent/internal/openaicodex"
	"github.com/stretchr/testify/require"
)

func TestEnsureOpenAICodexProvider(t *testing.T) {
	t.Parallel()

	providers := []catwalk.Provider{
		{
			ID: catwalk.InferenceProviderOpenAI,
			Models: []catwalk.Model{
				{ID: "gpt-5.4", Name: "gpt-5.4", ContextWindow: 272000, DefaultMaxTokens: 32768, CanReason: true},
				{ID: "gpt-5.4-mini", Name: "gpt-5.4-mini", ContextWindow: 272000, DefaultMaxTokens: 32768, CanReason: true},
			},
		},
	}

	got := ensureOpenAICodexProvider(providers)

	idx := -1
	for i, provider := range got {
		if provider.ID == catwalk.InferenceProvider(openai_codex.ProviderID) {
			idx = i
			break
		}
	}
	require.NotEqual(t, -1, idx)

	provider := got[idx]
	require.Equal(t, openAICodexBaseURL, provider.APIEndpoint)
	require.Equal(t, catwalk.TypeOpenAI, provider.Type)
	require.Equal(t, "gpt-5.4", provider.DefaultLargeModelID)
	require.Equal(t, "gpt-5.4-mini", provider.DefaultSmallModelID)
	require.GreaterOrEqual(t, len(provider.Models), 2)
}

func TestMergeOpenAICodexModelsKeepsFallbacks(t *testing.T) {
	t.Parallel()

	got := mergeOpenAICodexModels(
		[]catwalk.Model{{ID: "dynamic", Name: "Dynamic"}, {ID: "pinned", Name: "Remote Pinned"}},
		[]catwalk.Model{{ID: "pinned", Name: "Pinned"}, {ID: "fallback", Name: "Fallback"}},
	)

	require.Equal(t, []string{"dynamic", "pinned", "fallback"}, modelIDs(got))
	require.Equal(t, "Remote Pinned", got[1].Name)
	require.Equal(t, int64(200000), got[0].ContextWindow)
	require.Equal(t, int64(32768), got[0].DefaultMaxTokens)
}

func TestOpenAICodexRemoteModels(t *testing.T) {
	t.Parallel()

	got := openAICodexRemoteModels([]openaicodex.RemoteModel{
		{
			ID:                     "gpt-dynamic",
			Name:                   "GPT Dynamic",
			ContextWindow:          272000,
			DefaultMaxTokens:       64000,
			CanReason:              true,
			ReasoningLevels:        []string{"low", "medium"},
			DefaultReasoningEffort: "medium",
			SupportsImages:         true,
		},
	})

	require.Len(t, got, 1)
	require.Equal(t, "gpt-dynamic", got[0].ID)
	require.Equal(t, "GPT Dynamic", got[0].Name)
	require.Equal(t, []string{"low", "medium"}, got[0].ReasoningLevels)
	require.True(t, got[0].SupportsImages)
}

func TestRefreshOpenAICodexModelsCachesRemoteModels(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{
				{
					"slug":           "gpt-live",
					"display_name":   "GPT Live",
					"context_window": 300000,
					"supported_reasoning_levels": []map[string]any{
						{"effort": "medium"},
					},
				},
			},
		}))
	}))
	defer server.Close()

	fallback := []catwalk.Model{{ID: "gpt-fallback", Name: "GPT Fallback"}}
	provider := ProviderConfig{
		BaseURL: server.URL + "/backend-api/codex",
		APIKey:  "token",
		ExtraHeaders: map[string]string{
			"chatgpt-account-id": "acc",
		},
	}

	provider.Models = fallback
	got, err := RefreshOpenAICodexModels(context.Background(), provider)
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-live", "gpt-fallback"}, modelIDs(got))

	provider.BaseURL = "http://127.0.0.1:1/backend-api/codex"
	got = openAICodexModelsForProvider(provider, fallback)
	require.Equal(t, []string{"gpt-live", "gpt-fallback"}, modelIDs(got))
}

func modelIDs(models []catwalk.Model) []string {
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	return ids
}
