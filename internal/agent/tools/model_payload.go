package tools

import (
	"context"
	"regexp"
	"strings"

	"charm.land/fantasy"
)

var (
	markupTagRegex  = regexp.MustCompile(`<[^>]+>`)
	whitespaceRegex = regexp.MustCompile(`\s+`)
)

var essentialToolSet = map[string]struct{}{
	"agent":      {},
	"bash":       {},
	"job_output": {},
	"job_kill":   {},
	"edit":       {},
	"multiedit":  {},
	"write":      {},
	"view":       {},
	"glob":       {},
	"grep":       {},
	"ls":         {},
	"todos":      {},
	"fetch":      {},
}

// KeepEssentialTools returns only tools from the essential default set while
// preserving input order.
func KeepEssentialTools(input []fantasy.AgentTool) []fantasy.AgentTool {
	filtered := make([]fantasy.AgentTool, 0, len(input))
	for _, tool := range input {
		if _, ok := essentialToolSet[tool.Info().Name]; ok {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

// CompactToolsForModelPayload wraps tools with compact descriptions to reduce
// request payload size without changing runtime behavior.
func CompactToolsForModelPayload(input []fantasy.AgentTool) []fantasy.AgentTool {
	compact := make([]fantasy.AgentTool, 0, len(input))
	for _, tool := range input {
		compact = append(compact, compactDescriptionTool{inner: tool})
	}
	return compact
}

type compactDescriptionTool struct {
	inner fantasy.AgentTool
}

func (t compactDescriptionTool) Info() fantasy.ToolInfo {
	info := t.inner.Info()
	info.Description = compactDescription(info.Description)
	return info
}

func (t compactDescriptionTool) Run(ctx context.Context, params fantasy.ToolCall) (fantasy.ToolResponse, error) {
	return t.inner.Run(ctx, params)
}

func (t compactDescriptionTool) ProviderOptions() fantasy.ProviderOptions {
	return t.inner.ProviderOptions()
}

func (t compactDescriptionTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	t.inner.SetProviderOptions(opts)
}

func compactDescription(text string) string {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return clean
	}
	clean = markupTagRegex.ReplaceAllString(clean, " ")
	clean = whitespaceRegex.ReplaceAllString(clean, " ")
	clean = strings.TrimSpace(clean)

	// Try to keep first sentence if it is reasonably short.
	if idx := strings.Index(clean, ". "); idx > 0 && idx <= 220 {
		return strings.TrimSpace(clean[:idx+1])
	}
	runes := []rune(clean)
	if len(runes) <= 260 {
		return clean
	}
	return string(runes[:257]) + "..."
}
