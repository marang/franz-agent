package tools

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

type testInput struct {
	Name string `json:"name"`
}

func TestCompactToolsForModelPayload(t *testing.T) {
	tool := fantasy.NewAgentTool(
		"test_tool",
		"<usage>Very long description with   extra spaces.</usage> Additional details that should be trimmed.",
		func(_ context.Context, _ testInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		},
	)

	wrapped := CompactToolsForModelPayload([]fantasy.AgentTool{tool})
	require.Len(t, wrapped, 1)

	info := wrapped[0].Info()
	require.Equal(t, "test_tool", info.Name)
	require.NotContains(t, info.Description, "<usage>")
	require.NotContains(t, info.Description, "  ")
}

func TestKeepEssentialTools(t *testing.T) {
	essential := fantasy.NewAgentTool(
		"bash",
		"run commands",
		func(_ context.Context, _ testInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		},
	)
	nonEssential := fantasy.NewAgentTool(
		"sourcegraph",
		"search",
		func(_ context.Context, _ testInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		},
	)

	filtered := KeepEssentialTools([]fantasy.AgentTool{essential, nonEssential})
	require.Len(t, filtered, 1)
	require.Equal(t, "bash", filtered[0].Info().Name)
}
