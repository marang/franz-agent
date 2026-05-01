package config

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/marang/franz-agent/internal/oauth/openai_codex"
	"github.com/marang/franz-agent/internal/openaicodex"
	"github.com/marang/franz-agent/internal/version"
)

const (
	openAICodexBaseURL       = "https://chatgpt.com/backend-api/codex"
	openAICodexModelCacheTTL = 5 * time.Minute
)

var openAICodexPinnedModels = []string{
	"gpt-5.4",
	"gpt-5.4-mini",
	"gpt-5.3-codex",
	"gpt-5.2-codex",
}

func ensureOpenAICodexProvider(providers []catwalk.Provider) []catwalk.Provider {
	id := catwalk.InferenceProvider(openai_codex.ProviderID)
	idx := slices.IndexFunc(providers, func(p catwalk.Provider) bool { return p.ID == id })
	if idx >= 0 {
		existing := providers[idx]
		if existing.DefaultLargeModelID == "" {
			existing.DefaultLargeModelID = openAICodexPinnedModels[0]
		}
		if existing.DefaultSmallModelID == "" {
			existing.DefaultSmallModelID = openAICodexPinnedModels[1]
		}
		if len(existing.Models) == 0 {
			existing.Models = buildPinnedOpenAICodexModels(providers)
		}
		providers[idx] = existing
		return providers
	}

	return append(providers, catwalk.Provider{
		Name:                openai_codex.ProviderName,
		ID:                  id,
		APIEndpoint:         openAICodexBaseURL,
		Type:                catwalk.TypeOpenAI,
		DefaultLargeModelID: openAICodexPinnedModels[0],
		DefaultSmallModelID: openAICodexPinnedModels[1],
		DefaultHeaders: map[string]string{
			"originator":  "franz",
			"OpenAI-Beta": "responses=experimental",
		},
		Models: buildPinnedOpenAICodexModels(providers),
	})
}

func buildPinnedOpenAICodexModels(providers []catwalk.Provider) []catwalk.Model {
	byID := make(map[string]catwalk.Model)
	for _, provider := range providers {
		for _, model := range provider.Models {
			if model.ID != "" {
				if _, exists := byID[model.ID]; !exists {
					byID[model.ID] = model
				}
			}
		}
	}

	models := make([]catwalk.Model, 0, len(openAICodexPinnedModels))
	for _, id := range openAICodexPinnedModels {
		if model, ok := byID[id]; ok {
			models = append(models, model)
			continue
		}
		models = append(models, catwalk.Model{
			ID:                     id,
			Name:                   id,
			ContextWindow:          200000,
			DefaultMaxTokens:       32768,
			CanReason:              strings.HasPrefix(id, "gpt-5"),
			DefaultReasoningEffort: "medium",
		})
	}
	return models
}

func openAICodexModelsForProvider(providerCfg ProviderConfig, fallback []catwalk.Model) []catwalk.Model {
	return mergeOpenAICodexModels(readCachedOpenAICodexModels(), fallback)
}

func RefreshOpenAICodexModels(ctx context.Context, providerCfg ProviderConfig) ([]catwalk.Model, error) {
	accountID := strings.TrimSpace(providerCfg.ExtraHeaders["chatgpt-account-id"])
	if accountID == "" && providerCfg.OAuthToken != nil {
		if parsed, err := openai_codex.ExtractAccountID(providerCfg.OAuthToken.AccessToken); err == nil {
			accountID = parsed
		}
	}
	accessToken := strings.TrimSpace(providerCfg.APIKey)
	if providerCfg.OAuthToken != nil && strings.TrimSpace(providerCfg.OAuthToken.AccessToken) != "" {
		accessToken = providerCfg.OAuthToken.AccessToken
	}
	if accessToken == "" || accountID == "" {
		return nil, fmt.Errorf("missing OpenAI Codex credentials")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	remote, err := openaicodex.FetchModels(ctx, openaicodex.ModelsRequest{
		BaseURL:       providerCfg.BaseURL,
		AccessToken:   accessToken,
		AccountID:     accountID,
		ClientVersion: version.Version,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch OpenAI Codex models: %w", err)
	}

	models := openAICodexRemoteModels(remote)
	if len(models) == 0 {
		return nil, fmt.Errorf("OpenAI Codex models payload contained no usable models")
	}
	if err := newCache[[]catwalk.Model](openAICodexModelCachePath()).Store(models); err != nil {
		slog.Debug("Failed to cache OpenAI Codex models", "error", err)
	}
	return mergeOpenAICodexModels(models, providerCfg.Models), nil
}

func readCachedOpenAICodexModels() []catwalk.Model {
	models, _, err := newCache[[]catwalk.Model](openAICodexModelCachePath()).Get()
	if err != nil {
		return nil
	}
	return models
}

func openAICodexModelCachePath() string {
	return cachePathFor("openai-codex-models")
}

func openAICodexRemoteModels(remote []openaicodex.RemoteModel) []catwalk.Model {
	models := make([]catwalk.Model, 0, len(remote))
	for _, item := range remote {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		models = append(models, catwalk.Model{
			ID:                     item.ID,
			Name:                   item.Name,
			ContextWindow:          item.ContextWindow,
			DefaultMaxTokens:       item.DefaultMaxTokens,
			CanReason:              item.CanReason,
			ReasoningLevels:        item.ReasoningLevels,
			DefaultReasoningEffort: item.DefaultReasoningEffort,
			SupportsImages:         item.SupportsImages,
		})
	}
	return models
}

func mergeOpenAICodexModels(primary, fallback []catwalk.Model) []catwalk.Model {
	merged := make([]catwalk.Model, 0, len(primary)+len(fallback))
	seen := make(map[string]bool, len(primary)+len(fallback))
	for _, list := range [][]catwalk.Model{primary, fallback} {
		for _, model := range list {
			if strings.TrimSpace(model.ID) == "" || seen[model.ID] {
				continue
			}
			if model.Name == "" {
				model.Name = model.ID
			}
			if model.ContextWindow == 0 {
				model.ContextWindow = 200000
			}
			if model.DefaultMaxTokens == 0 {
				model.DefaultMaxTokens = 32768
			}
			if model.DefaultReasoningEffort == "" && model.CanReason {
				model.DefaultReasoningEffort = "medium"
			}
			merged = append(merged, model)
			seen[model.ID] = true
		}
	}
	return merged
}
