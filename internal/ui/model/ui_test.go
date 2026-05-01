package model

import (
	"reflect"
	"testing"
	"unsafe"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/marang/franz-agent/internal/app"
	"github.com/marang/franz-agent/internal/config"
	"github.com/marang/franz-agent/internal/csync"
	"github.com/marang/franz-agent/internal/permission"
	"github.com/marang/franz-agent/internal/ui/attachments"
	"github.com/marang/franz-agent/internal/ui/common"
	"github.com/marang/franz-agent/internal/ui/dialog"
	"github.com/marang/franz-agent/internal/ui/util"
	"github.com/stretchr/testify/require"
)

func TestCurrentModelSupportsImages(t *testing.T) {
	t.Parallel()

	t.Run("returns false when config is nil", func(t *testing.T) {
		t.Parallel()

		ui := newTestUIWithConfig(t, nil)
		require.False(t, ui.currentModelSupportsImages())
	})

	t.Run("returns false when coder agent is missing", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Providers: csync.NewMap[string, config.ProviderConfig](),
			Agents:    map[string]config.Agent{},
		}
		ui := newTestUIWithConfig(t, cfg)
		require.False(t, ui.currentModelSupportsImages())
	})

	t.Run("returns false when model is not found", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Providers: csync.NewMap[string, config.ProviderConfig](),
			Agents: map[string]config.Agent{
				config.AgentCoder: {Model: config.SelectedModelTypeLarge},
			},
		}
		ui := newTestUIWithConfig(t, cfg)
		require.False(t, ui.currentModelSupportsImages())
	})

	t.Run("returns true when current model supports images", func(t *testing.T) {
		t.Parallel()

		providers := csync.NewMap[string, config.ProviderConfig]()
		providers.Set("test-provider", config.ProviderConfig{
			ID: "test-provider",
			Models: []catwalk.Model{
				{ID: "test-model", SupportsImages: true},
			},
		})

		cfg := &config.Config{
			Models: map[config.SelectedModelType]config.SelectedModel{
				config.SelectedModelTypeLarge: {
					Provider: "test-provider",
					Model:    "test-model",
				},
			},
			Providers: providers,
			Agents: map[string]config.Agent{
				config.AgentCoder: {Model: config.SelectedModelTypeLarge},
			},
		}

		ui := newTestUIWithConfig(t, cfg)
		require.True(t, ui.currentModelSupportsImages())
	})
}

func newTestUIWithConfig(t *testing.T, cfg *config.Config) *UI {
	t.Helper()

	store := &config.ConfigStore{}
	setUnexportedField(t, store, "config", cfg)

	appInstance := &app.App{}
	setUnexportedField(t, appInstance, "config", store)

	return &UI{
		com: &common.Common{
			App: appInstance,
		},
	}
}

func TestShouldCyclePermissionMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		ui     UI
		wants  bool
		dialog dialog.Dialog
	}{
		{
			name:  "chat editor without dialogs cycles",
			ui:    UI{state: uiChat, focus: uiFocusEditor, dialog: dialog.NewOverlay()},
			wants: true,
		},
		{
			name:  "landing editor without dialogs cycles",
			ui:    UI{state: uiLanding, focus: uiFocusEditor, dialog: dialog.NewOverlay()},
			wants: true,
		},
		{
			name:  "chat main focus does not cycle",
			ui:    UI{state: uiChat, focus: uiFocusMain, dialog: dialog.NewOverlay()},
			wants: false,
		},
		{
			name:  "initialize editor does not cycle",
			ui:    UI{state: uiInitialize, focus: uiFocusEditor, dialog: dialog.NewOverlay()},
			wants: false,
		},
		{
			name: "open dialog does not cycle",
			ui: UI{state: uiChat, focus: uiFocusEditor, dialog: dialog.NewOverlay(
				dialog.NewQuit(nil),
			)},
			wants: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.wants, tt.ui.shouldCyclePermissionMode())
		})
	}
}

func TestShouldAttemptClipboardImagePaste(t *testing.T) {
	t.Parallel()

	t.Run("returns true for empty paste payload", func(t *testing.T) {
		t.Parallel()
		require.True(t, shouldAttemptClipboardImagePaste(tea.PasteMsg{Content: ""}))
	})

	t.Run("returns false for non-empty paste payload", func(t *testing.T) {
		t.Parallel()
		require.False(t, shouldAttemptClipboardImagePaste(tea.PasteMsg{Content: "hello"}))
	})
}

func TestSidebarShortcutDoesNotReachTextareaOutsideChat(t *testing.T) {
	t.Parallel()

	keyMap := DefaultKeyMap()
	ta := textarea.New()
	ta.SetValue("abc")
	ta.SetCursorColumn(3)
	ui := &UI{
		state:    uiLanding,
		focus:    uiFocusEditor,
		dialog:   dialog.NewOverlay(),
		keyMap:   keyMap,
		textarea: ta,
		attachments: attachments.New(nil, attachments.Keymap{
			DeleteMode: keyMap.Editor.AttachmentDeleteMode,
			DeleteAll:  keyMap.Editor.DeleteAllAttachments,
			Escape:     keyMap.Editor.Escape,
		}),
	}

	_ = ui.handleKeyPressMsg(tea.KeyPressMsg(tea.Key{Code: 'b', Mod: tea.ModCtrl}))

	require.Equal(t, 3, ui.textarea.Column())
	require.Equal(t, "abc", ui.textarea.Value())
}

func TestShortHelpShowsAddFileWhenEditorFocused(t *testing.T) {
	t.Parallel()

	ui := newTestUI()
	ui.keyMap = DefaultKeyMap()

	var found bool
	for _, binding := range ui.ShortHelp() {
		help := binding.Help()
		if help.Key == "ctrl+f" && help.Desc == "add file" {
			found = true
			break
		}
	}

	require.True(t, found)
}

func TestPermissionDiscussionPromptUsesEnglishAndWaits(t *testing.T) {
	t.Parallel()

	prompt := permissionDiscussionPrompt(permission.PermissionRequest{
		ToolName:    "bash",
		Action:      "execute",
		Path:        "/tmp/project",
		Description: "Execute command: go test ./...",
	})

	require.Contains(t, prompt, "Let's discuss this proposed tool action before continuing.")
	require.Contains(t, prompt, "Do not run tools or continue the implementation until the user explicitly approves the next step.")
	require.NotContains(t, prompt, "Lass uns")
	require.NotContains(t, prompt, "setze danach")
}

func TestCyclePermissionModeCyclesModes(t *testing.T) {
	t.Parallel()

	perm := permission.NewPermissionService(t.TempDir(), false, nil)
	ui := &UI{
		state:           uiChat,
		focus:           uiFocusEditor,
		dialog:          dialog.NewOverlay(),
		planModeEnabled: false,
		textarea:        textarea.New(),
		com: &common.Common{
			App: &app.App{
				Permissions: perm,
			},
		},
	}

	require.False(t, ui.com.App.Permissions.SkipRequests())
	require.False(t, ui.planModeEnabled)

	msg := ui.cyclePermissionMode()
	require.Equal(t, "Yolo mode on", msg)
	require.True(t, ui.com.App.Permissions.SkipRequests())
	require.False(t, ui.planModeEnabled)

	msg = ui.cyclePermissionMode()
	require.Equal(t, "Planning mode on", msg)
	require.False(t, ui.com.App.Permissions.SkipRequests())
	require.True(t, ui.planModeEnabled)

	msg = ui.cyclePermissionMode()
	require.Equal(t, "Mode set to normal", msg)
	require.False(t, ui.com.App.Permissions.SkipRequests())
	require.False(t, ui.planModeEnabled)
}

func TestHandleLocalBTWCommand(t *testing.T) {
	t.Parallel()

	t.Run("returns not handled for other commands", func(t *testing.T) {
		t.Parallel()

		ui := &UI{}
		cmd, handled, inline := ui.handleLocalBTWCommand("/status")
		require.False(t, handled)
		require.Nil(t, cmd)
		require.Empty(t, inline)
		require.False(t, ui.btwModeEnabled)
	})

	t.Run("arms one-shot mode for bare command", func(t *testing.T) {
		t.Parallel()

		ui := &UI{}
		cmd, handled, inline := ui.handleLocalBTWCommand("/btw")
		require.True(t, handled)
		require.NotNil(t, cmd)
		require.Empty(t, inline)
		require.True(t, ui.btwModeEnabled)

		msg := cmd()
		info, ok := msg.(util.InfoMsg)
		require.True(t, ok)
		require.Equal(t, util.InfoTypeInfo, info.Type)
		require.Contains(t, info.Msg, "BTW mode armed")
	})

	t.Run("supports inline message", func(t *testing.T) {
		t.Parallel()

		ui := &UI{}
		cmd, handled, inline := ui.handleLocalBTWCommand("/btw this is a side note")
		require.True(t, handled)
		require.Nil(t, cmd)
		require.Equal(t, "BTW (side note): this is a side note", inline)
		require.False(t, ui.btwModeEnabled)
	})
}

func TestPrepareOutgoingMessageOneShotBTW(t *testing.T) {
	t.Parallel()

	ui := &UI{btwModeEnabled: true}
	got := ui.prepareOutgoingMessage("check this")
	require.Equal(t, "BTW (side note): check this", got)
	require.False(t, ui.btwModeEnabled)

	got = ui.prepareOutgoingMessage("normal")
	require.Equal(t, "normal", got)
}

func setUnexportedField(t *testing.T, target any, name string, value any) {
	t.Helper()

	v := reflect.ValueOf(target)
	require.Equal(t, reflect.Pointer, v.Kind())
	require.False(t, v.IsNil())

	field := v.Elem().FieldByName(name)
	require.Truef(t, field.IsValid(), "field %q not found", name)

	fieldValue := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	fieldValue.Set(reflect.ValueOf(value))
}
