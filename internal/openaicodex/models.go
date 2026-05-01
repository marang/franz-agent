package openaicodex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxErrorBodyExcerpt = 512

type ModelsRequest struct {
	BaseURL       string
	AccessToken   string
	AccountID     string
	ClientVersion string
	HTTPClient    *http.Client
}

type RemoteModel struct {
	ID                     string
	Name                   string
	ContextWindow          int64
	DefaultMaxTokens       int64
	CanReason              bool
	ReasoningLevels        []string
	DefaultReasoningEffort string
	SupportsImages         bool
}

type modelsPayload struct {
	Models []remoteModelPayload `json:"models"`
	Data   []remoteModelPayload `json:"data"`
}

type remoteModelPayload struct {
	ID                    string                  `json:"id"`
	Slug                  string                  `json:"slug"`
	DisplayName           string                  `json:"display_name"`
	Name                  string                  `json:"name"`
	ContextWindow         int64                   `json:"context_window"`
	MaxContextWindow      int64                   `json:"max_context_window"`
	DefaultMaxTokens      int64                   `json:"default_max_tokens"`
	DefaultReasoningLevel string                  `json:"default_reasoning_level"`
	SupportedReasoning    []string                `json:"reasoning_levels"`
	SupportedReasoningAlt []reasoningLevelPayload `json:"supported_reasoning_levels"`
	InputModalities       []string                `json:"input_modalities"`
	SupportedInAPI        *bool                   `json:"supported_in_api"`
	Visibility            string                  `json:"visibility"`
	ShowInPicker          *bool                   `json:"show_in_picker"`
}

type reasoningLevelPayload struct {
	Effort string `json:"effort"`
}

func FetchModels(ctx context.Context, req ModelsRequest) ([]RemoteModel, error) {
	if strings.TrimSpace(req.AccessToken) == "" {
		return nil, fmt.Errorf("missing access token")
	}
	if strings.TrimSpace(req.AccountID) == "" {
		return nil, fmt.Errorf("missing chatgpt account id")
	}

	endpoint, err := modelsEndpoint(req.BaseURL, req.ClientVersion)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create models request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+req.AccessToken)
	httpReq.Header.Set("OpenAI-Beta", "responses=experimental")
	httpReq.Header.Set("chatgpt-account-id", req.AccountID)
	httpReq.Header.Set("originator", "franz-agent")

	client := req.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request models endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read models response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("models endpoint returned %d: %s", resp.StatusCode, responseBodyExcerpt(body))
	}

	var payload modelsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse models payload: %w", err)
	}
	items := payload.Models
	if len(items) == 0 {
		items = payload.Data
	}

	models := make([]RemoteModel, 0, len(items))
	for _, item := range items {
		model, ok := normalizeRemoteModel(item)
		if ok {
			models = append(models, model)
		}
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("models payload contained no usable models")
	}
	return models, nil
}

func responseBodyExcerpt(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) <= maxErrorBodyExcerpt {
		return text
	}
	return text[:maxErrorBodyExcerpt] + "...[truncated]"
}

func modelsEndpoint(baseURL, clientVersion string) (string, error) {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = defaultBackendBaseURL + "/codex"
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse models base URL: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/models"
	q := u.Query()
	if strings.TrimSpace(clientVersion) != "" {
		q.Set("client_version", clientVersion)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func normalizeRemoteModel(item remoteModelPayload) (RemoteModel, bool) {
	id := strings.TrimSpace(firstNonEmpty(item.Slug, item.ID))
	if id == "" || !isVisibleModel(item) {
		return RemoteModel{}, false
	}
	name := strings.TrimSpace(firstNonEmpty(item.DisplayName, item.Name, id))
	contextWindow := firstPositive(item.ContextWindow, item.MaxContextWindow, 200000)
	defaultMaxTokens := firstPositive(item.DefaultMaxTokens, 32768)
	reasoningLevels := normalizeReasoningLevels(item)
	defaultReasoning := strings.TrimSpace(item.DefaultReasoningLevel)
	if defaultReasoning == "" && len(reasoningLevels) > 0 {
		defaultReasoning = reasoningLevels[min(1, len(reasoningLevels)-1)]
	}

	return RemoteModel{
		ID:                     id,
		Name:                   name,
		ContextWindow:          contextWindow,
		DefaultMaxTokens:       defaultMaxTokens,
		CanReason:              len(reasoningLevels) > 0 || strings.HasPrefix(id, "gpt-5"),
		ReasoningLevels:        reasoningLevels,
		DefaultReasoningEffort: defaultReasoning,
		SupportsImages:         hasInputModality(item.InputModalities, "image"),
	}, true
}

func normalizeReasoningLevels(item remoteModelPayload) []string {
	seen := make(map[string]bool)
	var levels []string
	for _, level := range item.SupportedReasoning {
		level = strings.TrimSpace(level)
		if level != "" && !seen[level] {
			levels = append(levels, level)
			seen[level] = true
		}
	}
	for _, preset := range item.SupportedReasoningAlt {
		level := strings.TrimSpace(preset.Effort)
		if level != "" && !seen[level] {
			levels = append(levels, level)
			seen[level] = true
		}
	}
	return levels
}

func isVisibleModel(item remoteModelPayload) bool {
	if item.SupportedInAPI != nil && !*item.SupportedInAPI {
		return false
	}
	if item.ShowInPicker != nil {
		return *item.ShowInPicker
	}
	switch strings.ToLower(strings.TrimSpace(item.Visibility)) {
	case "", "list", "visible", "picker":
		return true
	case "hidden":
		return false
	default:
		return true
	}
}

func hasInputModality(items []string, want string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), want) {
			return true
		}
	}
	return false
}

func firstPositive(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
