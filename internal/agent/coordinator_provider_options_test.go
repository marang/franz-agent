package agent

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy/providers/openai"
	"github.com/marang/franz-agent/internal/config"
	"github.com/marang/franz-agent/internal/openaicodex"
	"github.com/stretchr/testify/require"
)

func TestGetProviderOptionsPromptCacheKeyForSubscription(t *testing.T) {
	t.Setenv("FRANZ_FEATURE_PROMPT_CACHE_KEY", "true")

	model := Model{
		CatwalkCfg: catwalk.Model{
			ID: "gpt-5.3-codex",
		},
		ModelCfg: config.SelectedModel{
			Provider: openaicodex.ProviderID,
		},
	}
	providerCfg := config.ProviderConfig{
		ID:   openaicodex.ProviderID,
		Type: openai.Name,
	}

	opts := getProviderOptions(model, providerCfg, "session-123")
	raw, ok := opts[openai.Name]
	require.True(t, ok)

	parsed, ok := raw.(*openai.ResponsesProviderOptions)
	require.True(t, ok)
	require.NotNil(t, parsed.PromptCacheKey)
	require.Equal(t, "session-123", *parsed.PromptCacheKey)
}

func TestGetProviderOptionsPromptCacheKeyDisabled(t *testing.T) {
	t.Setenv("FRANZ_FEATURE_PROMPT_CACHE_KEY", "false")

	model := Model{
		CatwalkCfg: catwalk.Model{
			ID: "gpt-5.3-codex",
		},
		ModelCfg: config.SelectedModel{
			Provider: openaicodex.ProviderID,
		},
	}
	providerCfg := config.ProviderConfig{
		ID:   openaicodex.ProviderID,
		Type: openai.Name,
	}

	opts := getProviderOptions(model, providerCfg, "session-123")
	raw, ok := opts[openai.Name]
	require.True(t, ok)

	parsed, ok := raw.(*openai.ResponsesProviderOptions)
	require.True(t, ok)
	require.Nil(t, parsed.PromptCacheKey)
}

func TestGetProviderOptionsDeepMerge(t *testing.T) {
	model := Model{
		CatwalkCfg: catwalk.Model{
			ID: "gpt-5.3-codex",
			Options: catwalk.ModelOptions{
				ProviderOptions: map[string]any{
					"metadata": map[string]any{
						"source": "catwalk",
						"keep":   true,
					},
				},
			},
		},
		ModelCfg: config.SelectedModel{
			Provider: openaicodex.ProviderID,
			ProviderOptions: map[string]any{
				"metadata": map[string]any{
					"model": "selected",
				},
			},
		},
	}
	providerCfg := config.ProviderConfig{
		ID:   openaicodex.ProviderID,
		Type: openai.Name,
		ProviderOptions: map[string]any{
			"metadata": map[string]any{
				"provider": "cfg",
			},
		},
	}

	opts := getProviderOptions(model, providerCfg, "session-123")
	raw, ok := opts[openai.Name]
	require.True(t, ok)
	parsed, ok := raw.(*openai.ResponsesProviderOptions)
	require.True(t, ok)
	require.NotNil(t, parsed.Metadata)
	require.Equal(t, "catwalk", parsed.Metadata["source"])
	require.Equal(t, true, parsed.Metadata["keep"])
	require.Equal(t, "cfg", parsed.Metadata["provider"])
	require.Equal(t, "selected", parsed.Metadata["model"])
}
