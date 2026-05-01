package model

import (
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/ultraviolet/layout"
	"github.com/charmbracelet/ultraviolet/screen"
	"github.com/charmbracelet/x/editor"
	"github.com/marang/franz-agent/internal/agent"
	"github.com/marang/franz-agent/internal/agent/notify"
	"github.com/marang/franz-agent/internal/agent/tools/mcp"
	"github.com/marang/franz-agent/internal/app"
	"github.com/marang/franz-agent/internal/commands"
	"github.com/marang/franz-agent/internal/config"
	"github.com/marang/franz-agent/internal/fsext"
	"github.com/marang/franz-agent/internal/history"
	"github.com/marang/franz-agent/internal/home"
	"github.com/marang/franz-agent/internal/message"
	"github.com/marang/franz-agent/internal/oauth/openai_codex"
	"github.com/marang/franz-agent/internal/openaicodex"
	"github.com/marang/franz-agent/internal/permission"
	"github.com/marang/franz-agent/internal/pubsub"
	"github.com/marang/franz-agent/internal/session"
	"github.com/marang/franz-agent/internal/subscription"
	"github.com/marang/franz-agent/internal/ui/anim"
	"github.com/marang/franz-agent/internal/ui/attachments"
	"github.com/marang/franz-agent/internal/ui/chat"
	"github.com/marang/franz-agent/internal/ui/common"
	"github.com/marang/franz-agent/internal/ui/completions"
	"github.com/marang/franz-agent/internal/ui/dialog"
	fimage "github.com/marang/franz-agent/internal/ui/image"
	"github.com/marang/franz-agent/internal/ui/logo"
	"github.com/marang/franz-agent/internal/ui/notification"
	"github.com/marang/franz-agent/internal/ui/styles"
	"github.com/marang/franz-agent/internal/ui/util"
	"github.com/marang/franz-agent/internal/version"
)

// MouseScrollThreshold defines how many lines to scroll the chat when a mouse
// wheel event occurs.
const MouseScrollThreshold = 5

// Compact mode breakpoints.
const (
	compactModeWidthBreakpoint  = 120
	compactModeHeightBreakpoint = 30
)

// If pasted text has more than 10 newlines, treat it as a file attachment.
const pasteLinesThreshold = 10

// If pasted text has more than 1000 columns, treat it as a file attachment.
const pasteColsThreshold = 1000

// Session details panel max height.
const sessionDetailsMaxHeight = 20

// TextareaMaxHeight is the maximum height of the prompt textarea.
const TextareaMaxHeight = 15

// editorHeightMargin is the vertical margin added to the textarea height to
// account for the attachments row (top) and bottom margin.
const editorHeightMargin = 2

// TextareaMinHeight is the minimum height of the prompt textarea.
const (
	TextareaMinHeight  = 3
	localStatusCommand = "/status"
	localPlanCommand   = "/plan"
	localBTWCommand    = "/btw"
)

// uiFocusState represents the current focus state of the UI.
type uiFocusState uint8

// Possible uiFocusState values.
const (
	uiFocusNone uiFocusState = iota
	uiFocusEditor
	uiFocusMain
)

type uiState uint8

// Possible uiState values.
const (
	uiOnboarding uiState = iota
	uiInitialize
	uiLanding
	uiChat
)

type openEditorMsg struct {
	Text string
}

type (
	// cancelTimerExpiredMsg is sent when the cancel timer expires.
	cancelTimerExpiredMsg struct{}
	// userCommandsLoadedMsg is sent when user commands are loaded.
	userCommandsLoadedMsg struct {
		Commands []commands.CustomCommand
	}
	// mcpPromptsLoadedMsg is sent when mcp prompts are loaded.
	mcpPromptsLoadedMsg struct {
		Prompts []commands.MCPPrompt
	}
	// mcpStateChangedMsg is sent when there is a change in MCP client states.
	mcpStateChangedMsg struct {
		states map[string]mcp.ClientInfo
	}
	// sendMessageMsg is sent to send a message.
	// currently only used for mcp prompts.
	sendMessageMsg struct {
		Content     string
		Attachments []message.Attachment
	}

	// closeDialogMsg is sent to close the current dialog.
	closeDialogMsg struct{}

	// copyChatHighlightMsg is sent to copy the current chat highlight to clipboard.
	copyChatHighlightMsg struct{}

	// sessionFilesUpdatesMsg is sent when the files for this session have been updated
	sessionFilesUpdatesMsg struct {
		sessionFiles []SessionFile
	}

	// subscriptionUsageLoadedMsg is sent when subscription usage has been fetched.
	subscriptionUsageLoadedMsg struct {
		providerID string
		report     *subscription.UsageReport
		err        error
		fetchedAt  time.Time
	}

	// subscriptionUsageTickMsg triggers periodic subscription usage refresh.
	subscriptionUsageTickMsg struct{}
)

// UI represents the main user interface model.
type UI struct {
	com          *common.Common
	session      *session.Session
	sessionFiles []SessionFile

	// keeps track of read files while we don't have a session id
	sessionFileReads []string

	// initialSessionID is set when loading a specific session on startup.
	initialSessionID string
	// continueLastSession is set to continue the most recent session on startup.
	continueLastSession bool

	lastUserMessageTime int64

	// The width and height of the terminal in cells.
	width  int
	height int
	layout uiLayout

	isTransparent bool

	focus uiFocusState
	state uiState

	keyMap KeyMap
	keyenh tea.KeyboardEnhancementsMsg

	dialog *dialog.Overlay
	status *Status

	// isCanceling tracks whether the user has pressed escape once to cancel.
	isCanceling bool

	header *header

	// sendProgressBar instructs the TUI to send progress bar updates to the
	// terminal.
	sendProgressBar    bool
	progressBarEnabled bool

	// caps hold different terminal capabilities that we query for.
	caps common.Capabilities

	// Editor components
	textarea textarea.Model

	// Attachment list
	attachments *attachments.Attachments

	readyPlaceholder   string
	workingPlaceholder string

	// Completions state
	completions              *completions.Completions
	completionsOpen          bool
	completionsStartIndex    int
	completionsQuery         string
	completionsPositionStart image.Point // x,y where user typed '@'

	// Chat components
	chat *Chat

	// onboarding state
	onboarding struct {
		yesInitializeSelected bool
	}

	// lsp
	lspStates map[string]app.LSPClientInfo

	// mcp
	mcpStates map[string]mcp.ClientInfo

	// sidebarLogo keeps a cached version of the sidebar sidebarLogo.
	sidebarLogo string

	// Notification state
	notifyBackend       notification.Backend
	notifyWindowFocused bool
	// custom commands & mcp commands
	customCommands []commands.CustomCommand
	mcpPrompts     []commands.MCPPrompt

	// planModeEnabled toggles planning-only responses (no tool execution).
	planModeEnabled bool
	// btwModeEnabled toggles one-shot BTW mode for the next sent message.
	btwModeEnabled bool

	// forceCompactMode tracks whether compact mode is forced by user toggle
	forceCompactMode bool
	// forceWideMode tracks whether wide mode is forced by user toggle.
	forceWideMode bool

	// isCompact tracks whether we're currently in compact layout mode (either
	// by user toggle or auto-switch based on window size)
	isCompact bool

	// detailsOpen tracks whether the details panel is open (in compact mode)
	detailsOpen bool

	// pills state
	pillsExpanded      bool
	focusedPillSection pillSection
	promptQueue        int
	pillsView          string

	// Todo spinner
	todoSpinner    spinner.Model
	todoIsSpinning bool

	// mouse highlighting related state
	lastClickTime time.Time

	// Prompt history for up/down navigation through previous messages.
	promptHistory struct {
		messages []string
		index    int
		draft    string
	}

	// Sidebar subscription usage state.
	subscriptionUsage           *subscription.UsageReport
	subscriptionUsageProviderID string
	subscriptionUsageLoading    bool
	subscriptionUsageError      string
	subscriptionUsageFetchedAt  time.Time

	// Tracks assistant message IDs that already triggered a re-auth dialog.
	reauthPromptedMessageIDs map[string]struct{}
}

// New creates a new instance of the [UI] model.
func New(com *common.Common, initialSessionID string, continueLast bool) *UI {
	// Editor components
	ta := textarea.New()
	ta.SetStyles(com.Styles.TextArea)
	ta.ShowLineNumbers = false
	ta.CharLimit = -1
	ta.SetVirtualCursor(false)
	ta.DynamicHeight = true
	ta.MinHeight = TextareaMinHeight
	ta.MaxHeight = TextareaMaxHeight
	ta.Focus()

	ch := NewChat(com)

	keyMap := DefaultKeyMap()

	// Completions component
	comp := completions.New(
		com.Styles.Completions.Normal,
		com.Styles.Completions.Focused,
		com.Styles.Completions.Match,
	)

	todoSpinner := spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(com.Styles.Pills.TodoSpinner),
	)

	// Attachments component
	attachments := attachments.New(
		attachments.NewRenderer(
			com.Styles.Attachments.Normal,
			com.Styles.Attachments.Deleting,
			com.Styles.Attachments.Image,
			com.Styles.Attachments.Text,
		),
		attachments.Keymap{
			DeleteMode: keyMap.Editor.AttachmentDeleteMode,
			DeleteAll:  keyMap.Editor.DeleteAllAttachments,
			Escape:     keyMap.Editor.Escape,
		},
	)

	header := newHeader(com)

	ui := &UI{
		com:                      com,
		dialog:                   dialog.NewOverlay(),
		keyMap:                   keyMap,
		textarea:                 ta,
		chat:                     ch,
		header:                   header,
		completions:              comp,
		attachments:              attachments,
		todoSpinner:              todoSpinner,
		lspStates:                make(map[string]app.LSPClientInfo),
		mcpStates:                make(map[string]mcp.ClientInfo),
		notifyBackend:            notification.NoopBackend{},
		notifyWindowFocused:      true,
		initialSessionID:         initialSessionID,
		continueLastSession:      continueLast,
		reauthPromptedMessageIDs: make(map[string]struct{}),
	}

	status := NewStatus(com, ui)

	ui.setEditorPrompt(false, false)
	ui.randomizePlaceholders()
	ui.textarea.Placeholder = ui.readyPlaceholder
	ui.status = status

	// Initialize compact mode from config
	ui.forceCompactMode = com.Config().Options.TUI.CompactMode

	// set onboarding state defaults
	ui.onboarding.yesInitializeSelected = true

	desiredState := uiLanding
	desiredFocus := uiFocusEditor
	if !com.Config().IsConfigured() {
		desiredState = uiOnboarding
	} else if n, _ := config.ProjectNeedsInitialization(com.Store()); n {
		desiredState = uiInitialize
	}

	// set initial state
	ui.setState(desiredState, desiredFocus)

	opts := com.Config().Options

	// disable indeterminate progress bar
	ui.progressBarEnabled = opts.Progress == nil || *opts.Progress
	// enable transparent mode
	ui.isTransparent = opts.TUI.Transparent != nil && *opts.TUI.Transparent

	return ui
}

// Init initializes the UI model.
func (m *UI) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.state == uiOnboarding {
		if cmd := m.openModelsDialog(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	// load the user commands async
	cmds = append(cmds, m.loadCustomCommands())
	// load prompt history async
	cmds = append(cmds, m.loadPromptHistory())
	// load initial session if specified
	if cmd := m.loadInitialSession(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	cmds = append(cmds, m.subscriptionUsageTick())
	return tea.Batch(cmds...)
}

// loadInitialSession loads the initial session if one was specified on startup.
func (m *UI) loadInitialSession() tea.Cmd {
	switch {
	case m.state != uiLanding:
		// Only load if we're in landing state (i.e., fully configured)
		return nil
	case m.initialSessionID != "":
		return m.loadSession(m.initialSessionID)
	case m.continueLastSession:
		return func() tea.Msg {
			sess, err := m.com.App.Sessions.GetLast(context.Background())
			if err != nil {
				return nil
			}
			return m.loadSession(sess.ID)()
		}
	default:
		return nil
	}
}

// sendNotification returns a command that sends a notification if allowed by policy.
func (m *UI) sendNotification(n notification.Notification) tea.Cmd {
	if !m.shouldSendNotification() {
		return nil
	}

	backend := m.notifyBackend
	return func() tea.Msg {
		if err := backend.Send(n); err != nil {
			slog.Error("Failed to send notification", "error", err)
		}
		return nil
	}
}

// shouldSendNotification returns true if notifications should be sent based on
// current state. Focus reporting must be supported, window must not focused,
// and notifications must not be disabled in config.
func (m *UI) shouldSendNotification() bool {
	cfg := m.com.Config()
	if cfg != nil && cfg.Options != nil && cfg.Options.DisableNotifications {
		return false
	}
	return m.caps.ReportFocusEvents && !m.notifyWindowFocused
}

// setState changes the UI state and focus.
func (m *UI) setState(state uiState, focus uiFocusState) {
	if state == uiLanding {
		// Always turn off compact mode when going to landing
		m.isCompact = false
	}
	m.state = state
	m.focus = focus
	// Changing the state may change layout, so update it.
	m.updateLayoutAndSize()
}

// loadCustomCommands loads the custom commands asynchronously.
func (m *UI) loadCustomCommands() tea.Cmd {
	return func() tea.Msg {
		customCommands, err := commands.LoadCustomCommands(m.com.Config())
		if err != nil {
			slog.Error("Failed to load custom commands", "error", err)
		}
		return userCommandsLoadedMsg{Commands: customCommands}
	}
}

// loadMCPrompts loads the MCP prompts asynchronously.
func (m *UI) loadMCPrompts() tea.Msg {
	prompts, err := commands.LoadMCPPrompts()
	if err != nil {
		slog.Error("Failed to load MCP prompts", "error", err)
	}
	if prompts == nil {
		// flag them as loaded even if there is none or an error
		prompts = []commands.MCPPrompt{}
	}
	return mcpPromptsLoadedMsg{Prompts: prompts}
}

// Update handles updates to the UI model.
func (m *UI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if m.hasSession() && m.isAgentBusy() {
		queueSize := m.com.App.AgentCoordinator.QueuedPrompts(m.session.ID)
		if queueSize != m.promptQueue {
			m.promptQueue = queueSize
			m.updateLayoutAndSize()
		}
	}
	// Update terminal capabilities
	m.caps.Update(msg)
	switch msg := msg.(type) {
	case tea.EnvMsg:
		// Is this Windows Terminal?
		if !m.sendProgressBar {
			m.sendProgressBar = slices.Contains(msg, "WT_SESSION")
		}
		cmds = append(cmds, common.QueryCmd(uv.Environ(msg)))
	case tea.ModeReportMsg:
		if m.caps.ReportFocusEvents {
			m.notifyBackend = notification.NewNativeBackend(notification.Icon)
		}
	case tea.FocusMsg:
		m.notifyWindowFocused = true
	case tea.BlurMsg:
		m.notifyWindowFocused = false
	case pubsub.Event[notify.Notification]:
		if cmd := m.handleAgentNotification(msg.Payload); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case loadSessionMsg:
		if m.forceCompactMode {
			m.isCompact = true
		}
		m.setState(uiChat, m.focus)
		m.session = msg.session
		m.sessionFiles = msg.files
		cmds = append(cmds, m.startLSPs(msg.lspFilePaths()))
		msgs, err := m.com.App.Messages.List(context.Background(), m.session.ID)
		if err != nil {
			cmds = append(cmds, util.ReportError(err))
			break
		}
		if cmd := m.setSessionMessages(msgs); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if hasInProgressTodo(m.session.Todos) {
			// only start spinner if there is an in-progress todo
			if m.isAgentBusy() {
				m.todoIsSpinning = true
				cmds = append(cmds, m.todoSpinner.Tick)
			}
			m.updateLayoutAndSize()
		}
		// Reload prompt history for the new session.
		m.historyReset()
		cmds = append(cmds, m.loadPromptHistory())
		if cmd := m.refreshSubscriptionUsage(true); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.updateLayoutAndSize()

	case sessionFilesUpdatesMsg:
		m.sessionFiles = msg.sessionFiles
		var paths []string
		for _, f := range msg.sessionFiles {
			paths = append(paths, f.LatestVersion.Path)
		}
		cmds = append(cmds, m.startLSPs(paths))

	case sendMessageMsg:
		cmds = append(cmds, m.sendMessage(msg.Content, msg.Attachments...))

	case userCommandsLoadedMsg:
		m.customCommands = msg.Commands
		dia := m.dialog.Dialog(dialog.CommandsID)
		if dia == nil {
			break
		}

		commands, ok := dia.(*dialog.Commands)
		if ok {
			commands.SetCustomCommands(m.customCommands)
		}

	case mcpStateChangedMsg:
		m.mcpStates = msg.states
	case mcpPromptsLoadedMsg:
		m.mcpPrompts = msg.Prompts
		dia := m.dialog.Dialog(dialog.CommandsID)
		if dia == nil {
			break
		}

		commands, ok := dia.(*dialog.Commands)
		if ok {
			commands.SetMCPPrompts(m.mcpPrompts)
		}

	case promptHistoryLoadedMsg:
		m.promptHistory.messages = msg.messages
		m.promptHistory.index = -1
		m.promptHistory.draft = ""

	case subscriptionUsageTickMsg:
		if cmd := m.refreshSubscriptionUsage(false); cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.subscriptionUsageTick())

	case subscriptionUsageLoadedMsg:
		m.subscriptionUsageLoading = false
		m.subscriptionUsageProviderID = msg.providerID
		m.subscriptionUsageFetchedAt = msg.fetchedAt
		if msg.err != nil {
			m.subscriptionUsage = nil
			m.subscriptionUsageError = msg.err.Error()
		} else {
			m.subscriptionUsage = msg.report
			m.subscriptionUsageError = ""
		}

	case closeDialogMsg:
		m.dialog.CloseFrontDialog()

	case pubsub.Event[session.Session]:
		if msg.Type == pubsub.DeletedEvent {
			if m.session != nil && m.session.ID == msg.Payload.ID {
				if cmd := m.newSession(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
			break
		}
		if m.session != nil && msg.Payload.ID == m.session.ID {
			prevHasInProgress := hasInProgressTodo(m.session.Todos)
			m.session = &msg.Payload
			if !prevHasInProgress && hasInProgressTodo(m.session.Todos) {
				m.todoIsSpinning = true
				cmds = append(cmds, m.todoSpinner.Tick)
				m.updateLayoutAndSize()
			}
		}
	case pubsub.Event[message.Message]:
		// Check if this is a child session message for an agent tool.
		if m.session == nil {
			break
		}
		if msg.Payload.SessionID != m.session.ID {
			// This might be a child session message from an agent tool.
			if cmd := m.handleChildSessionMessage(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
			break
		}
		switch msg.Type {
		case pubsub.CreatedEvent:
			cmds = append(cmds, m.appendSessionMessage(msg.Payload))
		case pubsub.UpdatedEvent:
			cmds = append(cmds, m.updateSessionMessage(msg.Payload))
		case pubsub.DeletedEvent:
			m.chat.RemoveMessage(msg.Payload.ID)
		}
		// start the spinner if there is a new message
		if hasInProgressTodo(m.session.Todos) && m.isAgentBusy() && !m.todoIsSpinning {
			m.todoIsSpinning = true
			cmds = append(cmds, m.todoSpinner.Tick)
		}
		// stop the spinner if the agent is not busy anymore
		if m.todoIsSpinning && !m.isAgentBusy() {
			m.todoIsSpinning = false
		}
		// there is a number of things that could change the pills here so we want to re-render
		m.renderPills()
	case pubsub.Event[history.File]:
		cmds = append(cmds, m.handleFileEvent(msg.Payload))
	case pubsub.Event[app.LSPEvent]:
		m.lspStates = app.GetLSPStates()
	case pubsub.Event[mcp.Event]:
		switch msg.Payload.Type {
		case mcp.EventStateChanged:
			return m, tea.Batch(
				m.handleStateChanged(),
				m.loadMCPrompts,
			)
		case mcp.EventPromptsListChanged:
			return m, handleMCPPromptsEvent(msg.Payload.Name)
		case mcp.EventToolsListChanged:
			return m, handleMCPToolsEvent(m.com.Store(), msg.Payload.Name)
		case mcp.EventResourcesListChanged:
			return m, handleMCPResourcesEvent(msg.Payload.Name)
		}
	case pubsub.Event[permission.PermissionRequest]:
		if cmd := m.openPermissionsDialog(msg.Payload); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if cmd := m.sendNotification(notification.Notification{
			Title:   "Franz is waiting...",
			Message: fmt.Sprintf("Permission required to execute \"%s\"", msg.Payload.ToolName),
		}); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case pubsub.Event[permission.PermissionNotification]:
		m.handlePermissionNotification(msg.Payload)
	case cancelTimerExpiredMsg:
		m.isCanceling = false
	case tea.TerminalVersionMsg:
		termVersion := strings.ToLower(msg.Name)
		// Only enable progress bar for the following terminals.
		if !m.sendProgressBar {
			m.sendProgressBar = strings.Contains(termVersion, "ghostty")
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.updateLayoutAndSize()
		if m.state == uiChat && m.chat.Follow() {
			if cmd := m.chat.ScrollToBottomAndAnimate(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	case tea.KeyboardEnhancementsMsg:
		m.keyenh = msg
		if msg.SupportsKeyDisambiguation() {
			m.keyMap.Models.SetHelp("ctrl+m", "models")
		}
	case copyChatHighlightMsg:
		cmds = append(cmds, m.copyChatHighlight())
	case DelayedClickMsg:
		// Handle delayed single-click action (e.g., expansion).
		m.chat.HandleDelayedClick(msg)
	case tea.MouseClickMsg:
		// Pass mouse events to dialogs first if any are open.
		if m.dialog.HasDialogs() {
			m.dialog.Update(msg)
			return m, tea.Batch(cmds...)
		}

		if cmd := m.handleClickFocus(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}

		switch m.state {
		case uiChat:
			x, y := msg.X, msg.Y
			// Adjust for chat area position
			x -= m.layout.main.Min.X
			y -= m.layout.main.Min.Y
			if !image.Pt(msg.X, msg.Y).In(m.layout.sidebar) {
				if handled, cmd := m.chat.HandleMouseDown(x, y); handled {
					m.lastClickTime = time.Now()
					if cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			}
		}

	case tea.MouseMotionMsg:
		// Pass mouse events to dialogs first if any are open.
		if m.dialog.HasDialogs() {
			m.dialog.Update(msg)
			return m, tea.Batch(cmds...)
		}

		switch m.state {
		case uiChat:
			if msg.Y <= 0 {
				if cmd := m.chat.ScrollByAndAnimate(-1); cmd != nil {
					cmds = append(cmds, cmd)
				}
				if !m.chat.SelectedItemInView() {
					m.chat.SelectPrev()
					if cmd := m.chat.ScrollToSelectedAndAnimate(); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			} else if msg.Y >= m.chat.Height()-1 {
				if cmd := m.chat.ScrollByAndAnimate(1); cmd != nil {
					cmds = append(cmds, cmd)
				}
				if !m.chat.SelectedItemInView() {
					m.chat.SelectNext()
					if cmd := m.chat.ScrollToSelectedAndAnimate(); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			}

			x, y := msg.X, msg.Y
			// Adjust for chat area position
			x -= m.layout.main.Min.X
			y -= m.layout.main.Min.Y
			m.chat.HandleMouseDrag(x, y)
		}

	case tea.MouseReleaseMsg:
		// Pass mouse events to dialogs first if any are open.
		if m.dialog.HasDialogs() {
			m.dialog.Update(msg)
			return m, tea.Batch(cmds...)
		}

		switch m.state {
		case uiChat:
			x, y := msg.X, msg.Y
			// Adjust for chat area position
			x -= m.layout.main.Min.X
			y -= m.layout.main.Min.Y
			if m.chat.HandleMouseUp(x, y) && m.chat.HasHighlight() {
				cmds = append(cmds, tea.Tick(doubleClickThreshold, func(t time.Time) tea.Msg {
					if time.Since(m.lastClickTime) >= doubleClickThreshold {
						return copyChatHighlightMsg{}
					}
					return nil
				}))
			}
		}
	case tea.MouseWheelMsg:
		// Pass mouse events to dialogs first if any are open.
		if m.dialog.HasDialogs() {
			m.dialog.Update(msg)
			return m, tea.Batch(cmds...)
		}

		// Otherwise handle mouse wheel for chat.
		switch m.state {
		case uiChat:
			switch msg.Button {
			case tea.MouseWheelUp:
				if cmd := m.chat.ScrollByAndAnimate(-MouseScrollThreshold); cmd != nil {
					cmds = append(cmds, cmd)
				}
				if !m.chat.SelectedItemInView() {
					m.chat.SelectPrev()
					if cmd := m.chat.ScrollToSelectedAndAnimate(); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			case tea.MouseWheelDown:
				if cmd := m.chat.ScrollByAndAnimate(MouseScrollThreshold); cmd != nil {
					cmds = append(cmds, cmd)
				}
				if !m.chat.SelectedItemInView() {
					if m.chat.AtBottom() {
						m.chat.SelectLast()
					} else {
						m.chat.SelectNext()
					}
					if cmd := m.chat.ScrollToSelectedAndAnimate(); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			}
		}
	case anim.StepMsg:
		if m.state == uiChat {
			if cmd := m.chat.Animate(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
			if m.chat.Follow() {
				if cmd := m.chat.ScrollToBottomAndAnimate(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
	case spinner.TickMsg:
		if m.dialog.HasDialogs() {
			// route to dialog
			if cmd := m.handleDialogMsg(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if m.state == uiChat && m.hasSession() && hasInProgressTodo(m.session.Todos) && m.todoIsSpinning {
			var cmd tea.Cmd
			m.todoSpinner, cmd = m.todoSpinner.Update(msg)
			if cmd != nil {
				m.renderPills()
				cmds = append(cmds, cmd)
			}
		}

	case tea.KeyPressMsg:
		if cmd := m.handleKeyPressMsg(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case tea.PasteMsg:
		if cmd := m.handlePasteMsg(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case openEditorMsg:
		prevHeight := m.textarea.Height()
		m.textarea.SetValue(msg.Text)
		m.textarea.MoveToEnd()
		cmds = append(cmds, m.updateTextareaWithPrevHeight(msg, prevHeight))
	case util.InfoMsg:
		m.status.SetInfoMsg(msg)
		ttl := msg.TTL
		if ttl <= 0 {
			ttl = DefaultStatusTTL
		}
		cmds = append(cmds, clearInfoMsgCmd(ttl))
	case util.ClearStatusMsg:
		m.status.ClearInfoMsg()
	case completions.CompletionItemsLoadedMsg:
		if m.completionsOpen {
			m.completions.SetItems(msg.Files, msg.Resources)
		}
	case uv.KittyGraphicsEvent:
		if !bytes.HasPrefix(msg.Payload, []byte("OK")) {
			slog.Warn("Unexpected Kitty graphics response",
				"response", string(msg.Payload),
				"options", msg.Options)
		}
	default:
		if m.dialog.HasDialogs() {
			if cmd := m.handleDialogMsg(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	// This logic gets triggered on any message type, but should it?
	switch m.focus {
	case uiFocusMain:
	case uiFocusEditor:
		// Textarea placeholder logic
		if m.isAgentBusy() {
			m.textarea.Placeholder = m.workingPlaceholder
		} else {
			m.textarea.Placeholder = m.readyPlaceholder
		}
		if m.com.App.Permissions.SkipRequests() {
			m.textarea.Placeholder = "Yolo mode!"
		}
	}

	// at this point this can only handle [message.Attachment] message, and we
	// should return all cmds anyway.
	_ = m.attachments.Update(msg)
	return m, tea.Batch(cmds...)
}

// setSessionMessages sets the messages for the current session in the chat
func (m *UI) setSessionMessages(msgs []message.Message) tea.Cmd {
	var cmds []tea.Cmd
	// Build tool result map to link tool calls with their results
	msgPtrs := make([]*message.Message, len(msgs))
	for i := range msgs {
		msgPtrs[i] = &msgs[i]
	}
	toolResultMap := chat.BuildToolResultMap(msgPtrs)
	if len(msgPtrs) > 0 {
		m.lastUserMessageTime = msgPtrs[0].CreatedAt
	}

	// Add messages to chat with linked tool results
	items := make([]chat.MessageItem, 0, len(msgs)*2)
	for _, msg := range msgPtrs {
		switch msg.Role {
		case message.User:
			m.lastUserMessageTime = msg.CreatedAt
			items = append(items, chat.ExtractMessageItems(m.com.Styles, msg, toolResultMap)...)
		case message.Assistant:
			items = append(items, chat.ExtractMessageItems(m.com.Styles, msg, toolResultMap)...)
			if msg.FinishPart() != nil && msg.FinishPart().Reason == message.FinishReasonEndTurn {
				infoItem := chat.NewAssistantInfoItem(m.com.Styles, msg, m.com.Config(), time.Unix(m.lastUserMessageTime, 0))
				items = append(items, infoItem)
			}
		default:
			items = append(items, chat.ExtractMessageItems(m.com.Styles, msg, toolResultMap)...)
		}
	}

	// Load nested tool calls for agent/agentic_fetch tools.
	m.loadNestedToolCalls(items)

	// If the user switches between sessions while the agent is working we want
	// to make sure the animations are shown.
	for _, item := range items {
		if animatable, ok := item.(chat.Animatable); ok {
			if cmd := animatable.StartAnimation(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	m.chat.SetMessages(items...)
	if cmd := m.chat.ScrollToBottomAndAnimate(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.chat.SelectLast()
	return tea.Sequence(cmds...)
}

// loadNestedToolCalls recursively loads nested tool calls for agent/agentic_fetch tools.
func (m *UI) loadNestedToolCalls(items []chat.MessageItem) {
	for _, item := range items {
		nestedContainer, ok := item.(chat.NestedToolContainer)
		if !ok {
			continue
		}
		toolItem, ok := item.(chat.ToolMessageItem)
		if !ok {
			continue
		}

		tc := toolItem.ToolCall()
		messageID := toolItem.MessageID()

		// Get the agent tool session ID.
		agentSessionID := m.com.App.Sessions.CreateAgentToolSessionID(messageID, tc.ID)

		// Fetch nested messages.
		nestedMsgs, err := m.com.App.Messages.List(context.Background(), agentSessionID)
		if err != nil || len(nestedMsgs) == 0 {
			continue
		}

		// Build tool result map for nested messages.
		nestedMsgPtrs := make([]*message.Message, len(nestedMsgs))
		for i := range nestedMsgs {
			nestedMsgPtrs[i] = &nestedMsgs[i]
		}
		nestedToolResultMap := chat.BuildToolResultMap(nestedMsgPtrs)

		// Extract nested tool items.
		var nestedTools []chat.ToolMessageItem
		for _, nestedMsg := range nestedMsgPtrs {
			nestedItems := chat.ExtractMessageItems(m.com.Styles, nestedMsg, nestedToolResultMap)
			for _, nestedItem := range nestedItems {
				if nestedToolItem, ok := nestedItem.(chat.ToolMessageItem); ok {
					// Mark nested tools as simple (compact) rendering.
					if simplifiable, ok := nestedToolItem.(chat.Compactable); ok {
						simplifiable.SetCompact(true)
					}
					nestedTools = append(nestedTools, nestedToolItem)
				}
			}
		}

		// Recursively load nested tool calls for any agent tools within.
		nestedMessageItems := make([]chat.MessageItem, len(nestedTools))
		for i, nt := range nestedTools {
			nestedMessageItems[i] = nt
		}
		m.loadNestedToolCalls(nestedMessageItems)

		// Set nested tools on the parent.
		nestedContainer.SetNestedTools(nestedTools)
	}
}

// appendSessionMessage appends a new message to the current session in the chat
// if the message is a tool result it will update the corresponding tool call message
func (m *UI) appendSessionMessage(msg message.Message) tea.Cmd {
	var cmds []tea.Cmd

	existing := m.chat.MessageItem(msg.ID)
	if existing != nil {
		// message already exists, skip
		return nil
	}

	switch msg.Role {
	case message.User:
		m.lastUserMessageTime = msg.CreatedAt
		items := chat.ExtractMessageItems(m.com.Styles, &msg, nil)
		for _, item := range items {
			if animatable, ok := item.(chat.Animatable); ok {
				if cmd := animatable.StartAnimation(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
		m.chat.AppendMessages(items...)
		if cmd := m.chat.ScrollToBottomAndAnimate(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case message.Assistant:
		items := chat.ExtractMessageItems(m.com.Styles, &msg, nil)
		for _, item := range items {
			if animatable, ok := item.(chat.Animatable); ok {
				if cmd := animatable.StartAnimation(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
		m.chat.AppendMessages(items...)
		if m.chat.Follow() {
			if cmd := m.chat.ScrollToBottomAndAnimate(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if msg.FinishPart() != nil && msg.FinishPart().Reason == message.FinishReasonEndTurn {
			infoItem := chat.NewAssistantInfoItem(m.com.Styles, &msg, m.com.Config(), time.Unix(m.lastUserMessageTime, 0))
			m.chat.AppendMessages(infoItem)
			if m.chat.Follow() {
				if cmd := m.chat.ScrollToBottomAndAnimate(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
		if cmd := m.maybePromptSubscriptionReauth(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case message.Tool:
		for _, tr := range msg.ToolResults() {
			toolItem := m.chat.MessageItem(tr.ToolCallID)
			if toolItem == nil {
				// we should have an item!
				continue
			}
			if toolMsgItem, ok := toolItem.(chat.ToolMessageItem); ok {
				toolMsgItem.SetResult(&tr)
				if m.chat.Follow() {
					if cmd := m.chat.ScrollToBottomAndAnimate(); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			}
		}
	}
	return tea.Sequence(cmds...)
}

func (m *UI) handleClickFocus(msg tea.MouseClickMsg) (cmd tea.Cmd) {
	switch {
	case m.state != uiChat:
		return nil
	case image.Pt(msg.X, msg.Y).In(m.layout.sidebar):
		return nil
	case m.focus != uiFocusEditor && image.Pt(msg.X, msg.Y).In(m.layout.editor):
		m.focus = uiFocusEditor
		cmd = m.textarea.Focus()
		m.chat.Blur()
	case m.focus != uiFocusMain && image.Pt(msg.X, msg.Y).In(m.layout.main):
		m.focus = uiFocusMain
		m.textarea.Blur()
		m.chat.Focus()
	}
	return cmd
}

// updateSessionMessage updates an existing message in the current session in the chat
// when an assistant message is updated it may include updated tool calls as well
// that is why we need to handle creating/updating each tool call message too
func (m *UI) updateSessionMessage(msg message.Message) tea.Cmd {
	var cmds []tea.Cmd
	existingItem := m.chat.MessageItem(msg.ID)

	if existingItem != nil {
		if assistantItem, ok := existingItem.(*chat.AssistantMessageItem); ok {
			assistantItem.SetMessage(&msg)
		}
	}

	shouldRenderAssistant := chat.ShouldRenderAssistantMessage(&msg)
	// if the message of the assistant does not have any  response just tool calls we need to remove it
	if !shouldRenderAssistant && len(msg.ToolCalls()) > 0 && existingItem != nil {
		m.chat.RemoveMessage(msg.ID)
		if infoItem := m.chat.MessageItem(chat.AssistantInfoID(msg.ID)); infoItem != nil {
			m.chat.RemoveMessage(chat.AssistantInfoID(msg.ID))
		}
	}

	if shouldRenderAssistant && msg.FinishPart() != nil && msg.FinishPart().Reason == message.FinishReasonEndTurn {
		if infoItem := m.chat.MessageItem(chat.AssistantInfoID(msg.ID)); infoItem == nil {
			newInfoItem := chat.NewAssistantInfoItem(m.com.Styles, &msg, m.com.Config(), time.Unix(m.lastUserMessageTime, 0))
			m.chat.AppendMessages(newInfoItem)
		}
	}

	var items []chat.MessageItem
	for _, tc := range msg.ToolCalls() {
		existingToolItem := m.chat.MessageItem(tc.ID)
		if toolItem, ok := existingToolItem.(chat.ToolMessageItem); ok {
			existingToolCall := toolItem.ToolCall()
			// only update if finished state changed or input changed
			// to avoid clearing the cache
			if (tc.Finished && !existingToolCall.Finished) || tc.Input != existingToolCall.Input {
				toolItem.SetToolCall(tc)
			}
		}
		if existingToolItem == nil {
			items = append(items, chat.NewToolMessageItem(m.com.Styles, msg.ID, tc, nil, false))
		}
	}

	for _, item := range items {
		if animatable, ok := item.(chat.Animatable); ok {
			if cmd := animatable.StartAnimation(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	m.chat.AppendMessages(items...)

	if cmd := m.maybePromptSubscriptionReauth(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if m.chat.Follow() {
		if cmd := m.chat.ScrollToBottomAndAnimate(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.chat.SelectLast()
	}

	return tea.Sequence(cmds...)
}

func (m *UI) maybePromptSubscriptionReauth(msg message.Message) tea.Cmd {
	if msg.Role != message.Assistant || msg.ID == "" {
		return nil
	}
	if _, ok := m.reauthPromptedMessageIDs[msg.ID]; ok {
		return nil
	}
	finish := msg.FinishPart()
	if finish == nil || finish.Reason != message.FinishReasonError {
		return nil
	}

	errorText := strings.ToLower(strings.TrimSpace(finish.Message + " " + finish.Details))
	if !strings.Contains(errorText, "unauthorized") {
		return nil
	}

	cfg := m.com.Config()
	if cfg == nil {
		return nil
	}
	agentCfg, ok := cfg.Agents[config.AgentCoder]
	if !ok {
		return nil
	}
	selectedModel, ok := cfg.Models[agentCfg.Model]
	if !ok || !subscription.HasFetcher(selectedModel.Provider) {
		return nil
	}

	providerCfg, ok := cfg.Providers.Get(selectedModel.Provider)
	if !ok {
		return nil
	}
	provider := providerCfg.ToProvider()

	m.reauthPromptedMessageIDs[msg.ID] = struct{}{}
	return m.openAuthenticationDialog(provider, selectedModel, agentCfg.Model)
}

// handleChildSessionMessage handles messages from child sessions (agent tools).
func (m *UI) handleChildSessionMessage(event pubsub.Event[message.Message]) tea.Cmd {
	var cmds []tea.Cmd

	// Only process messages with tool calls or results.
	if len(event.Payload.ToolCalls()) == 0 && len(event.Payload.ToolResults()) == 0 {
		return nil
	}

	// Check if this is an agent tool session and parse it.
	childSessionID := event.Payload.SessionID
	_, toolCallID, ok := m.com.App.Sessions.ParseAgentToolSessionID(childSessionID)
	if !ok {
		return nil
	}

	// Find the parent agent tool item.
	var agentItem chat.NestedToolContainer
	for i := 0; i < m.chat.Len(); i++ {
		item := m.chat.MessageItem(toolCallID)
		if item == nil {
			continue
		}
		if agent, ok := item.(chat.NestedToolContainer); ok {
			if toolMessageItem, ok := item.(chat.ToolMessageItem); ok {
				if toolMessageItem.ToolCall().ID == toolCallID {
					// Verify this agent belongs to the correct parent message.
					// We can't directly check parentMessageID on the item, so we trust the session parsing.
					agentItem = agent
					break
				}
			}
		}
	}

	if agentItem == nil {
		return nil
	}

	// Get existing nested tools.
	nestedTools := agentItem.NestedTools()

	// Update or create nested tool calls.
	for _, tc := range event.Payload.ToolCalls() {
		found := false
		for _, existingTool := range nestedTools {
			if existingTool.ToolCall().ID == tc.ID {
				existingTool.SetToolCall(tc)
				found = true
				break
			}
		}
		if !found {
			// Create a new nested tool item.
			nestedItem := chat.NewToolMessageItem(m.com.Styles, event.Payload.ID, tc, nil, false)
			if simplifiable, ok := nestedItem.(chat.Compactable); ok {
				simplifiable.SetCompact(true)
			}
			if animatable, ok := nestedItem.(chat.Animatable); ok {
				if cmd := animatable.StartAnimation(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
			nestedTools = append(nestedTools, nestedItem)
		}
	}

	// Update nested tool results.
	for _, tr := range event.Payload.ToolResults() {
		for _, nestedTool := range nestedTools {
			if nestedTool.ToolCall().ID == tr.ToolCallID {
				nestedTool.SetResult(&tr)
				break
			}
		}
	}

	// Update the agent item with the new nested tools.
	agentItem.SetNestedTools(nestedTools)

	// Update the chat so it updates the index map for animations to work as expected
	m.chat.UpdateNestedToolIDs(toolCallID)

	if m.chat.Follow() {
		if cmd := m.chat.ScrollToBottomAndAnimate(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.chat.SelectLast()
	}

	return tea.Sequence(cmds...)
}

func (m *UI) handleDialogMsg(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	action := m.dialog.Update(msg)
	if action == nil {
		return tea.Batch(cmds...)
	}

	isOnboarding := m.state == uiOnboarding

	switch msg := action.(type) {
	// Generic dialog messages
	case dialog.ActionClose:
		if isOnboarding && m.dialog.ContainsDialog(dialog.ModelsID) {
			break
		}

		if m.dialog.ContainsDialog(dialog.FilePickerID) {
			defer fimage.ResetCache()
		}

		m.dialog.CloseFrontDialog()

		if isOnboarding {
			if cmd := m.openModelsDialog(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

		if m.focus == uiFocusEditor {
			cmds = append(cmds, m.textarea.Focus())
		}
	case dialog.ActionCmd:
		if msg.Cmd != nil {
			cmds = append(cmds, msg.Cmd)
		}

	// Session dialog messages.
	case dialog.ActionSelectSession:
		m.dialog.CloseDialog(dialog.SessionsID)
		cmds = append(cmds, m.loadSession(msg.Session.ID))

	// Open dialog message.
	case dialog.ActionOpenDialog:
		m.dialog.CloseDialog(dialog.CommandsID)
		if cmd := m.openDialog(msg.DialogID); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case dialog.ActionOpenConfirm:
		m.dialog.OpenDialog(dialog.NewConfirm(m.com, msg.Title, msg.Message, msg.Payload))
	case dialog.ActionConfirmChoice:
		m.dialog.CloseFrontDialog()
		if !msg.Confirmed {
			break
		}
		switch payload := msg.Payload.(type) {
		case dialog.ActionSkillsDeleteBatch:
			if cmd := m.dialog.StartLoading(); cmd != nil {
				cmds = append(cmds, cmd)
			}
			cmds = append(cmds, tea.Sequence(m.deleteSkillsCmd(payload.Names), m.skillsInstalledLoadCmd()))
		}

	// Command dialog messages.
	case dialog.ActionToggleYoloMode:
		cmds = append(cmds, util.CmdHandler(util.NewInfoMsg(m.toggleYoloMode())))
		m.dialog.CloseDialog(dialog.CommandsID)
	case dialog.ActionTogglePlanMode:
		cmds = append(cmds, util.CmdHandler(util.NewInfoMsg(m.togglePlanMode())))
		m.dialog.CloseDialog(dialog.CommandsID)
	case dialog.ActionSkillsList:
		if cmd := m.openSkillsSHSearchDialog(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.dialog.CloseDialog(dialog.CommandsID)
	case dialog.ActionSkillsSHSources:
		if m.dialog.ContainsDialog(dialog.CommandsID) {
			cmds = append(cmds, m.skillsSHSourcesCmd())
			m.dialog.CloseDialog(dialog.CommandsID)
		} else {
			cmds = append(cmds, m.skillsSHSourcesLoadCmd())
		}
	case dialog.ActionSkillsSHUpdate:
		cmds = append(cmds, m.skillsSHUpdateCmd(""), m.skillsSHSourcesLoadCmd(), m.skillsInstalledLoadCmd())
		m.dialog.CloseDialog(dialog.CommandsID)
	case dialog.ActionSkillsSHSearchRequest:
		if cmd := m.dialog.StartLoading(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.skillsSHSearchCmd(msg.Query, msg.RequestID))
	case dialog.ActionSkillsInstalledRefreshRequest:
		if cmd := m.dialog.StartLoading(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.skillsInstalledLoadCmd())
	case dialog.ActionSkillsSHSourcesRefreshRequest:
		if cmd := m.dialog.StartLoading(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.skillsSHSourcesLoadCmd())
	case dialog.ActionSkillsSHSearch:
		if len(msg.Arguments) > 0 && msg.Args == nil {
			m.dialog.CloseFrontDialog()
			argsDialog := dialog.NewArguments(
				m.com,
				"Search Skills.sh",
				"Search terms for skills.sh",
				msg.Arguments,
				msg,
			)
			m.dialog.OpenDialog(argsDialog)
			break
		}
		query := strings.TrimSpace(msg.Query)
		if msg.Args != nil {
			query = strings.TrimSpace(msg.Args["QUERY"])
		}
		if query == "" {
			cmds = append(cmds, util.ReportWarn("skills.sh query is required"))
			m.dialog.CloseFrontDialog()
			break
		}
		cmds = append(cmds, m.skillsSHSearchCmd(query, 0))
		m.dialog.CloseFrontDialog()
	case dialog.ActionSkillsSHInstall:
		if len(msg.Arguments) > 0 && msg.Args == nil {
			m.dialog.CloseFrontDialog()
			argsDialog := dialog.NewArguments(
				m.com,
				"Install Skills.sh Source",
				"Source format: skills.sh/<owner>/<repo>[/<skill>]",
				msg.Arguments,
				msg,
			)
			m.dialog.OpenDialog(argsDialog)
			break
		}
		source := strings.TrimSpace(msg.Source)
		if msg.Args != nil {
			source = strings.TrimSpace(msg.Args["SOURCE"])
		}
		if source == "" {
			cmds = append(cmds, util.ReportWarn("skills.sh source is required"))
			m.dialog.CloseFrontDialog()
			break
		}
		cmds = append(cmds, m.skillsSHInstallCmd(source))
		m.dialog.CloseFrontDialog()
	case dialog.ActionSkillsSHInstallSelected:
		if cmd := m.dialog.StartLoading(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.skillsSHInstallSelectedCmd(msg.Sources))
	case dialog.ActionSkillsSHInstallSource:
		if cmd := m.dialog.StartLoading(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.skillsSHInstallSourceCmd(msg.Source))
	case dialog.ActionSkillsSetDisabled:
		cmds = append(cmds, m.setSkillDisabledCmd(msg.Name, msg.Disabled), m.skillsInstalledLoadCmd())
	case dialog.ActionSkillsDelete:
		if cmd := m.dialog.StartLoading(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, tea.Sequence(m.deleteSkillsCmd([]string{msg.Name}), m.skillsInstalledLoadCmd()))
	case dialog.ActionSkillsSetDisabledBatch:
		cmds = append(cmds, m.setSkillsDisabledCmd(msg.Names, msg.Disabled), m.skillsInstalledLoadCmd())
	case dialog.ActionSkillsDeleteBatch:
		if cmd := m.dialog.StartLoading(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, tea.Sequence(m.deleteSkillsCmd(msg.Names), m.skillsInstalledLoadCmd()))
	case dialog.ActionSkillsFixPerms:
		cmds = append(cmds, m.fixSkillPermissionsCmd(msg.Names), m.skillsInstalledLoadCmd())
	case dialog.ActionSkillsSHOpenDetails:
		cmds = append(cmds, m.openExternalURLCmd(msg.URL))
	case dialog.ActionToggleNotifications:
		cfg := m.com.Config()
		if cfg != nil && cfg.Options != nil {
			disabled := !cfg.Options.DisableNotifications
			cfg.Options.DisableNotifications = disabled
			if err := m.com.Store().SetConfigField(config.ScopeGlobal, "options.disable_notifications", disabled); err != nil {
				cmds = append(cmds, util.ReportError(err))
			} else {
				status := "enabled"
				if disabled {
					status = "disabled"
				}
				cmds = append(cmds, util.CmdHandler(util.NewInfoMsg("Notifications "+status)))
			}
		}
		m.dialog.CloseDialog(dialog.CommandsID)
	case dialog.ActionNewSession:
		if m.isAgentBusy() {
			cmds = append(cmds, util.ReportWarn("Agent is busy, please wait before starting a new session..."))
			break
		}
		if cmd := m.newSession(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.dialog.CloseDialog(dialog.CommandsID)
	case dialog.ActionSummarize:
		if m.isAgentBusy() {
			cmds = append(cmds, util.ReportWarn("Agent is busy, please wait before summarizing session..."))
			break
		}
		cmds = append(cmds, func() tea.Msg {
			err := m.com.App.AgentCoordinator.Summarize(context.Background(), msg.SessionID)
			if err != nil {
				return util.ReportError(err)()
			}
			return nil
		})
		m.dialog.CloseDialog(dialog.CommandsID)
	case dialog.ActionToggleHelp:
		m.status.ToggleHelp()
		m.dialog.CloseDialog(dialog.CommandsID)
	case dialog.ActionExternalEditor:
		if m.isAgentBusy() {
			cmds = append(cmds, util.ReportWarn("Agent is working, please wait..."))
			break
		}
		cmds = append(cmds, m.openEditor(m.textarea.Value()))
		m.dialog.CloseDialog(dialog.CommandsID)
	case dialog.ActionToggleCompactMode:
		cmds = append(cmds, m.toggleCompactMode())
		m.dialog.CloseDialog(dialog.CommandsID)
	case dialog.ActionTogglePills:
		if cmd := m.togglePillsExpanded(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.dialog.CloseDialog(dialog.CommandsID)
	case dialog.ActionToggleThinking:
		cmds = append(cmds, func() tea.Msg {
			cfg := m.com.Config()
			if cfg == nil {
				return util.ReportError(errors.New("configuration not found"))()
			}

			agentCfg, ok := cfg.Agents[config.AgentCoder]
			if !ok {
				return util.ReportError(errors.New("agent configuration not found"))()
			}

			currentModel := cfg.Models[agentCfg.Model]
			currentModel.Think = !currentModel.Think
			if err := m.com.Store().UpdatePreferredModel(config.ScopeGlobal, agentCfg.Model, currentModel); err != nil {
				return util.ReportError(err)()
			}
			m.com.App.UpdateAgentModel(context.TODO())
			status := "disabled"
			if currentModel.Think {
				status = "enabled"
			}
			return util.NewInfoMsg("Thinking mode " + status)
		})
		m.dialog.CloseDialog(dialog.CommandsID)
	case dialog.ActionToggleTransparentBackground:
		cmds = append(cmds, func() tea.Msg {
			cfg := m.com.Config()
			if cfg == nil {
				return util.ReportError(errors.New("configuration not found"))()
			}

			isTransparent := cfg.Options != nil && cfg.Options.TUI.Transparent != nil && *cfg.Options.TUI.Transparent
			newValue := !isTransparent
			if err := m.com.Store().SetTransparentBackground(config.ScopeGlobal, newValue); err != nil {
				return util.ReportError(err)()
			}
			m.isTransparent = newValue

			status := "disabled"
			if newValue {
				status = "enabled"
			}
			return util.NewInfoMsg("Transparent background " + status)
		})
		m.dialog.CloseDialog(dialog.CommandsID)
	case dialog.ActionQuit:
		cmds = append(cmds, tea.Quit)
	case dialog.ActionEnableDockerMCP:
		m.dialog.CloseDialog(dialog.CommandsID)
		cmds = append(cmds, m.enableDockerMCP)
	case dialog.ActionDisableDockerMCP:
		m.dialog.CloseDialog(dialog.CommandsID)
		cmds = append(cmds, m.disableDockerMCP)
	case dialog.ActionInitializeProject:
		if m.isAgentBusy() {
			cmds = append(cmds, util.ReportWarn("Agent is busy, please wait before summarizing session..."))
			break
		}
		cmds = append(cmds, m.initializeProject())
		m.dialog.CloseDialog(dialog.CommandsID)

	case dialog.ActionSelectModel:
		if m.isAgentBusy() {
			cmds = append(cmds, util.ReportWarn("Agent is busy, please wait..."))
			break
		}

		cfg := m.com.Config()
		if cfg == nil {
			cmds = append(cmds, util.ReportError(errors.New("configuration not found")))
			break
		}

		var (
			providerID   = msg.Model.Provider
			isCopilot    = providerID == string(catwalk.InferenceProviderCopilot)
			isConfigured = func() bool { _, ok := cfg.Providers.Get(providerID); return ok }
		)

		// Attempt to import GitHub Copilot tokens from VSCode if available.
		if isCopilot && !isConfigured() && !msg.ReAuthenticate {
			m.com.Store().ImportCopilot()
		}

		if !isConfigured() || msg.ReAuthenticate {
			m.dialog.CloseDialog(dialog.ModelsID)
			if cmd := m.openAuthenticationDialog(msg.Provider, msg.Model, msg.ModelType); cmd != nil {
				cmds = append(cmds, cmd)
			}
			break
		}

		if err := m.com.Store().UpdatePreferredModel(config.ScopeGlobal, msg.ModelType, msg.Model); err != nil {
			cmds = append(cmds, util.ReportError(err))
		} else if _, ok := cfg.Models[config.SelectedModelTypeSmall]; !ok {
			// Ensure small model is set is unset.
			smallModel := m.com.App.GetDefaultSmallModel(providerID)
			if err := m.com.Store().UpdatePreferredModel(config.ScopeGlobal, config.SelectedModelTypeSmall, smallModel); err != nil {
				cmds = append(cmds, util.ReportError(err))
			}
		}

		cmds = append(cmds, func() tea.Msg {
			if err := m.com.App.UpdateAgentModel(context.TODO()); err != nil {
				return util.ReportError(err)
			}

			modelMsg := fmt.Sprintf("%s model changed to %s", msg.ModelType, msg.Model.Model)

			return util.NewInfoMsg(modelMsg)
		})
		if cmd := m.refreshSubscriptionUsage(true); cmd != nil {
			cmds = append(cmds, cmd)
		}

		m.dialog.CloseDialog(dialog.APIKeyInputID)
		m.dialog.CloseDialog(dialog.OAuthID)
		m.dialog.CloseDialog(dialog.ModelsID)

		if isOnboarding {
			m.setState(uiLanding, uiFocusEditor)
			m.com.Config().SetupAgents()
			if err := m.com.App.InitCoderAgent(context.TODO()); err != nil {
				cmds = append(cmds, util.ReportError(err))
			}
		}
	case dialog.ActionSelectReasoningEffort:
		if m.isAgentBusy() {
			cmds = append(cmds, util.ReportWarn("Agent is busy, please wait..."))
			break
		}

		cfg := m.com.Config()
		if cfg == nil {
			cmds = append(cmds, util.ReportError(errors.New("configuration not found")))
			break
		}

		agentCfg, ok := cfg.Agents[config.AgentCoder]
		if !ok {
			cmds = append(cmds, util.ReportError(errors.New("agent configuration not found")))
			break
		}

		currentModel := cfg.Models[agentCfg.Model]
		currentModel.ReasoningEffort = msg.Effort
		if err := m.com.Store().UpdatePreferredModel(config.ScopeGlobal, agentCfg.Model, currentModel); err != nil {
			cmds = append(cmds, util.ReportError(err))
			break
		}

		cmds = append(cmds, func() tea.Msg {
			m.com.App.UpdateAgentModel(context.TODO())
			return util.NewInfoMsg("Reasoning effort set to " + msg.Effort)
		})
		m.dialog.CloseDialog(dialog.ReasoningID)
	case dialog.ActionPermissionResponse:
		m.dialog.CloseDialog(dialog.PermissionsID)
		switch msg.Action {
		case dialog.PermissionAllow:
			m.com.App.Permissions.Grant(msg.Permission)
		case dialog.PermissionAllowForSession:
			m.com.App.Permissions.GrantPersistent(msg.Permission)
		case dialog.PermissionDiscuss:
			m.com.App.Permissions.Discuss(msg.Permission)
			discussPrompt := fmt.Sprintf(
				"Lass uns genau diese vorgeschlagene Änderung diskutieren und dann fortsetzen.\n\nPermission-Request:\n- Tool: %s\n- Action: %s\n- Path: %s\n- Beschreibung: %s\n\nBitte beziehe dich auf diese Änderung, schlage kurz einen sicheren Weg vor und setze danach die Umsetzung fort.",
				msg.Permission.ToolName,
				msg.Permission.Action,
				msg.Permission.Path,
				msg.Permission.Description,
			)
			cmds = append(cmds, m.sendMessage(discussPrompt))
		case dialog.PermissionDeny:
			m.com.App.Permissions.Deny(msg.Permission)
		}

	case dialog.ActionFilePickerSelected:
		cmds = append(cmds, tea.Sequence(
			msg.Cmd(),
			func() tea.Msg {
				m.dialog.CloseDialog(dialog.FilePickerID)
				return nil
			},
			func() tea.Msg {
				fimage.ResetCache()
				return nil
			},
		))

	case dialog.ActionRunCustomCommand:
		if len(msg.Arguments) > 0 && msg.Args == nil {
			m.dialog.CloseFrontDialog()
			argsDialog := dialog.NewArguments(
				m.com,
				"Custom Command Arguments",
				"",
				msg.Arguments,
				msg, // Pass the action as the result
			)
			m.dialog.OpenDialog(argsDialog)
			break
		}
		content := msg.Content
		if msg.Args != nil {
			content = substituteArgs(content, msg.Args)
		}
		cmds = append(cmds, m.sendMessage(content))
		m.dialog.CloseFrontDialog()
	case dialog.ActionRunMCPPrompt:
		if len(msg.Arguments) > 0 && msg.Args == nil {
			m.dialog.CloseFrontDialog()
			title := cmp.Or(msg.Title, "MCP Prompt Arguments")
			argsDialog := dialog.NewArguments(
				m.com,
				title,
				msg.Description,
				msg.Arguments,
				msg, // Pass the action as the result
			)
			m.dialog.OpenDialog(argsDialog)
			break
		}
		cmds = append(cmds, m.runMCPPrompt(msg.ClientID, msg.PromptID, msg.Args))
	default:
		cmds = append(cmds, util.CmdHandler(msg))
	}

	return tea.Batch(cmds...)
}

// substituteArgs replaces $ARG_NAME placeholders in content with actual values.
func substituteArgs(content string, args map[string]string) string {
	for name, value := range args {
		placeholder := "$" + name
		content = strings.ReplaceAll(content, placeholder, value)
	}
	return content
}

func (m *UI) openAuthenticationDialog(provider catwalk.Provider, model config.SelectedModel, modelType config.SelectedModelType) tea.Cmd {
	var (
		dlg dialog.Dialog
		cmd tea.Cmd

		isOnboarding = m.state == uiOnboarding
	)

	switch provider.ID {
	case "hyper":
		dlg, cmd = dialog.NewOAuthHyper(m.com, isOnboarding, provider, model, modelType)
	case catwalk.InferenceProviderCopilot:
		dlg, cmd = dialog.NewOAuthCopilot(m.com, isOnboarding, provider, model, modelType)
	case "openai-codex":
		dlg, cmd = dialog.NewOAuthOpenAICodex(m.com, isOnboarding, provider, model, modelType)
	default:
		dlg, cmd = dialog.NewAPIKeyInput(m.com, isOnboarding, provider, model, modelType)
	}

	if m.dialog.ContainsDialog(dlg.ID()) {
		m.dialog.BringToFront(dlg.ID())
		return nil
	}

	m.dialog.OpenDialog(dlg)
	return cmd
}

func (m *UI) handleKeyPressMsg(msg tea.KeyPressMsg) tea.Cmd {
	var cmds []tea.Cmd

	handleGlobalKeys := func(msg tea.KeyPressMsg) bool {
		switch {
		case key.Matches(msg, m.keyMap.Help):
			m.status.ToggleHelp()
			m.updateLayoutAndSize()
			return true
		case key.Matches(msg, m.keyMap.Commands):
			if cmd := m.openCommandsDialog(); cmd != nil {
				cmds = append(cmds, cmd)
			}
			return true
		case key.Matches(msg, m.keyMap.Models):
			if cmd := m.openModelsDialog(); cmd != nil {
				cmds = append(cmds, cmd)
			}
			return true
		case key.Matches(msg, m.keyMap.Sidebar):
			if m.state == uiChat {
				cmds = append(cmds, m.toggleCompactMode())
				return true
			}
		case key.Matches(msg, m.keyMap.Yolo):
			if m.shouldCyclePermissionMode() {
				cmds = append(cmds, util.ReportInfo(m.cyclePermissionMode()))
				return true
			}
		case key.Matches(msg, m.keyMap.YoloMode):
			if m.shouldCyclePermissionMode() {
				cmds = append(cmds, util.ReportInfo(m.toggleYoloMode()))
				return true
			}
		case key.Matches(msg, m.keyMap.PlanMode):
			if m.shouldCyclePermissionMode() {
				cmds = append(cmds, util.ReportInfo(m.togglePlanMode()))
				return true
			}
		case key.Matches(msg, m.keyMap.Sessions):
			if cmd := m.openSessionsDialog(); cmd != nil {
				cmds = append(cmds, cmd)
			}
			return true
		case key.Matches(msg, m.keyMap.Chat.Details) && m.isCompact:
			m.detailsOpen = !m.detailsOpen
			m.updateLayoutAndSize()
			return true
		case key.Matches(msg, m.keyMap.Chat.TogglePills):
			if m.state == uiChat && m.hasSession() {
				if cmd := m.togglePillsExpanded(); cmd != nil {
					cmds = append(cmds, cmd)
				}
				return true
			}
		case key.Matches(msg, m.keyMap.Chat.PillLeft):
			if m.state == uiChat && m.hasSession() && m.pillsExpanded && m.focus != uiFocusEditor {
				if cmd := m.switchPillSection(-1); cmd != nil {
					cmds = append(cmds, cmd)
				}
				return true
			}
		case key.Matches(msg, m.keyMap.Chat.PillRight):
			if m.state == uiChat && m.hasSession() && m.pillsExpanded && m.focus != uiFocusEditor {
				if cmd := m.switchPillSection(1); cmd != nil {
					cmds = append(cmds, cmd)
				}
				return true
			}
		case key.Matches(msg, m.keyMap.Suspend):
			if m.isAgentBusy() {
				cmds = append(cmds, util.ReportWarn("Agent is busy, please wait..."))
				return true
			}
			cmds = append(cmds, tea.Suspend)
			return true
		}
		return false
	}

	if key.Matches(msg, m.keyMap.Quit) && !m.dialog.ContainsDialog(dialog.QuitID) {
		// Always handle quit keys first
		if cmd := m.openQuitDialog(); cmd != nil {
			cmds = append(cmds, cmd)
		}

		return tea.Batch(cmds...)
	}

	// Route all messages to dialog if one is open.
	if m.dialog.HasDialogs() {
		return m.handleDialogMsg(msg)
	}

	// Handle cancel key when agent is busy.
	if key.Matches(msg, m.keyMap.Chat.Cancel) {
		if m.isAgentBusy() {
			if cmd := m.cancelAgent(); cmd != nil {
				cmds = append(cmds, cmd)
			}
			return tea.Batch(cmds...)
		}
	}

	switch m.state {
	case uiOnboarding:
		return tea.Batch(cmds...)
	case uiInitialize:
		cmds = append(cmds, m.updateInitializeView(msg)...)
		return tea.Batch(cmds...)
	case uiChat, uiLanding:
		switch m.focus {
		case uiFocusEditor:
			// Handle completions if open.
			if m.completionsOpen {
				if msg, ok := m.completions.Update(msg); ok {
					switch msg := msg.(type) {
					case completions.SelectionMsg[completions.FileCompletionValue]:
						cmds = append(cmds, m.insertFileCompletion(msg.Value.Path))
						if !msg.KeepOpen {
							m.closeCompletions()
						}
					case completions.SelectionMsg[completions.ResourceCompletionValue]:
						cmds = append(cmds, m.insertMCPResourceCompletion(msg.Value))
						if !msg.KeepOpen {
							m.closeCompletions()
						}
					case completions.ClosedMsg:
						m.completionsOpen = false
					}
					return tea.Batch(cmds...)
				}
			}

			if ok := m.attachments.Update(msg); ok {
				return tea.Batch(cmds...)
			}

			switch {
			case key.Matches(msg, m.keyMap.Editor.AddImage):
				if cmd := m.openFilesDialog(); cmd != nil {
					cmds = append(cmds, cmd)
				}

			case key.Matches(msg, m.keyMap.Editor.PasteImage):
				cmds = append(cmds, m.pasteAttachmentFromClipboard)

			case key.Matches(msg, m.keyMap.Editor.SendMessage):
				prevHeight := m.textarea.Height()
				value := m.textarea.Value()
				if before, ok := strings.CutSuffix(value, "\\"); ok {
					// If the last character is a backslash, remove it and add a newline.
					m.textarea.SetValue(before)
					if cmd := m.handleTextareaHeightChange(prevHeight); cmd != nil {
						cmds = append(cmds, cmd)
					}
					break
				}

				// Otherwise, send the message
				m.textarea.Reset()
				if cmd := m.handleTextareaHeightChange(prevHeight); cmd != nil {
					cmds = append(cmds, cmd)
				}

				value = strings.TrimSpace(value)
				if value == "exit" || value == "quit" {
					return m.openQuitDialog()
				}

				attachments := m.attachments.List()
				m.attachments.Reset()
				if value == localStatusCommand {
					m.randomizePlaceholders()
					m.historyReset()
					return tea.Batch(m.showLocalStatus(), m.loadPromptHistory())
				}
				if strings.HasPrefix(value, localPlanCommand) {
					m.randomizePlaceholders()
					m.historyReset()
					cmd, handled := m.handleLocalPlanCommand(value)
					if handled {
						return tea.Batch(cmd, m.loadPromptHistory())
					}
				}
				if strings.HasPrefix(strings.ToLower(value), localBTWCommand) {
					m.randomizePlaceholders()
					m.historyReset()
					cmd, handled, inlineMessage := m.handleLocalBTWCommand(value)
					if handled {
						if inlineMessage != "" {
							if cmd != nil {
								return tea.Batch(cmd, m.sendMessage(inlineMessage, attachments...), m.loadPromptHistory())
							}
							return tea.Batch(m.sendMessage(inlineMessage, attachments...), m.loadPromptHistory())
						}
						if cmd != nil {
							return tea.Batch(cmd, m.loadPromptHistory())
						}
						return m.loadPromptHistory()
					}
				}
				if len(value) == 0 && !message.ContainsTextAttachment(attachments) {
					return nil
				}

				value = m.prepareOutgoingMessage(value)
				m.randomizePlaceholders()
				m.historyReset()

				return tea.Batch(m.sendMessage(value, attachments...), m.loadPromptHistory())
			case key.Matches(msg, m.keyMap.Chat.NewSession):
				if !m.hasSession() {
					break
				}
				if m.isAgentBusy() {
					cmds = append(cmds, util.ReportWarn("Agent is busy, please wait before starting a new session..."))
					break
				}
				if cmd := m.newSession(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			case key.Matches(msg, m.keyMap.Tab):
				if m.state != uiLanding {
					m.setState(m.state, uiFocusMain)
					m.textarea.Blur()
					m.chat.Focus()
					m.chat.SetSelected(m.chat.Len() - 1)
				}
			case key.Matches(msg, m.keyMap.Editor.OpenEditor):
				if m.isAgentBusy() {
					cmds = append(cmds, util.ReportWarn("Agent is working, please wait..."))
					break
				}
				cmds = append(cmds, m.openEditor(m.textarea.Value()))
			case key.Matches(msg, m.keyMap.Editor.PrevWord):
				m.moveEditorCursorByWord(-1)
			case key.Matches(msg, m.keyMap.Editor.NextWord):
				m.moveEditorCursorByWord(1)
			case msg.String() == "ctrl+backspace":
				prevHeight := m.textarea.Height()
				m.textarea.SetValue("")
				m.closeCompletions()
				if cmd := m.handleTextareaHeightChange(prevHeight); cmd != nil {
					cmds = append(cmds, cmd)
				}
			case key.Matches(msg, m.keyMap.Editor.Newline):
				prevHeight := m.textarea.Height()
				m.textarea.InsertRune('\n')
				m.closeCompletions()
				cmds = append(cmds, m.updateTextareaWithPrevHeight(msg, prevHeight))
			case key.Matches(msg, m.keyMap.Editor.HistoryPrev):
				cmd := m.handleHistoryUp(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			case key.Matches(msg, m.keyMap.Editor.HistoryNext):
				cmd := m.handleHistoryDown(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			case key.Matches(msg, m.keyMap.Editor.Escape):
				cmd := m.handleHistoryEscape(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			default:
				if handleGlobalKeys(msg) {
					// Handle global keys first before passing to textarea.
					break
				}

				// Check for @ trigger before passing to textarea.
				curValue := m.textarea.Value()
				curIdx := len(curValue)

				// Trigger completions on @.
				if msg.String() == "@" && !m.completionsOpen {
					// Only show if beginning of prompt or after whitespace.
					if curIdx == 0 || (curIdx > 0 && isWhitespace(curValue[curIdx-1])) {
						m.completionsOpen = true
						m.completionsQuery = ""
						m.completionsStartIndex = curIdx
						m.completionsPositionStart = m.completionsPosition()
						depth, limit := m.com.Config().Options.TUI.Completions.Limits()
						cmds = append(cmds, m.completions.Open(depth, limit))
					}
				}

				// remove the details if they are open when user starts typing
				if m.detailsOpen {
					m.detailsOpen = false
					m.updateLayoutAndSize()
				}

				prevHeight := m.textarea.Height()
				cmds = append(cmds, m.updateTextareaWithPrevHeight(msg, prevHeight))

				// Any text modification becomes the current draft.
				m.updateHistoryDraft(curValue)

				// After updating textarea, check if we need to filter completions.
				// Skip filtering on the initial @ keystroke since items are loading async.
				if m.completionsOpen && msg.String() != "@" {
					newValue := m.textarea.Value()
					newIdx := len(newValue)

					// Close completions if cursor moved before start.
					if newIdx <= m.completionsStartIndex {
						m.closeCompletions()
					} else if msg.String() == "space" {
						// Close on space.
						m.closeCompletions()
					} else {
						// Extract current word and filter.
						word := m.textareaWord()
						if strings.HasPrefix(word, "@") {
							m.completionsQuery = word[1:]
							m.completions.Filter(m.completionsQuery)
						} else if m.completionsOpen {
							m.closeCompletions()
						}
					}
				}
			}
		case uiFocusMain:
			switch {
			case key.Matches(msg, m.keyMap.Tab):
				m.focus = uiFocusEditor
				cmds = append(cmds, m.textarea.Focus())
				m.chat.Blur()
			case key.Matches(msg, m.keyMap.Chat.NewSession):
				if !m.hasSession() {
					break
				}
				if m.isAgentBusy() {
					cmds = append(cmds, util.ReportWarn("Agent is busy, please wait before starting a new session..."))
					break
				}
				m.focus = uiFocusEditor
				if cmd := m.newSession(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			case key.Matches(msg, m.keyMap.Chat.Expand):
				m.chat.ToggleExpandedSelectedItem()
			case key.Matches(msg, m.keyMap.Chat.Up):
				if cmd := m.chat.ScrollByAndAnimate(-1); cmd != nil {
					cmds = append(cmds, cmd)
				}
				if !m.chat.SelectedItemInView() {
					m.chat.SelectPrev()
					if cmd := m.chat.ScrollToSelectedAndAnimate(); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			case key.Matches(msg, m.keyMap.Chat.Down):
				if cmd := m.chat.ScrollByAndAnimate(1); cmd != nil {
					cmds = append(cmds, cmd)
				}
				if !m.chat.SelectedItemInView() {
					m.chat.SelectNext()
					if cmd := m.chat.ScrollToSelectedAndAnimate(); cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			case key.Matches(msg, m.keyMap.Chat.UpOneItem):
				m.chat.SelectPrev()
				if cmd := m.chat.ScrollToSelectedAndAnimate(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			case key.Matches(msg, m.keyMap.Chat.DownOneItem):
				m.chat.SelectNext()
				if cmd := m.chat.ScrollToSelectedAndAnimate(); cmd != nil {
					cmds = append(cmds, cmd)
				}
			case key.Matches(msg, m.keyMap.Chat.HalfPageUp):
				if cmd := m.chat.ScrollByAndAnimate(-m.chat.Height() / 2); cmd != nil {
					cmds = append(cmds, cmd)
				}
				m.chat.SelectFirstInView()
			case key.Matches(msg, m.keyMap.Chat.HalfPageDown):
				if cmd := m.chat.ScrollByAndAnimate(m.chat.Height() / 2); cmd != nil {
					cmds = append(cmds, cmd)
				}
				m.chat.SelectLastInView()
			case key.Matches(msg, m.keyMap.Chat.PageUp):
				if cmd := m.chat.ScrollByAndAnimate(-m.chat.Height()); cmd != nil {
					cmds = append(cmds, cmd)
				}
				m.chat.SelectFirstInView()
			case key.Matches(msg, m.keyMap.Chat.PageDown):
				if cmd := m.chat.ScrollByAndAnimate(m.chat.Height()); cmd != nil {
					cmds = append(cmds, cmd)
				}
				m.chat.SelectLastInView()
			case key.Matches(msg, m.keyMap.Chat.Home):
				if cmd := m.chat.ScrollToTopAndAnimate(); cmd != nil {
					cmds = append(cmds, cmd)
				}
				m.chat.SelectFirst()
			case key.Matches(msg, m.keyMap.Chat.End):
				if cmd := m.chat.ScrollToBottomAndAnimate(); cmd != nil {
					cmds = append(cmds, cmd)
				}
				m.chat.SelectLast()
			default:
				if ok, cmd := m.chat.HandleKeyMsg(msg); ok {
					cmds = append(cmds, cmd)
				} else {
					handleGlobalKeys(msg)
				}
			}
		default:
			handleGlobalKeys(msg)
		}
	default:
		handleGlobalKeys(msg)
	}

	return tea.Sequence(cmds...)
}

// drawHeader draws the header section of the UI.
func (m *UI) drawHeader(scr uv.Screen, area uv.Rectangle) {
	m.header.drawHeader(
		scr,
		area,
		m.session,
		m.isCompact,
		m.detailsOpen,
		area.Dx(),
	)
}

// Draw implements [uv.Drawable] and draws the UI model.
func (m *UI) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	layout := m.generateLayout(area.Dx(), area.Dy())

	if m.layout != layout {
		m.layout = layout
		m.updateSize()
	}

	// Clear the screen first
	screen.Clear(scr)

	switch m.state {
	case uiOnboarding:
		m.drawHeader(scr, layout.header)

		// NOTE: Onboarding flow will be rendered as dialogs below, but
		// positioned at the bottom left of the screen.

	case uiInitialize:
		m.drawHeader(scr, layout.header)

		main := uv.NewStyledString(m.initializeView())
		main.Draw(scr, layout.main)

	case uiLanding:
		m.drawHeader(scr, layout.header)
		main := uv.NewStyledString(m.landingView())
		main.Draw(scr, layout.main)

		editor := uv.NewStyledString(m.renderEditorView(scr.Bounds().Dx()))
		editor.Draw(scr, layout.editor)

	case uiChat:
		if m.isCompact {
			m.drawHeader(scr, layout.header)
		} else {
			m.drawSidebar(scr, layout.sidebar)
		}

		m.chat.Draw(scr, layout.main)
		if layout.pills.Dy() > 0 && m.pillsView != "" {
			uv.NewStyledString(m.pillsView).Draw(scr, layout.pills)
		}

		editorWidth := scr.Bounds().Dx()
		if !m.isCompact {
			editorWidth -= layout.sidebar.Dx()
		}
		editor := uv.NewStyledString(m.renderEditorView(editorWidth))
		editor.Draw(scr, layout.editor)

		// Draw details overlay in compact mode when open
		if m.isCompact && m.detailsOpen {
			m.drawSessionDetails(scr, layout.sessionDetails)
		}
	}

	isOnboarding := m.state == uiOnboarding

	// Add status and help layer
	m.status.SetHideHelp(isOnboarding)
	m.status.Draw(scr, layout.status)

	// Draw completions popup if open
	if !isOnboarding && m.completionsOpen && m.completions.HasItems() {
		w, h := m.completions.Size()
		x := m.completionsPositionStart.X
		y := m.completionsPositionStart.Y - h

		screenW := area.Dx()
		if x+w > screenW {
			x = screenW - w
		}
		x = max(0, x)
		y = max(0, y+1) // Offset for attachments row

		completionsView := uv.NewStyledString(m.completions.Render())
		completionsView.Draw(scr, image.Rectangle{
			Min: image.Pt(x, y),
			Max: image.Pt(x+w, y+h),
		})
	}

	// Debugging rendering (visually see when the tui rerenders)
	if os.Getenv("FRANZ_UI_DEBUG") == "true" {
		debugView := lipgloss.NewStyle().Background(lipgloss.ANSIColor(rand.Intn(256))).Width(4).Height(2)
		debug := uv.NewStyledString(debugView.String())
		debug.Draw(scr, image.Rectangle{
			Min: image.Pt(4, 1),
			Max: image.Pt(8, 3),
		})
	}

	// This needs to come last to overlay on top of everything. We always pass
	// the full screen bounds because the dialogs will position themselves
	// accordingly.
	if m.dialog.HasDialogs() {
		return m.dialog.Draw(scr, scr.Bounds())
	}

	switch m.focus {
	case uiFocusEditor:
		if m.layout.editor.Dy() <= 0 {
			// Don't show cursor if editor is not visible
			return nil
		}
		if m.detailsOpen && m.isCompact {
			// Don't show cursor if details overlay is open
			return nil
		}

		if m.textarea.Focused() {
			cur := m.textarea.Cursor()
			cur.X++                            // Adjust for app margins
			cur.Y += m.layout.editor.Min.Y + 1 // Offset for attachments row
			return cur
		}
	}
	return nil
}

// View renders the UI model's view.
func (m *UI) View() tea.View {
	var v tea.View
	v.AltScreen = true
	if !m.isTransparent {
		v.BackgroundColor = m.com.Styles.Background
	}
	v.MouseMode = tea.MouseModeCellMotion
	v.ReportFocus = m.caps.ReportFocusEvents
	v.WindowTitle = "franz " + home.Short(m.com.Store().WorkingDir())

	canvas := uv.NewScreenBuffer(m.width, m.height)
	v.Cursor = m.Draw(canvas, canvas.Bounds())

	content := strings.ReplaceAll(canvas.Render(), "\r\n", "\n") // normalize newlines
	contentLines := strings.Split(content, "\n")
	for i, line := range contentLines {
		// Trim trailing spaces for concise rendering
		contentLines[i] = strings.TrimRight(line, " ")
	}

	content = strings.Join(contentLines, "\n")

	v.Content = content
	if m.progressBarEnabled && m.sendProgressBar && m.isAgentBusy() {
		// HACK: use a random percentage to prevent ghostty from hiding it
		// after a timeout.
		v.ProgressBar = tea.NewProgressBar(tea.ProgressBarIndeterminate, rand.Intn(100))
	}

	return v
}

// ShortHelp implements [help.KeyMap].
func (m *UI) ShortHelp() []key.Binding {
	var binds []key.Binding
	k := &m.keyMap
	tab := k.Tab
	commands := k.Commands
	if m.focus == uiFocusEditor && m.textarea.Value() == "" {
		commands.SetHelp("ctrl+p", "commands")
	}

	switch m.state {
	case uiInitialize:
		binds = append(binds, k.Quit)
	case uiChat:
		// Show cancel binding if agent is busy.
		if m.isAgentBusy() {
			cancelBinding := k.Chat.Cancel
			if m.isCanceling {
				cancelBinding.SetHelp("esc", "press again to cancel")
			} else if m.com.App.AgentCoordinator.QueuedPrompts(m.session.ID) > 0 {
				cancelBinding.SetHelp("esc", "clear queue")
			}
			binds = append(binds, cancelBinding)
		}

		if m.focus == uiFocusEditor {
			tab.SetHelp("tab", "focus chat")
		} else {
			tab.SetHelp("tab", "focus editor")
		}

		binds = append(binds,
			tab,
			commands,
			k.Models,
		)

		switch m.focus {
		case uiFocusEditor:
		case uiFocusMain:
			binds = append(binds,
				k.Chat.UpDown,
				k.Chat.UpDownOneItem,
				k.Chat.PageUp,
				k.Chat.PageDown,
				k.Chat.Copy,
			)
			if m.pillsExpanded && hasIncompleteTodos(m.session.Todos) && m.promptQueue > 0 {
				binds = append(binds, k.Chat.PillLeft)
			}
		}
	default:
		// TODO: other states
		// if m.session == nil {
		// no session selected
		binds = append(binds,
			commands,
			k.Models,
		)
	}

	binds = append(binds,
		k.Quit,
		k.Help,
	)

	return binds
}

// FullHelp implements [help.KeyMap].
func (m *UI) FullHelp() [][]key.Binding {
	var binds [][]key.Binding
	k := &m.keyMap
	help := k.Help
	help.SetHelp("ctrl+g", "less")
	hasAttachments := len(m.attachments.List()) > 0
	hasSession := m.hasSession()
	commands := k.Commands
	if m.focus == uiFocusEditor && m.textarea.Value() == "" {
		commands.SetHelp("ctrl+p", "commands")
	}

	switch m.state {
	case uiInitialize:
		binds = append(binds,
			[]key.Binding{
				k.Quit,
			})
	case uiChat:
		// Show cancel binding if agent is busy.
		if m.isAgentBusy() {
			cancelBinding := k.Chat.Cancel
			if m.isCanceling {
				cancelBinding.SetHelp("esc", "press again to cancel")
			} else if m.com.App.AgentCoordinator.QueuedPrompts(m.session.ID) > 0 {
				cancelBinding.SetHelp("esc", "clear queue")
			}
			binds = append(binds, []key.Binding{cancelBinding})
		}

		mainBinds := []key.Binding{}
		tab := k.Tab
		if m.focus == uiFocusEditor {
			tab.SetHelp("tab", "focus chat")
		} else {
			tab.SetHelp("tab", "focus editor")
		}

		mainBinds = append(mainBinds,
			tab,
			commands,
			k.Models,
			k.Sidebar,
			k.Yolo,
			k.Sessions,
		)
		if hasSession {
			mainBinds = append(mainBinds, k.Chat.NewSession)
		}

		binds = append(binds, mainBinds)

		switch m.focus {
		case uiFocusEditor:
			editorBinds := []key.Binding{
				k.Editor.PrevWord,
				k.Editor.NextWord,
				k.Editor.MentionFile,
				k.Editor.OpenEditor,
			}
			editorBinds = append(editorBinds, k.Editor.AddImage)
			binds = append(binds, editorBinds)
			if hasAttachments {
				binds = append(binds,
					[]key.Binding{
						k.Editor.AttachmentDeleteMode,
						k.Editor.DeleteAllAttachments,
						k.Editor.Escape,
					},
				)
			}
		case uiFocusMain:
			binds = append(binds,
				[]key.Binding{
					k.Chat.UpDown,
					k.Chat.UpDownOneItem,
					k.Chat.PageUp,
					k.Chat.PageDown,
				},
				[]key.Binding{
					k.Chat.HalfPageUp,
					k.Chat.HalfPageDown,
					k.Chat.Home,
					k.Chat.End,
				},
				[]key.Binding{
					k.Chat.Copy,
					k.Chat.ClearHighlight,
				},
			)
			if m.pillsExpanded && hasIncompleteTodos(m.session.Todos) && m.promptQueue > 0 {
				binds = append(binds, []key.Binding{k.Chat.PillLeft})
			}
		}
	default:
		if m.session == nil {
			// no session selected
			binds = append(binds,
				[]key.Binding{
					commands,
					k.Models,
					k.Sessions,
				},
			)
			editorBinds := []key.Binding{
				k.Editor.PrevWord,
				k.Editor.NextWord,
				k.Editor.MentionFile,
				k.Editor.OpenEditor,
			}
			editorBinds = append(editorBinds, k.Editor.AddImage)
			binds = append(binds, editorBinds)
			if hasAttachments {
				binds = append(binds,
					[]key.Binding{
						k.Editor.AttachmentDeleteMode,
						k.Editor.DeleteAllAttachments,
						k.Editor.Escape,
					},
				)
			}
		}
	}

	binds = append(binds,
		[]key.Binding{
			help,
			k.Quit,
		},
	)

	return binds
}

func (m *UI) currentModelSupportsImages() bool {
	cfg := m.com.Config()
	if cfg == nil {
		return false
	}
	agentCfg, ok := cfg.Agents[config.AgentCoder]
	if !ok {
		return false
	}
	model := cfg.GetModelByType(agentCfg.Model)
	return model != nil && model.SupportsImages
}

func (m *UI) toggleYoloMode() string {
	yolo := !m.com.App.Permissions.SkipRequests()
	if yolo {
		// Keep modes exclusive: enabling yolo disables planning mode.
		m.planModeEnabled = false
	}
	m.com.App.Permissions.SetSkipRequests(yolo)
	m.setEditorPrompt(yolo, m.planModeEnabled)

	status := "off"
	if yolo {
		status = "on"
	}
	return "Yolo mode " + status
}

func (m *UI) togglePlanMode() string {
	m.planModeEnabled = !m.planModeEnabled
	if m.planModeEnabled {
		// Keep modes exclusive: enabling planning mode disables yolo mode.
		m.com.App.Permissions.SetSkipRequests(false)
	}
	m.setEditorPrompt(m.com.App.Permissions.SkipRequests(), m.planModeEnabled)
	if m.planModeEnabled {
		return "Planning mode on"
	}
	return "Planning mode off"
}

func (m *UI) shouldCyclePermissionMode() bool {
	if m.state != uiChat && m.state != uiLanding {
		return false
	}
	if m.focus != uiFocusEditor {
		return false
	}
	return m.dialog == nil || !m.dialog.HasDialogs()
}

func (m *UI) cyclePermissionMode() string {
	yolo := m.com.App.Permissions.SkipRequests()
	plan := m.planModeEnabled

	switch {
	case !yolo && !plan:
		// normal -> yolo
		m.com.App.Permissions.SetSkipRequests(true)
		m.planModeEnabled = false
		m.setEditorPrompt(true, false)
		return "Yolo mode on"
	case yolo && !plan:
		// yolo -> plan
		m.com.App.Permissions.SetSkipRequests(false)
		m.planModeEnabled = true
		m.setEditorPrompt(false, true)
		return "Planning mode on"
	default:
		// plan -> normal (and any fallback mixed state -> normal)
		m.com.App.Permissions.SetSkipRequests(false)
		m.planModeEnabled = false
		m.setEditorPrompt(false, false)
		return "Mode set to normal"
	}
}

// toggleCompactMode toggles compact mode between uiChat and uiChatCompact states.
func (m *UI) toggleCompactMode() tea.Cmd {
	if m.isCompact {
		m.forceCompactMode = false
		m.forceWideMode = true
	} else {
		m.forceCompactMode = true
		m.forceWideMode = false
	}

	err := m.com.Store().SetCompactMode(config.ScopeGlobal, m.forceCompactMode)
	if err != nil {
		return util.ReportError(err)
	}

	m.updateLayoutAndSize()

	return nil
}

// updateLayoutAndSize updates the layout and sizes of UI components.
func (m *UI) updateLayoutAndSize() {
	// Determine if we should be in compact mode
	if m.state == uiChat {
		if m.forceCompactMode {
			m.isCompact = true
		} else if m.forceWideMode {
			m.isCompact = false
		} else if m.width < compactModeWidthBreakpoint || m.height < compactModeHeightBreakpoint {
			m.isCompact = true
		} else {
			m.isCompact = false
		}
	}

	// First pass sizes components from the current textarea height.
	m.layout = m.generateLayout(m.width, m.height)
	prevHeight := m.textarea.Height()
	m.updateSize()

	// SetWidth can change textarea height due to soft-wrap recalculation.
	// If that happens, run one reconciliation pass with the new height.
	if m.textarea.Height() != prevHeight {
		m.layout = m.generateLayout(m.width, m.height)
		m.updateSize()
	}
}

// handleTextareaHeightChange checks whether the textarea height changed and,
// if so, recalculates the layout. When the chat is in follow mode it keeps
// the view scrolled to the bottom. The returned command, if non-nil, must be
// batched by the caller.
func (m *UI) handleTextareaHeightChange(prevHeight int) tea.Cmd {
	if m.textarea.Height() == prevHeight {
		return nil
	}
	m.updateLayoutAndSize()
	if m.state == uiChat && m.chat.Follow() {
		return m.chat.ScrollToBottomAndAnimate()
	}
	return nil
}

// updateTextarea updates the textarea for msg and then reconciles layout if
// the textarea height changed as a result.
func (m *UI) updateTextarea(msg tea.Msg) tea.Cmd {
	return m.updateTextareaWithPrevHeight(msg, m.textarea.Height())
}

// updateTextareaWithPrevHeight is for cases when the height of the layout may
// have changed.
//
// Particularly, it's for cases where the textarea changes before
// textarea.Update is called (for example, SetValue, Reset, and InsertRune). We
// pass the height from before those changes took place so we can compare
// "before" vs "after" sizing and recalculate the layout if the textarea grew
// or shrank.
func (m *UI) updateTextareaWithPrevHeight(msg tea.Msg, prevHeight int) tea.Cmd {
	ta, cmd := m.textarea.Update(msg)
	m.textarea = ta
	return tea.Batch(cmd, m.handleTextareaHeightChange(prevHeight))
}

// updateSize updates the sizes of UI components based on the current layout.
func (m *UI) updateSize() {
	// Set status width
	m.status.SetWidth(m.layout.status.Dx())

	m.chat.SetSize(m.layout.main.Dx(), m.layout.main.Dy())
	m.textarea.MaxHeight = TextareaMaxHeight
	m.textarea.SetWidth(m.layout.editor.Dx())
	m.renderPills()

	// Handle different app states
	switch m.state {
	case uiChat:
		if !m.isCompact {
			m.cacheSidebarLogo(m.layout.sidebar.Dx())
		}
	}
}

// generateLayout calculates the layout rectangles for all UI components based
// on the current UI state and terminal dimensions.
func (m *UI) generateLayout(w, h int) uiLayout {
	// The screen area we're working with
	area := image.Rect(0, 0, w, h)

	// The help height
	helpHeight := 1
	// The editor height: textarea height + margin for attachments and bottom spacing.
	editorHeight := m.textarea.Height() + editorHeightMargin
	// The sidebar width
	sidebarWidth := 30
	// The header height
	const landingHeaderHeight = 7

	var helpKeyMap help.KeyMap = m
	if m.status != nil && m.status.ShowingAll() {
		for _, row := range helpKeyMap.FullHelp() {
			helpHeight = max(helpHeight, len(row))
		}
	}

	// Add app margins
	appParts := layout.Vertical(layout.Len(area.Dy()-helpHeight), layout.Fill(1)).Split(area)
	appRect, helpRect := appParts[0], appParts[1]
	appRect.Min.Y += 1
	appRect.Max.Y -= 1
	helpRect.Min.Y -= 1
	appRect.Min.X += 1
	appRect.Max.X -= 1

	if slices.Contains([]uiState{uiOnboarding, uiInitialize, uiLanding}, m.state) {
		// extra padding on left and right for these states
		appRect.Min.X += 1
		appRect.Max.X -= 1
	}

	uiLayout := uiLayout{
		area:   area,
		status: helpRect,
	}

	// Handle different app states
	switch m.state {
	case uiOnboarding, uiInitialize:
		// Layout
		//
		// header
		// ------
		// main
		// ------
		// help

		headerParts := layout.Vertical(layout.Len(landingHeaderHeight), layout.Fill(1)).Split(appRect)
		headerRect, mainRect := headerParts[0], headerParts[1]
		uiLayout.header = headerRect
		uiLayout.main = mainRect

	case uiLanding:
		// Layout
		//
		// header
		// ------
		// main
		// ------
		// editor
		// ------
		// help
		headerParts := layout.Vertical(layout.Len(landingHeaderHeight), layout.Fill(1)).Split(appRect)
		headerRect, mainRect := headerParts[0], headerParts[1]
		mainEditorParts := layout.Vertical(layout.Len(mainRect.Dy()-editorHeight), layout.Fill(1)).Split(mainRect)
		mainRect, editorRect := mainEditorParts[0], mainEditorParts[1]
		// Remove extra padding from editor (but keep it for header and main)
		editorRect.Min.X -= 1
		editorRect.Max.X += 1
		uiLayout.header = headerRect
		uiLayout.main = mainRect
		uiLayout.editor = editorRect

	case uiChat:
		if m.isCompact {
			// Layout
			//
			// compact-header
			// ------
			// main
			// ------
			// editor
			// ------
			// help
			const compactHeaderHeight = 4
			compactHeaderParts := layout.Vertical(layout.Len(compactHeaderHeight), layout.Fill(1)).Split(appRect)
			headerRect, mainRect := compactHeaderParts[0], compactHeaderParts[1]
			detailsHeight := min(sessionDetailsMaxHeight, area.Dy()-1) // One row for the header
			detailsParts := layout.Vertical(layout.Len(detailsHeight), layout.Fill(1)).Split(appRect)
			sessionDetailsArea, _ := detailsParts[0], detailsParts[1]
			uiLayout.sessionDetails = sessionDetailsArea
			uiLayout.sessionDetails.Min.Y += compactHeaderHeight // adjust for header
			// Add one line gap between header and main content
			mainRect.Min.Y += 1
			mainEditorParts := layout.Vertical(layout.Len(mainRect.Dy()-editorHeight), layout.Fill(1)).Split(mainRect)
			mainRect, editorRect := mainEditorParts[0], mainEditorParts[1]
			mainRect.Max.X -= 1 // Add padding right
			uiLayout.header = headerRect
			pillsHeight := m.pillsAreaHeight()
			if pillsHeight > 0 {
				pillsHeight = min(pillsHeight, mainRect.Dy())
				compactPillsParts := layout.Vertical(layout.Len(mainRect.Dy()-pillsHeight), layout.Fill(1)).Split(mainRect)
				chatRect, pillsRect := compactPillsParts[0], compactPillsParts[1]
				uiLayout.main = chatRect
				uiLayout.pills = pillsRect
			} else {
				uiLayout.main = mainRect
			}
			// Add bottom margin to main
			uiLayout.main.Max.Y -= 1
			uiLayout.editor = editorRect
		} else {
			// Layout
			//
			// ------|---
			// main  |
			// ------| side
			// editor|
			// ----------
			// help

			mainSideParts := layout.Horizontal(layout.Len(appRect.Dx()-sidebarWidth), layout.Fill(1)).Split(appRect)
			mainRect, sideRect := mainSideParts[0], mainSideParts[1]
			// Add padding left
			sideRect.Min.X += 1
			mainEditorParts := layout.Vertical(layout.Len(mainRect.Dy()-editorHeight), layout.Fill(1)).Split(mainRect)
			mainRect, editorRect := mainEditorParts[0], mainEditorParts[1]
			mainRect.Max.X -= 1 // Add padding right
			uiLayout.sidebar = sideRect
			pillsHeight := m.pillsAreaHeight()
			if pillsHeight > 0 {
				pillsHeight = min(pillsHeight, mainRect.Dy())
				compactPillsParts := layout.Vertical(layout.Len(mainRect.Dy()-pillsHeight), layout.Fill(1)).Split(mainRect)
				chatRect, pillsRect := compactPillsParts[0], compactPillsParts[1]
				uiLayout.main = chatRect
				uiLayout.pills = pillsRect
			} else {
				uiLayout.main = mainRect
			}
			// Add bottom margin to main
			uiLayout.main.Max.Y -= 1
			uiLayout.editor = editorRect
		}
	}

	return uiLayout
}

// uiLayout defines the positioning of UI elements.
type uiLayout struct {
	// area is the overall available area.
	area uv.Rectangle

	// header is the header shown in special cases
	// e.x when the sidebar is collapsed
	// or when in the landing page
	// or in init/config
	header uv.Rectangle

	// main is the area for the main pane. (e.x chat, configure, landing)
	main uv.Rectangle

	// pills is the area for the pills panel.
	pills uv.Rectangle

	// editor is the area for the editor pane.
	editor uv.Rectangle

	// sidebar is the area for the sidebar.
	sidebar uv.Rectangle

	// status is the area for the status view.
	status uv.Rectangle

	// session details is the area for the session details overlay in compact mode.
	sessionDetails uv.Rectangle
}

func (m *UI) openEditor(value string) tea.Cmd {
	tmpfile, err := os.CreateTemp("", "msg_*.md")
	if err != nil {
		return util.ReportError(err)
	}
	tmpPath := tmpfile.Name()
	defer tmpfile.Close() //nolint:errcheck
	if _, err := tmpfile.WriteString(value); err != nil {
		return util.ReportError(err)
	}
	restoreEditorEnv := m.applyConfiguredEditorEnv()
	defer restoreEditorEnv()
	cmd, err := editor.Command(
		"franz",
		tmpPath,
		editor.AtPosition(
			m.textarea.Line()+1,
			m.textarea.Column()+1,
		),
	)
	if err != nil {
		return util.ReportError(err)
	}
	cmd.Env = os.Environ()
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		defer func() {
			_ = os.Remove(tmpPath)
		}()

		if err != nil {
			return util.ReportError(err)
		}
		content, err := os.ReadFile(tmpPath)
		if err != nil {
			return util.ReportError(err)
		}
		if len(content) == 0 {
			return util.ReportWarn("Message is empty")
		}
		return openEditorMsg{
			Text: strings.TrimSpace(string(content)),
		}
	})
}

func (m *UI) applyConfiguredEditorEnv() func() {
	cfg := m.com.Config()
	if cfg == nil || cfg.Options == nil {
		return func() {}
	}
	editorCmd := strings.TrimSpace(cfg.Options.Editor)
	if editorCmd == "" {
		return func() {}
	}

	previous, hadPrevious := os.LookupEnv("EDITOR")
	_ = os.Setenv("EDITOR", editorCmd)
	return func() {
		if hadPrevious {
			_ = os.Setenv("EDITOR", previous)
			return
		}
		_ = os.Unsetenv("EDITOR")
	}
}

// setEditorPrompt configures the textarea prompt function based on the current
// mode state.
func (m *UI) setEditorPrompt(yolo bool, plan bool) {
	switch {
	case yolo:
		m.textarea.SetPromptFunc(4, m.yoloPromptFunc)
	case plan:
		m.textarea.SetPromptFunc(4, m.planPromptFunc)
	default:
		m.textarea.SetPromptFunc(4, m.normalPromptFunc)
	}
}

// normalPromptFunc returns the normal editor prompt style ("  > " on first
// line, "::: " on subsequent lines).
func (m *UI) normalPromptFunc(info textarea.PromptInfo) string {
	t := m.com.Styles
	if info.LineNumber == 0 {
		if info.Focused {
			return "  > "
		}
		return "::: "
	}
	if info.Focused {
		return t.EditorPromptNormalFocused.Render()
	}
	return t.EditorPromptNormalBlurred.Render()
}

// yoloPromptFunc returns the yolo mode editor prompt style with warning icon
// and colored dots.
func (m *UI) yoloPromptFunc(info textarea.PromptInfo) string {
	t := m.com.Styles
	if info.LineNumber == 0 {
		if info.Focused {
			return t.EditorPromptYoloIconFocused.Render()
		} else {
			return t.EditorPromptYoloIconBlurred.Render()
		}
	}
	if info.Focused {
		return t.EditorPromptYoloDotsFocused.Render()
	}
	return t.EditorPromptYoloDotsBlurred.Render()
}

// planPromptFunc returns the plan mode editor prompt style with plan icon and
// colored dots.
func (m *UI) planPromptFunc(info textarea.PromptInfo) string {
	t := m.com.Styles
	if info.LineNumber == 0 {
		if info.Focused {
			return t.EditorPromptPlanIconFocused.Render()
		}
		return t.EditorPromptPlanIconBlurred.Render()
	}
	if info.Focused {
		return t.EditorPromptPlanDotsFocused.Render()
	}
	return t.EditorPromptPlanDotsBlurred.Render()
}

// closeCompletions closes the completions popup and resets state.
func (m *UI) closeCompletions() {
	m.completionsOpen = false
	m.completionsQuery = ""
	m.completionsStartIndex = 0
	m.completions.Close()
}

// insertCompletionText replaces the @query in the textarea with the given text.
// Returns false if the replacement cannot be performed.
func (m *UI) insertCompletionText(text string) bool {
	value := m.textarea.Value()
	if m.completionsStartIndex > len(value) {
		return false
	}

	word := m.textareaWord()
	endIdx := min(m.completionsStartIndex+len(word), len(value))
	newValue := value[:m.completionsStartIndex] + text + value[endIdx:]
	m.textarea.SetValue(newValue)
	m.textarea.MoveToEnd()
	m.textarea.InsertRune(' ')
	return true
}

// insertFileCompletion inserts the selected file path into the textarea,
// replacing the @query, and adds the file as an attachment.
func (m *UI) insertFileCompletion(path string) tea.Cmd {
	prevHeight := m.textarea.Height()
	if !m.insertCompletionText(path) {
		return nil
	}
	heightCmd := m.handleTextareaHeightChange(prevHeight)

	fileCmd := func() tea.Msg {
		absPath, _ := filepath.Abs(path)

		if m.hasSession() {
			// Skip attachment if file was already read and hasn't been modified.
			lastRead := m.com.App.FileTracker.LastReadTime(context.Background(), m.session.ID, absPath)
			if !lastRead.IsZero() {
				if info, err := os.Stat(path); err == nil && !info.ModTime().After(lastRead) {
					return nil
				}
			}
		} else if slices.Contains(m.sessionFileReads, absPath) {
			return nil
		}

		m.sessionFileReads = append(m.sessionFileReads, absPath)

		// Add file as attachment.
		content, err := os.ReadFile(path)
		if err != nil {
			// If it fails, let the LLM handle it later.
			return nil
		}

		return message.Attachment{
			FilePath: path,
			FileName: filepath.Base(path),
			MimeType: mimeOf(content),
			Content:  content,
		}
	}
	return tea.Batch(heightCmd, fileCmd)
}

// insertMCPResourceCompletion inserts the selected resource into the textarea,
// replacing the @query, and adds the resource as an attachment.
func (m *UI) insertMCPResourceCompletion(item completions.ResourceCompletionValue) tea.Cmd {
	displayText := cmp.Or(item.Title, item.URI)

	prevHeight := m.textarea.Height()
	if !m.insertCompletionText(displayText) {
		return nil
	}
	heightCmd := m.handleTextareaHeightChange(prevHeight)

	resourceCmd := func() tea.Msg {
		contents, err := mcp.ReadResource(
			context.Background(),
			m.com.Store(),
			item.MCPName,
			item.URI,
		)
		if err != nil {
			slog.Warn("Failed to read MCP resource", "uri", item.URI, "error", err)
			return nil
		}
		if len(contents) == 0 {
			return nil
		}

		content := contents[0]
		var data []byte
		if content.Text != "" {
			data = []byte(content.Text)
		} else if len(content.Blob) > 0 {
			data = content.Blob
		}
		if len(data) == 0 {
			return nil
		}

		mimeType := item.MIMEType
		if mimeType == "" && content.MIMEType != "" {
			mimeType = content.MIMEType
		}
		if mimeType == "" {
			mimeType = "text/plain"
		}

		return message.Attachment{
			FilePath: item.URI,
			FileName: displayText,
			MimeType: mimeType,
			Content:  data,
		}
	}
	return tea.Batch(heightCmd, resourceCmd)
}

// completionsPosition returns the X and Y position for the completions popup.
func (m *UI) completionsPosition() image.Point {
	cur := m.textarea.Cursor()
	if cur == nil {
		return image.Point{
			X: m.layout.editor.Min.X,
			Y: m.layout.editor.Min.Y,
		}
	}
	return image.Point{
		X: cur.X + m.layout.editor.Min.X,
		Y: m.layout.editor.Min.Y + cur.Y,
	}
}

// textareaWord returns the current word at the cursor position.
func (m *UI) textareaWord() string {
	return m.textarea.Word()
}

func (m *UI) moveEditorCursorByWord(direction int) {
	value := m.textarea.Value()
	if value == "" {
		return
	}

	runes := []rune(value)
	current := m.textareaCursorRuneIndex(value)

	var target int
	if direction < 0 {
		target = previousWordBoundary(runes, current)
	} else {
		target = nextWordBoundary(runes, current)
	}

	m.setTextareaCursorRuneIndex(runes, target)
}

func (m *UI) textareaCursorRuneIndex(value string) int {
	line := m.textarea.Line()
	col := m.textarea.Column()
	lines := strings.Split(value, "\n")
	if len(lines) == 0 {
		return 0
	}

	line = max(0, min(line, len(lines)-1))
	idx := 0
	for i := range line {
		idx += len([]rune(lines[i])) + 1 // +1 for '\n'
	}

	lineLen := len([]rune(lines[line]))
	col = max(0, min(col, lineLen))
	return idx + col
}

func (m *UI) setTextareaCursorRuneIndex(runes []rune, target int) {
	target = max(0, min(target, len(runes)))
	targetLine, targetCol := runeIndexToLineCol(runes, target)

	m.textarea.MoveToBegin()
	for range targetLine {
		m.textarea.CursorDown()
	}
	m.textarea.SetCursorColumn(targetCol)
}

func runeIndexToLineCol(runes []rune, idx int) (line int, col int) {
	i := 0
	for _, r := range runes {
		if i == idx {
			return line, col
		}
		if r == '\n' {
			line++
			col = 0
		} else {
			col++
		}
		i++
	}
	return line, col
}

func previousWordBoundary(runes []rune, idx int) int {
	if idx <= 0 {
		return 0
	}

	i := idx - 1
	for i >= 0 && unicode.IsSpace(runes[i]) {
		i--
	}
	if i < 0 {
		return 0
	}

	switch {
	case isWordRune(runes[i]):
		for i >= 0 && isWordRune(runes[i]) {
			i--
		}
	case isConnectorRune(runes[i]):
		for i >= 0 && isConnectorRune(runes[i]) {
			i--
		}
		for i >= 0 && isWordRune(runes[i]) {
			i--
		}
	default:
		for i >= 0 && !unicode.IsSpace(runes[i]) && !isWordRune(runes[i]) && !isConnectorRune(runes[i]) {
			i--
		}
	}

	return i + 1
}

func nextWordBoundary(runes []rune, idx int) int {
	n := len(runes)
	if idx >= n {
		return n
	}

	i := idx
	if unicode.IsSpace(runes[i]) {
		for i < n && unicode.IsSpace(runes[i]) {
			i++
		}
		return i
	}

	switch {
	case isWordRune(runes[i]):
		for i < n && isWordRune(runes[i]) {
			i++
		}
		for i < n && isConnectorRune(runes[i]) {
			i++
		}
	case isConnectorRune(runes[i]):
		for i < n && isConnectorRune(runes[i]) {
			i++
		}
	default:
		for i < n && !unicode.IsSpace(runes[i]) && !isWordRune(runes[i]) && !isConnectorRune(runes[i]) {
			i++
		}
	}

	return i
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isConnectorRune(r rune) bool {
	switch r {
	case '-', '_', '.', '/', '\\', ':':
		return true
	default:
		return false
	}
}

// isWhitespace returns true if the byte is a whitespace character.
func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// isAgentBusy returns true if the agent coordinator exists and is currently
// busy processing a request.
func (m *UI) isAgentBusy() bool {
	return m.com.App != nil &&
		m.com.App.AgentCoordinator != nil &&
		m.com.App.AgentCoordinator.IsBusy()
}

// hasSession returns true if there is an active session with a valid ID.
func (m *UI) hasSession() bool {
	return m.session != nil && m.session.ID != ""
}

// mimeOf detects the MIME type of the given content.
func mimeOf(content []byte) string {
	mimeBufferSize := min(512, len(content))
	return http.DetectContentType(content[:mimeBufferSize])
}

var readyPlaceholders = [...]string{
	"Ready!",
	"Ready...",
	"Ready?",
	"Ready for instructions",
}

var workingPlaceholders = [...]string{
	"Working!",
	"Working...",
	"Brrrrr...",
	"Prrrrrrrr...",
	"Processing...",
	"Thinking...",
}

// randomizePlaceholders selects random placeholder text for the textarea's
// ready and working states.
func (m *UI) randomizePlaceholders() {
	m.workingPlaceholder = workingPlaceholders[rand.Intn(len(workingPlaceholders))]
	m.readyPlaceholder = readyPlaceholders[rand.Intn(len(readyPlaceholders))]
}

// renderEditorView renders the editor view with attachments if any.
func (m *UI) renderEditorView(width int) string {
	var attachmentsView string
	if len(m.attachments.List()) > 0 {
		attachmentsView = m.attachments.Render(width)
	}
	return strings.Join([]string{
		attachmentsView,
		m.textarea.View(),
		"", // margin at bottom of editor
	}, "\n")
}

// cacheSidebarLogo renders and caches the sidebar logo at the specified width.
func (m *UI) cacheSidebarLogo(width int) {
	m.sidebarLogo = renderLogo(m.com.Styles, true, width)
}

// sendMessage sends a message with the given content and attachments.
func (m *UI) sendMessage(content string, attachments ...message.Attachment) tea.Cmd {
	if m.com.App.AgentCoordinator == nil {
		return util.ReportError(fmt.Errorf("coder agent is not initialized"))
	}

	var cmds []tea.Cmd
	if !m.hasSession() {
		newSession, err := m.com.App.Sessions.Create(context.Background(), "New Session")
		if err != nil {
			return util.ReportError(err)
		}
		if m.forceCompactMode {
			m.isCompact = true
		}
		if newSession.ID != "" {
			m.session = &newSession
			cmds = append(cmds, m.loadSession(newSession.ID))
		}
		m.setState(uiChat, m.focus)
	}

	ctx := context.Background()
	cmds = append(cmds, func() tea.Msg {
		for _, path := range m.sessionFileReads {
			m.com.App.FileTracker.RecordRead(ctx, m.session.ID, path)
			m.com.App.LSPManager.Start(ctx, path)
		}
		return nil
	})

	// Capture session ID to avoid race with main goroutine updating m.session.
	sessionID := m.session.ID
	runOptions := agent.RunOptions{
		PlanMode: m.planModeEnabled,
	}
	cmds = append(cmds, func() tea.Msg {
		_, err := m.com.App.AgentCoordinator.RunWithOptions(context.Background(), sessionID, content, runOptions, attachments...)
		if err != nil {
			isCancelErr := errors.Is(err, context.Canceled)
			isPermissionErr := errors.Is(err, permission.ErrorPermissionDenied)
			if isCancelErr || isPermissionErr {
				return nil
			}
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  err.Error(),
			}
		}
		return nil
	})
	return tea.Batch(cmds...)
}

func (m *UI) handleLocalPlanCommand(value string) (tea.Cmd, bool) {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(value)))
	if len(fields) == 0 || fields[0] != localPlanCommand {
		return nil, false
	}

	switch len(fields) {
	case 1:
		m.planModeEnabled = !m.planModeEnabled
	case 2:
		switch fields[1] {
		case "on":
			m.planModeEnabled = true
		case "off":
			m.planModeEnabled = false
		default:
			return util.ReportWarn("Usage: /plan [on|off]"), true
		}
	default:
		return util.ReportWarn("Usage: /plan [on|off]"), true
	}

	// Keep modes exclusive: enabling planning mode disables yolo mode.
	if m.planModeEnabled {
		m.com.App.Permissions.SetSkipRequests(false)
	}
	m.setEditorPrompt(m.com.App.Permissions.SkipRequests(), m.planModeEnabled)

	status := "off"
	if m.planModeEnabled {
		status = "on"
	}
	return util.ReportInfo("Planning mode " + status), true
}

func (m *UI) handleLocalBTWCommand(value string) (tea.Cmd, bool, string) {
	trimmed := strings.TrimSpace(value)
	fields := strings.Fields(trimmed)
	if len(fields) == 0 || strings.ToLower(fields[0]) != localBTWCommand {
		return nil, false, ""
	}

	inline := strings.TrimSpace(strings.TrimPrefix(trimmed, fields[0]))
	if inline != "" {
		return nil, true, formatBTWMessage(inline)
	}

	m.btwModeEnabled = true
	return util.ReportInfo("BTW mode armed for next message"), true, ""
}

func (m *UI) prepareOutgoingMessage(content string) string {
	if !m.btwModeEnabled {
		return content
	}
	m.btwModeEnabled = false
	return formatBTWMessage(content)
}

func formatBTWMessage(content string) string {
	return "BTW (side note): " + strings.TrimSpace(content)
}

func (m *UI) runSelfCommand(args ...string) (string, error) {
	if len(os.Args) == 0 || strings.TrimSpace(os.Args[0]) == "" {
		return "", errors.New("cannot execute command: binary path unavailable")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], args...)
	cmd.Env = os.Environ()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errOutput := strings.TrimSpace(stderr.String())
		if errOutput != "" {
			return "", fmt.Errorf("%w: %s", err, errOutput)
		}
		return "", err
	}

	output := strings.TrimSpace(stdout.String())
	if output != "" {
		return output, nil
	}
	return strings.TrimSpace(stderr.String()), nil
}

func (m *UI) showLocalStatus() tea.Cmd {
	return func() tea.Msg {
		cfg := m.com.Config()
		if cfg == nil {
			return util.NewWarnMsg("Status unavailable: configuration not found")
		}

		provider, ok := cfg.Providers.Get(openaicodex.ProviderID)
		if !ok || provider.Disable {
			return util.NewWarnMsg("Status unavailable: openai-codex is not configured")
		}

		accessToken, err := m.com.Store().Resolve(provider.APIKey)
		if err != nil {
			return util.NewWarnMsg("Status unavailable: failed to resolve API key")
		}
		accountID := strings.TrimSpace(provider.ExtraHeaders["chatgpt-account-id"])
		if accountID == "" {
			accountID, err = openai_codex.ExtractAccountID(accessToken)
			if err != nil {
				return util.NewWarnMsg("Status unavailable: failed to resolve account id")
			}
		}

		report, err := openaicodex.FetchUsage(context.Background(), openaicodex.UsageRequest{
			BaseURL:     provider.BaseURL,
			AccessToken: accessToken,
			AccountID:   accountID,
		})
		if err != nil {
			return util.NewWarnMsg("Status unavailable: " + err.Error())
		}
		return util.NewInfoMsg(openaicodex.FormatStatusSummary(report, time.Now()))
	}
}

const cancelTimerDuration = 2 * time.Second

// cancelTimerCmd creates a command that expires the cancel timer.
func cancelTimerCmd() tea.Cmd {
	return tea.Tick(cancelTimerDuration, func(time.Time) tea.Msg {
		return cancelTimerExpiredMsg{}
	})
}

// cancelAgent handles the cancel key press. The first press sets isCanceling to true
// and starts a timer. The second press (before the timer expires) actually
// cancels the agent.
func (m *UI) cancelAgent() tea.Cmd {
	if !m.hasSession() {
		return nil
	}

	coordinator := m.com.App.AgentCoordinator
	if coordinator == nil {
		return nil
	}

	if m.isCanceling {
		// Second escape press - actually cancel the agent.
		m.isCanceling = false
		coordinator.Cancel(m.session.ID)
		// Stop the spinning todo indicator.
		m.todoIsSpinning = false
		m.renderPills()
		return nil
	}

	// Check if there are queued prompts - if so, clear the queue.
	if coordinator.QueuedPrompts(m.session.ID) > 0 {
		coordinator.ClearQueue(m.session.ID)
		return nil
	}

	// First escape press - set canceling state and start timer.
	m.isCanceling = true
	return cancelTimerCmd()
}

// handlePasteMsg handles a paste message.
func (m *UI) handlePasteMsg(msg tea.PasteMsg) tea.Cmd {
	if m.dialog.HasDialogs() {
		return m.handleDialogMsg(msg)
	}

	if m.focus != uiFocusEditor {
		return nil
	}

	if shouldAttemptClipboardImagePaste(msg) {
		return m.pasteAttachmentFromClipboard
	}

	if hasPasteExceededThreshold(msg) {
		return func() tea.Msg {
			content := []byte(msg.Content)
			if int64(len(content)) > common.MaxAttachmentSize {
				return util.ReportWarn("Paste is too big (>5mb)")
			}
			name := fmt.Sprintf("paste_%d.txt", m.pasteIdx())
			mimeBufferSize := min(512, len(content))
			mimeType := http.DetectContentType(content[:mimeBufferSize])
			return message.Attachment{
				FileName: name,
				FilePath: name,
				MimeType: mimeType,
				Content:  content,
			}
		}
	}

	// Attempt to parse pasted content as file paths. If possible to parse,
	// all files exist and are valid, add as attachments.
	// Otherwise, paste as text.
	paths := fsext.ParsePastedFiles(msg.Content)
	allExistsAndValid := func() bool {
		if len(paths) == 0 {
			return false
		}
		for _, path := range paths {
			fileInfo, err := os.Stat(path)
			if err != nil || fileInfo.IsDir() {
				return false
			}
		}
		return true
	}
	if !allExistsAndValid() {
		prevHeight := m.textarea.Height()
		return m.updateTextareaWithPrevHeight(msg, prevHeight)
	}

	var cmds []tea.Cmd
	for _, path := range paths {
		cmds = append(cmds, m.handleFilePathPaste(path))
	}
	return tea.Batch(cmds...)
}

func shouldAttemptClipboardImagePaste(msg tea.PasteMsg) bool {
	return strings.TrimSpace(msg.Content) == ""
}

func hasPasteExceededThreshold(msg tea.PasteMsg) bool {
	var (
		lineCount = 0
		colCount  = 0
	)
	for line := range strings.SplitSeq(msg.Content, "\n") {
		lineCount++
		colCount = max(colCount, len(line))

		if lineCount > pasteLinesThreshold || colCount > pasteColsThreshold {
			return true
		}
	}
	return false
}

// handleFilePathPaste handles a pasted file path.
func (m *UI) handleFilePathPaste(path string) tea.Cmd {
	return func() tea.Msg {
		fileInfo, err := os.Stat(path)
		if err != nil {
			return util.ReportError(err)
		}
		if fileInfo.IsDir() {
			return util.ReportWarn("Cannot attach a directory")
		}
		if fileInfo.Size() > common.MaxAttachmentSize {
			return util.ReportWarn("File is too big (>5mb)")
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return util.ReportError(err)
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

// pasteAttachmentFromClipboard reads data from the system clipboard and
// creates an attachment. If no image data is found, it falls back to
// interpreting clipboard text as a file path or normal pasted text.
func (m *UI) pasteAttachmentFromClipboard() tea.Msg {
	// Prefer OS-specific shell fallback first (wl-paste/xclip/xsel on Linux).
	// This path is more reliable on Wayland than nativeclipboard image reads.
	imageData, err := readClipboardImageFallback()
	if err != nil || len(imageData) == 0 {
		imageData, err = readClipboard(clipboardFormatImage)
	}
	if int64(len(imageData)) > common.MaxAttachmentSize {
		return util.InfoMsg{
			Type: util.InfoTypeError,
			Msg:  "File too large, max 5MB",
		}
	}
	idx := m.pasteIdx()
	name := fmt.Sprintf("paste_%d.png", idx)
	if err == nil && len(imageData) > 0 {
		mimeType := mimeOf(imageData)
		if !strings.HasPrefix(mimeType, "image/") {
			return util.InfoMsg{
				Type: util.InfoTypeWarn,
				Msg:  "Clipboard content is not an image",
			}
		}
		return message.Attachment{
			FilePath: name,
			FileName: fmt.Sprintf("Image #%d", idx),
			MimeType: mimeType,
			Content:  imageData,
		}
	}

	textData, textErr := readClipboard(clipboardFormatText)
	if textErr == nil && len(textData) > 0 {
		text := strings.TrimSpace(string(textData))
		if decoded, mimeType, ok := parseClipboardImageDataURL(text); ok {
			if int64(len(decoded)) > common.MaxAttachmentSize {
				return util.InfoMsg{
					Type: util.InfoTypeError,
					Msg:  "File too large, max 5MB",
				}
			}
			return message.Attachment{
				FilePath: name,
				FileName: fmt.Sprintf("Image #%d", idx),
				MimeType: mimeType,
				Content:  decoded,
			}
		}

		paths := fsext.ParsePastedFiles(text)
		if len(paths) == 1 {
			if attachment, ok := attachmentFromPath(paths[0]); ok {
				return attachment
			}
		}

		return tea.PasteMsg{Content: string(textData)}
	}

	if err != nil {
		return util.InfoMsg{
			Type: util.InfoTypeWarn,
			Msg:  fmt.Sprintf("Clipboard image paste failed: %v", err),
		}
	}
	if textErr != nil {
		return util.InfoMsg{
			Type: util.InfoTypeWarn,
			Msg:  fmt.Sprintf("Clipboard text fallback failed: %v", textErr),
		}
	}
	return util.NewInfoMsg("Clipboard does not currently contain text, an image, or a file path")
}

func attachmentFromPath(path string) (message.Attachment, bool) {
	fileInfo, err := os.Stat(path)
	if err != nil || fileInfo.IsDir() || fileInfo.Size() > common.MaxAttachmentSize {
		return message.Attachment{}, false
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return message.Attachment{}, false
	}
	mimeBufferSize := min(512, len(content))
	return message.Attachment{
		FilePath: path,
		FileName: filepath.Base(path),
		MimeType: http.DetectContentType(content[:mimeBufferSize]),
		Content:  content,
	}, true
}

func parseClipboardImageDataURL(raw string) ([]byte, string, bool) {
	decoded, err := url.QueryUnescape(raw)
	if err != nil {
		decoded = raw
	}
	if !strings.HasPrefix(decoded, "data:image/") {
		return nil, "", false
	}
	comma := strings.IndexByte(decoded, ',')
	if comma <= 0 {
		return nil, "", false
	}
	header := decoded[:comma]
	payload := decoded[comma+1:]
	if !strings.Contains(header, ";base64") {
		return nil, "", false
	}
	mimeType := strings.TrimPrefix(strings.Split(header, ";")[0], "data:")
	b, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(b) == 0 {
		return nil, "", false
	}
	return b, mimeType, true
}

var (
	pasteTextRE  = regexp.MustCompile(`^paste_(\d+)\.txt$`)
	pasteImageRE = regexp.MustCompile(`^paste_(\d+)\.png$`)
	imageNameRE  = regexp.MustCompile(`^Image #(\d+)$`)
)

func (m *UI) pasteIdx() int {
	result := 0
	for _, at := range m.attachments.List() {
		for _, re := range []*regexp.Regexp{pasteTextRE, pasteImageRE, imageNameRE} {
			found := re.FindStringSubmatch(at.FileName)
			if len(found) == 0 {
				continue
			}
			idx, err := strconv.Atoi(found[1])
			if err == nil {
				result = max(result, idx)
			}
		}
	}
	return result + 1
}

// drawSessionDetails draws the session details in compact mode.
func (m *UI) drawSessionDetails(scr uv.Screen, area uv.Rectangle) {
	if m.session == nil {
		return
	}

	s := m.com.Styles

	width := area.Dx() - s.CompactDetails.View.GetHorizontalFrameSize()
	height := area.Dy() - s.CompactDetails.View.GetVerticalFrameSize()

	title := s.CompactDetails.Title.Width(width).MaxHeight(2).Render(m.session.Title)
	blocks := []string{
		title,
		"",
		m.modelInfo(width),
		"",
	}

	detailsHeader := lipgloss.JoinVertical(
		lipgloss.Left,
		blocks...,
	)

	version := s.CompactDetails.Version.Foreground(s.Border).Width(width).AlignHorizontal(lipgloss.Right).Render(version.Version)

	remainingHeight := height - lipgloss.Height(detailsHeader) - lipgloss.Height(version)

	const maxSectionWidth = 50
	sectionWidth := min(maxSectionWidth, width/3-2) // account for 2 spaces
	maxItemsPerSection := remainingHeight - 3       // Account for section title and spacing

	lspSection := m.lspInfo(sectionWidth, maxItemsPerSection, false)
	mcpSection := m.mcpInfo(sectionWidth, maxItemsPerSection, false)
	filesSection := m.filesInfo(m.com.Store().WorkingDir(), sectionWidth, maxItemsPerSection, false)
	sections := lipgloss.JoinHorizontal(lipgloss.Top, filesSection, " ", lspSection, " ", mcpSection)
	uv.NewStyledString(
		s.CompactDetails.View.
			Width(area.Dx()).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					detailsHeader,
					sections,
					version,
				),
			),
	).Draw(scr, area)
}

func (m *UI) runMCPPrompt(clientID, promptID string, arguments map[string]string) tea.Cmd {
	load := func() tea.Msg {
		prompt, err := commands.GetMCPPrompt(m.com.Store(), clientID, promptID, arguments)
		if err != nil {
			// TODO: make this better
			return util.ReportError(err)()
		}

		if prompt == "" {
			return nil
		}
		return sendMessageMsg{
			Content: prompt,
		}
	}

	var cmds []tea.Cmd
	if cmd := m.dialog.StartLoading(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	cmds = append(cmds, load, func() tea.Msg {
		return closeDialogMsg{}
	})

	return tea.Sequence(cmds...)
}

func (m *UI) handleStateChanged() tea.Cmd {
	return func() tea.Msg {
		m.com.App.UpdateAgentModel(context.Background())
		return mcpStateChangedMsg{
			states: mcp.GetStates(),
		}
	}
}

func handleMCPPromptsEvent(name string) tea.Cmd {
	return func() tea.Msg {
		mcp.RefreshPrompts(context.Background(), name)
		return nil
	}
}

func handleMCPToolsEvent(cfg *config.ConfigStore, name string) tea.Cmd {
	return func() tea.Msg {
		mcp.RefreshTools(
			context.Background(),
			cfg,
			name,
		)
		return nil
	}
}

func handleMCPResourcesEvent(name string) tea.Cmd {
	return func() tea.Msg {
		mcp.RefreshResources(context.Background(), name)
		return nil
	}
}

func (m *UI) copyChatHighlight() tea.Cmd {
	text := m.chat.HighlightContent()
	return common.CopyToClipboardWithCallback(
		text,
		"Selected text copied to clipboard",
		func() tea.Msg {
			m.chat.ClearMouse()
			return nil
		},
	)
}

func (m *UI) enableDockerMCP() tea.Msg {
	store := m.com.Store()
	// Stage Docker MCP in memory first so startup and persistence can be atomic.
	mcpConfig, err := store.PrepareDockerMCPConfig()
	if err != nil {
		return util.ReportError(err)()
	}

	ctx := context.Background()
	if err := mcp.InitializeSingle(ctx, config.DockerMCPName, store); err != nil {
		// Roll back runtime and in-memory state when startup fails.
		disableErr := mcp.DisableSingle(store, config.DockerMCPName)
		delete(store.Config().MCP, config.DockerMCPName)
		return util.ReportError(fmt.Errorf("failed to start docker MCP: %w", errors.Join(err, disableErr)))()
	}

	if err := store.PersistDockerMCPConfig(mcpConfig); err != nil {
		// Roll back runtime and in-memory state if persistence fails.
		disableErr := mcp.DisableSingle(store, config.DockerMCPName)
		delete(store.Config().MCP, config.DockerMCPName)
		return util.ReportError(fmt.Errorf("docker MCP started but failed to persist configuration: %w", errors.Join(err, disableErr)))()
	}

	return util.NewInfoMsg("Docker MCP enabled and started successfully")
}

func (m *UI) disableDockerMCP() tea.Msg {
	store := m.com.Store()
	// Close the Docker MCP client.
	if err := mcp.DisableSingle(store, config.DockerMCPName); err != nil {
		return util.ReportError(fmt.Errorf("failed to disable docker MCP: %w", err))()
	}

	// Remove from config and persist.
	if err := store.DisableDockerMCP(); err != nil {
		return util.ReportError(err)()
	}

	return util.NewInfoMsg("Docker MCP disabled successfully")
}

// renderLogo renders the Franz logo with the given styles and dimensions.
func renderLogo(t *styles.Styles, compact bool, width int) string {
	return logo.Render(t, version.Version, compact, logo.Opts{
		FieldColor:   t.LogoFieldColor,
		TitleColorA:  t.LogoTitleColorA,
		TitleColorB:  t.LogoTitleColorB,
		CharmColor:   t.LogoCharmColor,
		VersionColor: t.LogoVersionColor,
		Width:        width,
	})
}
