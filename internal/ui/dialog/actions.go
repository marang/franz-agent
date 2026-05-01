package dialog

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/marang/franz-agent/internal/commands"
	"github.com/marang/franz-agent/internal/config"
	"github.com/marang/franz-agent/internal/message"
	"github.com/marang/franz-agent/internal/oauth"
	"github.com/marang/franz-agent/internal/permission"
	"github.com/marang/franz-agent/internal/session"
	"github.com/marang/franz-agent/internal/ui/common"
	"github.com/marang/franz-agent/internal/ui/util"
)

// ActionClose is a message to close the current dialog.
type ActionClose struct{}

// ActionQuit is a message to quit the application.
type ActionQuit = tea.QuitMsg

// ActionOpenDialog is a message to open a dialog.
type ActionOpenDialog struct {
	DialogID string
}

// ActionOpenConfirm opens a generic confirmation dialog with a payload that is
// returned on accept.
type ActionOpenConfirm struct {
	Title   string
	Message string
	Payload any
}

// ActionConfirmChoice is emitted by the generic confirmation dialog.
type ActionConfirmChoice struct {
	Confirmed bool
	Payload   any
}

// ActionSelectSession is a message indicating a session has been selected.
type ActionSelectSession struct {
	Session session.Session
}

// ActionSelectModel is a message indicating a model has been selected.
type ActionSelectModel struct {
	Provider       catwalk.Provider
	Model          config.SelectedModel
	ModelType      config.SelectedModelType
	ReAuthenticate bool
}

// Messages for commands
type (
	ActionNewSession        struct{}
	ActionToggleHelp        struct{}
	ActionToggleCompactMode struct{}
	ActionToggleThinking    struct{}
	ActionTogglePills       struct{}
	ActionExternalEditor    struct{}
	ActionToggleYoloMode    struct{}
	ActionTogglePlanMode    struct{}
	ActionSkillsList        struct{}
	ActionSkillsSHSources   struct{}
	ActionSkillsSHUpdate    struct{}
	ActionSkillsSHSearch    struct {
		Query     string
		Arguments []commands.Argument
		Args      map[string]string
	}
	ActionSkillsSHSearchRequest struct {
		Query     string
		RequestID int
	}
	ActionSkillsInstalledRefreshRequest struct{}
	ActionSkillsSHSourcesRefreshRequest struct{}
	ActionSkillsSHInstall               struct {
		Source    string
		Arguments []commands.Argument
		Args      map[string]string
	}
	ActionSkillsSHInstallSelected struct {
		Sources []string
	}
	ActionSkillsSHInstallSource struct {
		Source string
	}
	ActionSkillsSetDisabled struct {
		Name     string
		Disabled bool
	}
	ActionSkillsSetDisabledBatch struct {
		Names    []string
		Disabled bool
	}
	ActionSkillsDelete struct {
		Name string
	}
	ActionSkillsDeleteBatch struct {
		Names []string
	}
	ActionSkillsFixPerms struct {
		Names []string
	}
	ActionSkillsSHOpenDetails struct {
		URL string
	}
	ActionToggleNotifications         struct{}
	ActionToggleTransparentBackground struct{}
	ActionInitializeProject           struct{}
	ActionSummarize                   struct {
		SessionID string
	}
	// ActionSelectReasoningEffort is a message indicating a reasoning effort
	// has been selected.
	ActionSelectReasoningEffort struct {
		Effort string
	}
	ActionPermissionResponse struct {
		Permission permission.PermissionRequest
		Action     PermissionAction
	}
	// ActionRunCustomCommand is a message to run a custom command.
	ActionRunCustomCommand struct {
		Content   string
		Arguments []commands.Argument
		Args      map[string]string // Actual argument values
	}
	// ActionRunMCPPrompt is a message to run a custom command.
	ActionRunMCPPrompt struct {
		Title       string
		Description string
		PromptID    string
		ClientID    string
		Arguments   []commands.Argument
		Args        map[string]string // Actual argument values
	}
	// ActionEnableDockerMCP is a message to enable Docker MCP.
	ActionEnableDockerMCP struct{}
	// ActionDisableDockerMCP is a message to disable Docker MCP.
	ActionDisableDockerMCP struct{}
)

// Messages for API key input dialog.
type (
	ActionChangeAPIKeyState struct {
		State APIKeyInputState
	}
)

// Messages for OAuth2 device flow dialog.
type (
	// ActionInitiateOAuth is sent when the device auth is initiated
	// successfully.
	ActionInitiateOAuth struct {
		DeviceCode      string
		UserCode        string
		ExpiresIn       int
		VerificationURL string
		Interval        int
	}

	// ActionCompleteOAuth is sent when the device flow completes successfully.
	ActionCompleteOAuth struct {
		Token *oauth.Token
	}

	// ActionOAuthErrored is sent when the device flow encounters an error.
	ActionOAuthErrored struct {
		Error error
	}
)

// ActionCmd represents an action that carries a [tea.Cmd] to be passed to the
// Bubble Tea program loop.
type ActionCmd struct {
	Cmd tea.Cmd
}

// ActionFilePickerSelected is a message indicating a file has been selected in
// the file picker dialog.
type ActionFilePickerSelected struct {
	Path string
}

// SkillsSHSearchResult contains a normalized search result used by the TUI.
type SkillsSHSearchResult struct {
	Name          string
	Source        string
	Slug          string
	SkillID       string
	Installs      int
	InstallSource string
	DetailsURL    string
}

// SkillsSHSearchResultsMsg carries search results back to the dialog.
type SkillsSHSearchResultsMsg struct {
	Query     string
	RequestID int
	Results   []SkillsSHSearchResult
	Err       error
}

// SkillsSHSourcesLoadedMsg carries tracked sources back to the dialog.
type SkillsSHSourcesLoadedMsg struct {
	Sources []string
	Err     error
}

// SkillsInstalledItem represents one installed/discovered skill entry.
type SkillsInstalledItem struct {
	Name               string
	Description        string
	Path               string
	SkillFile          string
	Disabled           bool
	Blocked            bool
	BlockReasons       []string
	PermissionWarnings []string
	Origin             string
}

// SkillsInstalledLoadedMsg carries installed skills back to the dialog.
type SkillsInstalledLoadedMsg struct {
	Items []SkillsInstalledItem
	Err   error
}

// SkillsSHInstallCompletedMsg carries install outcomes back to the dialog.
type SkillsSHInstallCompletedMsg struct {
	Installed []string
	Failed    map[string]string
}

// SkillsSHInstallStepCompletedMsg carries one install-step result.
type SkillsSHInstallStepCompletedMsg struct {
	Source string
	Err    error
}

// Cmd returns a command that reads the file at path and sends a
// [message.Attachement] to the program.
func (a ActionFilePickerSelected) Cmd() tea.Cmd {
	path := a.Path
	if path == "" {
		return nil
	}
	return func() tea.Msg {
		isFileLarge, err := common.IsFileTooBig(path, common.MaxAttachmentSize)
		if err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  fmt.Sprintf("unable to read the file: %v", err),
			}
		}
		if isFileLarge {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  "file too large, max 5MB",
			}
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  fmt.Sprintf("unable to read the file: %v", err),
			}
		}

		mimeBufferSize := min(512, len(content))
		mimeType := http.DetectContentType(content[:mimeBufferSize])
		fileName := filepath.Base(path)

		return message.Attachment{
			FilePath: path,
			FileName: fileName,
			MimeType: mimeType,
			Content:  content,
		}
	}
}
