package agent

import (
	"context"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/marang/franz-agent/internal/message"
	"github.com/marang/franz-agent/internal/permission"
	"github.com/stretchr/testify/require"
)

type queuedDiscussModel struct {
	calls int
}

func (m *queuedDiscussModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, nil
}

func (m *queuedDiscussModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	m.calls++
	call := m.calls
	return func(yield func(fantasy.StreamPart) bool) {
		switch call {
		case 1:
			if !yield(fantasy.StreamPart{
				Type:          fantasy.StreamPartTypeToolCall,
				ID:            "call-discuss",
				ToolCallName:  "needs_permission",
				ToolCallInput: "{}",
			}) {
				return
			}
			yield(fantasy.StreamPart{
				Type:         fantasy.StreamPartTypeFinish,
				FinishReason: fantasy.FinishReasonToolCalls,
			})
		default:
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "text-1"}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "text-1", Delta: "discussion started"}) {
				return
			}
			if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "text-1"}) {
				return
			}
			yield(fantasy.StreamPart{
				Type:         fantasy.StreamPartTypeFinish,
				FinishReason: fantasy.FinishReasonStop,
			})
		}
	}, nil
}

func (m *queuedDiscussModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, nil
}

func (m *queuedDiscussModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return func(func(fantasy.ObjectStreamPart) bool) {}, nil
}

func (m *queuedDiscussModel) Provider() string { return "test" }
func (m *queuedDiscussModel) Model() string    { return "queued-discuss" }

func TestPermissionDiscussRunsQueuedPrompt(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	model := &queuedDiscussModel{}
	session, err := env.sessions.Create(t.Context(), "test")
	require.NoError(t, err)

	var agent SessionAgent
	permissionTool := fantasy.NewAgentTool(
		"needs_permission",
		"requires permission",
		func(ctx context.Context, _ map[string]any, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			_, err := agent.Run(ctx, SessionAgentCall{
				Prompt:          "discuss queued",
				SessionID:       session.ID,
				MaxOutputTokens: 100,
			})
			require.NoError(t, err)
			return fantasy.ToolResponse{}, permission.ErrorPermissionDiscuss
		},
	)
	agent = testSessionAgent(env, model, model, "test prompt", permissionTool)

	_, err = agent.Run(t.Context(), SessionAgentCall{
		Prompt:          "first",
		SessionID:       session.ID,
		MaxOutputTokens: 100,
	})
	require.NoError(t, err)

	msgs, err := env.messages.List(t.Context(), session.ID)
	require.NoError(t, err)

	var sawDiscussionAssistant bool
	for _, msg := range msgs {
		if msg.Role == message.Assistant {
			if strings.Contains(msg.Content().Text, "discussion started") {
				sawDiscussionAssistant = true
			}
		}
	}
	require.GreaterOrEqual(t, model.calls, 2)
	require.True(t, sawDiscussionAssistant)
}

func TestPermissionDiscussToolResultIsNotError(t *testing.T) {
	t.Parallel()

	toolResult := (&sessionAgent{}).convertToToolResult(fantasy.ToolResultContent{
		ToolCallID: "call-discuss",
		ToolName:   "needs_permission",
		Result: fantasy.ToolResultOutputContentError{
			Error: permission.ErrorPermissionDiscuss,
		},
	})

	require.Equal(t, "call-discuss", toolResult.ToolCallID)
	require.Equal(t, "needs_permission", toolResult.Name)
	require.Equal(t, "Permission moved to discussion with the user", toolResult.Content)
	require.False(t, toolResult.IsError)
}
