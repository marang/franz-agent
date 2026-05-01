package dialog

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/marang/franz-agent/internal/ui/common"
	"github.com/marang/franz-agent/internal/ui/list"
	"github.com/marang/franz-agent/internal/ui/styles"
)

// SkillsSHSearchID is the identifier for the skills.sh search dialog.
const SkillsSHSearchID = "skills_sh_search"

const (
	skillsSHSearchDebounceDuration = 350 * time.Millisecond
	installErrorsTitle             = "install errors"
	exitStatusPrefix               = "exit status 1:"
)

const (
	focusLabelView               = "View:"
	focusLabelAction             = "Action:"
	focusLabelSafety             = "Installation Safety:"
	focusLabelSkillsSelection    = "Skills Selection:"
	focusLabelSkillSearchResults = "Skill Search Results:"
)

const (
	safetySummaryText = "Every skills.sh install is audited before activation."
	safetyDetailLine1 = "We block prompt-injection patterns before a skill is installed:"
	safetyDetailLine2 = "ignore_instructions, prompt_exfiltration, authority_impersonation,"
	safetyDetailLine3 = "tool_bypass, and data_exfiltration."
)

const (
	installedActionEnable = iota
	installedActionDisable
	installedActionFixPerms
	installedActionDelete
	installedActionRefresh
	installedActionSources
)

const (
	searchActionInstall = iota
	searchActionOpenLink
	searchActionRefresh
	searchActionSources
)

var (
	installedActionLabels = []string{"Enable", "Disable", "Fix Perms", "Delete", "Refresh", "Sources"}
	searchActionLabels    = []string{"Install", "Open Link", "Refresh", "Sources"}
)

type skillsSHSearchDebouncedMsg struct {
	RequestID int
}

type skillsManagerTab uint8

const (
	skillsManagerTabInstalled skillsManagerTab = iota
	skillsManagerTabSearch
)

type skillsManagerFocusArea uint8

const (
	skillsFocusTabs skillsManagerFocusArea = iota
	skillsFocusInput
	skillsFocusActions
	skillsFocusSafety
	skillsFocusList
)

// SkillsSHSearch provides an interactive skills.sh search and install flow.
type SkillsSHSearch struct {
	com   *common.Common
	help  help.Model
	input textinput.Model
	list  *list.List

	keyMap struct {
		Select,
		Details,
		Install,
		Enable,
		Disable,
		Delete,
		FixPerms,
		OpenURL,
		Update,
		Sources,
		Refresh,
		ActionPrev,
		ActionNext,
		ExecuteAction,
		TabPrev,
		TabNext,
		Tab,
		BackTab,
		Next,
		Previous,
		Close key.Binding
	}

	tab skillsManagerTab

	items             []*skillsSHSearchItem
	installed         []*skillsInstalledItem
	selected          map[string]struct{}
	selectedInstalled map[string]struct{}
	tracked           map[string]struct{}
	installedSources  map[string]struct{}

	statusLine string
	loading    bool
	spinner    spinner.Model

	installQueue      []string
	installTotal      int
	installDone       int
	installOK         []string
	installFailed     map[string]string
	lastInstallErrors []string

	requestSeq      int
	activeRequestID int

	actionIndexInstalled int
	actionIndexSearch    int
	focusArea            skillsManagerFocusArea
	showSafetyInfo       bool
	renderInnerWidth     int
}

var (
	_ Dialog        = (*SkillsSHSearch)(nil)
	_ LoadingDialog = (*SkillsSHSearch)(nil)
)

// NewSkillsSHSearch creates a new skills.sh search dialog.
func NewSkillsSHSearch(com *common.Common) *SkillsSHSearch {
	s := &SkillsSHSearch{
		com:               com,
		list:              list.NewList(),
		selected:          make(map[string]struct{}),
		selectedInstalled: make(map[string]struct{}),
		tracked:           make(map[string]struct{}),
		installedSources:  make(map[string]struct{}),
	}

	s.help = help.New()
	s.help.Styles = com.Styles.DialogHelpStyles()

	s.input = textinput.New()
	s.input.SetVirtualCursor(false)
	s.input.SetStyles(com.Styles.TextInput)
	s.input.Prompt = ">"
	s.input.Placeholder = "Search skills (for example: testing, go, react)"
	s.input.Focus()

	s.keyMap.Select = key.NewBinding(
		key.WithKeys("space"),
		key.WithHelp("space", "toggle select"),
	)
	s.keyMap.Details = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "details"),
	)
	s.keyMap.Install = key.NewBinding(
		key.WithKeys("ctrl+k"),
		key.WithHelp("ctrl+k", "install selected"),
	)
	s.keyMap.Enable = key.NewBinding(
		key.WithKeys("ctrl+e"),
		key.WithHelp("ctrl+e", "enable"),
	)
	s.keyMap.Disable = key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl+d", "disable"),
	)
	s.keyMap.Delete = key.NewBinding(
		key.WithKeys("ctrl+x"),
		key.WithHelp("ctrl+x", "delete"),
	)
	s.keyMap.FixPerms = key.NewBinding(
		key.WithKeys("ctrl+f"),
		key.WithHelp("ctrl+f", "fix perms"),
	)
	s.keyMap.OpenURL = key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl+d", "open details"),
	)
	s.keyMap.Update = key.NewBinding(
		key.WithKeys("ctrl+u"),
		key.WithHelp("ctrl+u", "update"),
	)
	s.keyMap.Sources = key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("ctrl+s", "sources"),
	)
	s.keyMap.Refresh = key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "refresh"),
	)
	s.keyMap.ActionPrev = key.NewBinding(
		key.WithKeys("ctrl+left"),
		key.WithHelp("ctrl+←", "prev action"),
	)
	s.keyMap.ActionNext = key.NewBinding(
		key.WithKeys("ctrl+right"),
		key.WithHelp("ctrl+→", "next action"),
	)
	s.keyMap.TabPrev = key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "installed tab"),
	)
	s.keyMap.TabNext = key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "search tab"),
	)
	s.keyMap.ExecuteAction = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "run action"),
	)
	s.keyMap.Tab = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next area"),
	)
	s.keyMap.BackTab = key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "prev area"),
	)
	s.keyMap.Next = key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "next"),
	)
	s.keyMap.Previous = key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "previous"),
	)
	closeKey := CloseKey
	closeKey.SetHelp("esc", "cancel")
	s.keyMap.Close = closeKey

	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = com.Styles.Dialog.Spinner
	s.spinner = spin
	s.tab = skillsManagerTabInstalled
	s.focusArea = skillsFocusTabs

	return s
}

// ID implements Dialog.
func (s *SkillsSHSearch) ID() string {
	return SkillsSHSearchID
}

// Cursor returns the cursor position relative to the dialog.
func (s *SkillsSHSearch) Cursor() *tea.Cursor {
	cur := InputCursor(s.com.Styles, s.input.Cursor())
	if cur == nil {
		return nil
	}
	if s.tab == skillsManagerTabSearch && s.focusArea == skillsFocusInput {
		// Search mode renders:
		// spacer, tabs, spacer, safety block, spacer, then input.
		cur.Y += s.searchInputCursorYOffset()
		return cur
	}
	return nil
}

func (s *SkillsSHSearch) searchInputCursorYOffset() int {
	// Rows before input in search layout:
	// 1 spacer + 1 tabs + 1 spacer + safety block height.
	safetyHeight := lipgloss.Height(s.safetyInfoView(max(1, s.renderInnerWidth)))
	return 3 + safetyHeight
}

// HandleMsg implements Dialog.
func (s *SkillsSHSearch) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case SkillsSHSearchResultsMsg:
		if msg.RequestID != s.activeRequestID {
			return nil
		}
		s.loading = false
		s.applyResults(msg.Results)
		if msg.Err != nil {
			s.statusLine = "Search failed: " + msg.Err.Error()
			return nil
		}
		switch len(msg.Results) {
		case 0:
			s.statusLine = "No skills found."
		case 1:
			s.statusLine = "1 skill found."
		default:
			s.statusLine = fmt.Sprintf("%d skills found.", len(msg.Results))
		}
		return nil
	case SkillsSHSourcesLoadedMsg:
		s.loading = false
		if msg.Err != nil {
			s.statusLine = "Failed to load tracked sources: " + msg.Err.Error()
			return nil
		}
		s.applyTrackedSources(msg.Sources)
		s.statusLine = fmt.Sprintf("Tracked sources: %d", len(msg.Sources))
		return nil
	case SkillsInstalledLoadedMsg:
		s.loading = false
		if msg.Err != nil {
			s.statusLine = "Failed to load installed skills: " + msg.Err.Error()
			return nil
		}
		s.applyInstalled(msg.Items)
		return nil
	case SkillsSHInstallCompletedMsg:
		if len(msg.Installed) == 0 && len(msg.Failed) == 0 {
			s.statusLine = "No selected skills to install."
			return ActionSkillsInstalledRefreshRequest{}
		}
		if len(msg.Failed) == 0 {
			s.statusLine = fmt.Sprintf("Installed %d skill source(s).", len(msg.Installed))
			for _, source := range msg.Installed {
				s.tracked[source] = struct{}{}
				delete(s.selected, source)
			}
			s.syncItemState()
			s.tab = skillsManagerTabInstalled
			return ActionSkillsInstalledRefreshRequest{}
		}

		s.statusLine = fmt.Sprintf("Installed %d, failed %d.", len(msg.Installed), len(msg.Failed))
		for _, source := range msg.Installed {
			s.tracked[source] = struct{}{}
			delete(s.selected, source)
		}
		s.syncItemState()
		return ActionSkillsInstalledRefreshRequest{}
	case SkillsSHInstallStepCompletedMsg:
		s.installDone++
		if msg.Err != nil {
			if s.installFailed == nil {
				s.installFailed = make(map[string]string)
			}
			s.installFailed[msg.Source] = msg.Err.Error()
		} else {
			s.installOK = append(s.installOK, msg.Source)
		}

		if len(s.installQueue) > 0 {
			next := s.installQueue[0]
			s.installQueue = s.installQueue[1:]
			s.statusLine = fmt.Sprintf(
				"Installing %d/%d: %s",
				s.installDone+1,
				s.installTotal,
				next,
			)
			return ActionSkillsSHInstallSource{Source: next}
		}

		s.loading = false
		for _, source := range s.installOK {
			s.tracked[source] = struct{}{}
			delete(s.selected, source)
		}
		s.syncItemState()
		s.tab = skillsManagerTabInstalled
		if len(s.installFailed) == 0 {
			s.statusLine = fmt.Sprintf("Installation %d/%d done.", s.installDone, s.installTotal)
		} else {
			firstSource, firstErr := firstInstallFailure(s.installFailed)
			s.statusLine = fmt.Sprintf("Installation %d/%d finished: %d failed. %s: %s", s.installDone, s.installTotal, len(s.installFailed), firstSource, firstErr)
			s.lastInstallErrors = renderInstallFailures(s.installFailed)
		}
		s.resetInstallState()
		return ActionSkillsInstalledRefreshRequest{}
	case skillsSHSearchDebouncedMsg:
		if msg.RequestID != s.activeRequestID {
			return nil
		}
		return s.requestSearch(msg.RequestID)
	case spinner.TickMsg:
		if s.loading {
			var cmd tea.Cmd
			s.spinner, cmd = s.spinner.Update(msg)
			return ActionCmd{Cmd: cmd}
		}
	case tea.KeyPressMsg:

		switch {
		case key.Matches(msg, s.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, s.keyMap.Tab):
			s.moveFocus(1)
			return nil
		case key.Matches(msg, s.keyMap.BackTab):
			s.moveFocus(-1)
			return nil
		case s.focusArea == skillsFocusTabs && key.Matches(msg, s.keyMap.TabPrev):
			s.setTab(skillsManagerTabInstalled)
			return nil
		case s.focusArea == skillsFocusTabs && key.Matches(msg, s.keyMap.TabNext):
			s.setTab(skillsManagerTabSearch)
			return nil
		case s.focusArea == skillsFocusActions && key.Matches(msg, s.keyMap.TabPrev):
			s.shiftAction(-1)
			return nil
		case s.focusArea == skillsFocusActions && key.Matches(msg, s.keyMap.TabNext):
			s.shiftAction(1)
			return nil
		case s.focusArea == skillsFocusActions && key.Matches(msg, s.keyMap.ActionPrev):
			s.shiftAction(-1)
			return nil
		case s.focusArea == skillsFocusActions && key.Matches(msg, s.keyMap.ActionNext):
			s.shiftAction(1)
			return nil
		case key.Matches(msg, s.keyMap.ExecuteAction):
			if s.tab == skillsManagerTabSearch && s.focusArea == skillsFocusSafety {
				s.showSafetyInfo = !s.showSafetyInfo
				return nil
			}
			return s.executeSelectedAction()
		case key.Matches(msg, s.keyMap.Previous):
			if s.focusArea == skillsFocusList {
				if s.list.IsSelectedFirst() || s.list.Selected() <= 0 {
					return nil
				}
				s.list.SelectPrev()
				s.syncFocus()
				s.list.ScrollToSelected()
				return nil
			}
			s.moveFocus(-1)
			return nil
		case key.Matches(msg, s.keyMap.Next):
			if s.focusArea == skillsFocusList {
				if s.list.Selected() < 0 && s.list.Len() > 0 {
					s.list.SetSelected(0)
					s.syncFocus()
					s.list.ScrollToSelected()
					return nil
				}
				if !s.list.IsSelectedLast() {
					s.list.SelectNext()
					s.syncFocus()
					s.list.ScrollToSelected()
					return nil
				}
				return nil
			}
			s.moveFocus(1)
			return nil
		case key.Matches(msg, s.keyMap.Update):
			return ActionSkillsSHUpdate{}
		case key.Matches(msg, s.keyMap.Sources):
			return ActionSkillsSHSources{}
		case s.focusArea == skillsFocusList && key.Matches(msg, s.keyMap.Select):
			if s.tab == skillsManagerTabSearch {
				s.toggleSelectedFocused()
				return nil
			}
			s.toggleSelectedInstalledFocused()
			return nil
		case key.Matches(msg, s.keyMap.Install):
			if s.tab != skillsManagerTabSearch {
				s.statusLine = "Switch to Search view to install."
				return nil
			}
			if s.loading {
				s.statusLine = "Please wait for current operation to finish."
				return nil
			}
			sources := s.selectedSources()
			if len(sources) == 0 {
				s.statusLine = "Select at least one skill first."
				return nil
			}
			s.startInstallQueue(sources)
			next := s.popNextInstallSource()
			if next == "" {
				s.loading = false
				s.statusLine = "No selected skills to install."
				return nil
			}
			s.statusLine = fmt.Sprintf("Installing 1/%d: %s", s.installTotal, next)
			return ActionSkillsSHInstallSource{Source: next}
		case s.tab == skillsManagerTabInstalled && key.Matches(msg, s.keyMap.Enable):
			names := s.selectedInstalledNames()
			if len(names) == 0 {
				s.statusLine = "Select at least one installed skill first."
				return nil
			}
			return ActionSkillsSetDisabledBatch{Names: names, Disabled: false}
		case s.tab == skillsManagerTabInstalled && key.Matches(msg, s.keyMap.Disable):
			names := s.selectedInstalledNames()
			if len(names) == 0 {
				s.statusLine = "Select at least one installed skill first."
				return nil
			}
			return ActionSkillsSetDisabledBatch{Names: names, Disabled: true}
		case s.tab == skillsManagerTabInstalled && key.Matches(msg, s.keyMap.Delete):
			names := s.selectedInstalledNames()
			if len(names) == 0 {
				s.statusLine = "Select at least one installed skill first."
				return nil
			}
			return ActionOpenConfirm{
				Title:   "Delete Skills",
				Message: fmt.Sprintf("Delete %d selected skill(s)?", len(names)),
				Payload: ActionSkillsDeleteBatch{Names: names},
			}
		case s.tab == skillsManagerTabInstalled && key.Matches(msg, s.keyMap.FixPerms):
			names := s.selectedInstalledNames()
			if len(names) == 0 {
				s.statusLine = "Select at least one installed skill first."
				return nil
			}
			return ActionSkillsFixPerms{Names: names}
		case s.tab == skillsManagerTabSearch && key.Matches(msg, s.keyMap.OpenURL):
			item := s.focusedItem()
			if item == nil {
				return nil
			}
			if strings.TrimSpace(item.result.DetailsURL) == "" {
				s.statusLine = "Details URL unavailable."
				return nil
			}
			return ActionSkillsSHOpenDetails{URL: item.result.DetailsURL}
		case key.Matches(msg, s.keyMap.Refresh):
			if s.tab == skillsManagerTabInstalled {
				return ActionSkillsInstalledRefreshRequest{}
			}
			s.requestSeq++
			s.activeRequestID = s.requestSeq
			return s.requestSearch(s.activeRequestID)
		default:
			if s.tab != skillsManagerTabSearch {
				return nil
			}
			if s.focusArea != skillsFocusInput {
				return nil
			}
			if isCtrlBackspace(msg) {
				s.input.SetValue("")
				s.loading = false
				s.items = nil
				s.list.SetItems()
				s.statusLine = "Type to search skills.sh."
				return nil
			}
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			_ = cmd

			s.requestSeq++
			s.activeRequestID = s.requestSeq
			if strings.TrimSpace(s.input.Value()) == "" {
				s.loading = false
				s.items = nil
				s.list.SetItems()
				s.statusLine = "Type to search skills.sh."
				return nil
			}
			s.loading = true
			return ActionCmd{
				Cmd: tea.Tick(skillsSHSearchDebounceDuration, func(time.Time) tea.Msg {
					return skillsSHSearchDebouncedMsg{RequestID: s.activeRequestID}
				}),
			}
		}
	}
	return nil
}

func (s *SkillsSHSearch) requestSearch(requestID int) Action {
	query := strings.TrimSpace(s.input.Value())
	if query == "" {
		return nil
	}
	s.loading = true
	return ActionSkillsSHSearchRequest{
		Query:     query,
		RequestID: requestID,
	}
}

func (s *SkillsSHSearch) applyResults(results []SkillsSHSearchResult) {
	previous := ""
	if item := s.focusedItem(); item != nil {
		previous = item.result.InstallSource
	}

	s.items = make([]*skillsSHSearchItem, 0, len(results))
	renderItems := make([]list.Item, 0, len(results))
	for i := range results {
		item := &skillsSHSearchItem{
			t:      s.com.Styles,
			result: results[i],
		}
		if _, ok := s.selected[item.result.InstallSource]; ok {
			item.selected = true
		}
		item.installed = isSkillsSourceTracked(item.result.InstallSource, s.installedSources)
		s.items = append(s.items, item)
		renderItems = append(renderItems, item)
	}

	s.list.SetItems(renderItems...)
	if len(s.items) == 0 {
		s.list.SetSelected(-1)
		if s.focusArea == skillsFocusList {
			s.setFocusArea(skillsFocusActions)
		}
		return
	}

	selectedIndex := 0
	for i, item := range s.items {
		if item.result.InstallSource == previous {
			selectedIndex = i
			break
		}
	}
	s.list.SetSelected(selectedIndex)
	s.syncFocus()
	s.list.ScrollToSelected()
}

func (s *SkillsSHSearch) applyTrackedSources(sources []string) {
	tracked := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		tracked[source] = struct{}{}
	}
	s.tracked = tracked
	s.syncItemState()
}

func (s *SkillsSHSearch) applyInstalled(items []SkillsInstalledItem) {
	previous := ""
	if item := s.focusedInstalledItem(); item != nil {
		previous = item.name
	}

	s.installed = make([]*skillsInstalledItem, 0, len(items))
	installedSources := make(map[string]struct{}, len(items))
	renderItems := make([]list.Item, 0, len(items))
	for i := range items {
		item := &skillsInstalledItem{
			t:                  s.com.Styles,
			name:               items[i].Name,
			description:        items[i].Description,
			path:               items[i].Path,
			skillFile:          items[i].SkillFile,
			disabled:           items[i].Disabled,
			blocked:            items[i].Blocked,
			blockReasons:       slices.Clone(items[i].BlockReasons),
			permissionWarnings: slices.Clone(items[i].PermissionWarnings),
			origin:             strings.TrimSpace(items[i].Origin),
		}
		if _, ok := s.selectedInstalled[item.name]; ok {
			item.checked = true
		}
		if item.origin != "" {
			installedSources[item.origin] = struct{}{}
		}
		s.installed = append(s.installed, item)
		renderItems = append(renderItems, item)
	}

	s.installedSources = installedSources
	s.syncItemState()

	if s.tab == skillsManagerTabInstalled {
		s.list.SetItems(renderItems...)
	}
	if len(s.installed) == 0 {
		s.list.SetSelected(-1)
		if s.focusArea == skillsFocusList {
			s.setFocusArea(skillsFocusActions)
		}
		return
	}
	selectedIndex := 0
	for i, item := range s.installed {
		if item.name == previous {
			selectedIndex = i
			break
		}
	}
	s.list.SetSelected(selectedIndex)
	s.syncFocus()
	s.list.ScrollToSelected()
}

func (s *SkillsSHSearch) syncItemState() {
	for _, item := range s.items {
		_, item.selected = s.selected[item.result.InstallSource]
		item.installed = isSkillsSourceTracked(item.result.InstallSource, s.installedSources)
	}
}

func (s *SkillsSHSearch) switchTab() {
	if s.tab == skillsManagerTabInstalled {
		s.setTab(skillsManagerTabSearch)
	} else {
		s.setTab(skillsManagerTabInstalled)
	}
}

func (s *SkillsSHSearch) setTab(tab skillsManagerTab) {
	s.tab = tab
	if s.tab == skillsManagerTabSearch {
		renderItems := make([]list.Item, 0, len(s.items))
		for _, item := range s.items {
			renderItems = append(renderItems, item)
		}
		s.list.SetItems(renderItems...)
	} else {
		renderItems := make([]list.Item, 0, len(s.installed))
		for _, item := range s.installed {
			renderItems = append(renderItems, item)
		}
		s.list.SetItems(renderItems...)
	}
	if s.list.Len() > 0 && s.list.Selected() < 0 {
		s.list.SetSelected(0)
	}
	s.setFocusArea(skillsFocusTabs)
}

func (s *SkillsSHSearch) syncFocus() {
	selected := s.list.Selected()
	for i, item := range s.items {
		item.focused = s.tab == skillsManagerTabSearch && s.focusArea == skillsFocusList && i == selected
	}
	for i, item := range s.installed {
		item.focused = s.tab == skillsManagerTabInstalled && s.focusArea == skillsFocusList && i == selected
	}
}

func (s *SkillsSHSearch) focusAreas() []skillsManagerFocusArea {
	hasItems := s.list.Len() > 0
	if s.tab == skillsManagerTabSearch {
		areas := []skillsManagerFocusArea{skillsFocusTabs, skillsFocusInput, skillsFocusActions, skillsFocusSafety}
		if hasItems {
			areas = append(areas, skillsFocusList)
		}
		return areas
	}
	areas := []skillsManagerFocusArea{skillsFocusTabs, skillsFocusActions}
	if hasItems {
		areas = append(areas, skillsFocusList)
	}
	return areas
}

func (s *SkillsSHSearch) setFocusArea(area skillsManagerFocusArea) {
	allowed := false
	for _, candidate := range s.focusAreas() {
		if candidate == area {
			allowed = true
			break
		}
	}
	if !allowed {
		area = skillsFocusTabs
	}
	s.focusArea = area
	if s.tab == skillsManagerTabSearch && s.focusArea == skillsFocusInput {
		s.input.Focus()
	} else {
		s.input.Blur()
	}
	s.syncFocus()
}

func (s *SkillsSHSearch) moveFocus(delta int) {
	areas := s.focusAreas()
	if len(areas) == 0 {
		return
	}
	idx := 0
	for i, area := range areas {
		if area == s.focusArea {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(areas)) % len(areas)
	s.setFocusArea(areas[idx])
}

// StartLoading starts animated loading feedback for this dialog.
func (s *SkillsSHSearch) StartLoading() tea.Cmd {
	if !s.loading {
		s.loading = true
	}
	return s.spinner.Tick
}

// StopLoading stops animated loading feedback for this dialog.
func (s *SkillsSHSearch) StopLoading() {
	s.loading = false
}

func (s *SkillsSHSearch) toggleSelectedFocused() {
	item := s.focusedItem()
	if item == nil {
		return
	}
	source := item.result.InstallSource
	if _, ok := s.selected[source]; ok {
		delete(s.selected, source)
		item.selected = false
	} else {
		s.selected[source] = struct{}{}
		item.selected = true
	}
}

func (s *SkillsSHSearch) toggleDetailsFocused() {
	item := s.focusedItem()
	if item == nil {
		return
	}
	item.showDetails = !item.showDetails
}

func (s *SkillsSHSearch) toggleSelectedInstalledFocused() {
	item := s.focusedInstalledItem()
	if item == nil {
		return
	}
	if _, ok := s.selectedInstalled[item.name]; ok {
		delete(s.selectedInstalled, item.name)
		item.checked = false
		return
	}
	s.selectedInstalled[item.name] = struct{}{}
	item.checked = true
}

func (s *SkillsSHSearch) selectedSources() []string {
	sources := make([]string, 0, len(s.selected))
	for source := range s.selected {
		if source == "" {
			continue
		}
		sources = append(sources, source)
	}
	return sources
}

func (s *SkillsSHSearch) focusedItem() *skillsSHSearchItem {
	idx := s.list.Selected()
	if idx < 0 || idx >= len(s.items) {
		return nil
	}
	return s.items[idx]
}

func (s *SkillsSHSearch) focusedInstalledItem() *skillsInstalledItem {
	idx := s.list.Selected()
	if idx < 0 || idx >= len(s.installed) {
		return nil
	}
	return s.installed[idx]
}

// Draw implements Dialog.
func (s *SkillsSHSearch) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := s.com.Styles
	width := max(0, min(defaultDialogMaxWidth+30, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight+8, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))

	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	s.renderInnerWidth = innerWidth
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	statusHeight := 1
	listHeight := max(1, height-heightOffset-statusHeight)

	s.input.SetWidth(max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-1))
	s.list.SetSize(innerWidth, listHeight)

	rc := NewRenderContext(t, width)
	rc.Title = "Skills Manager"
	rc.TitleInfo = ""

	rc.AddPart(lipgloss.NewStyle().Width(innerWidth).Render(""))
	rc.AddPart(s.tabsView(innerWidth))
	rc.AddPart(lipgloss.NewStyle().Width(innerWidth).Render(""))

	if s.tab == skillsManagerTabSearch {
		rc.AddPart(s.safetyInfoView(innerWidth))
		rc.AddPart(s.inputView(innerWidth))
	}
	rc.AddPart(s.actionsView(innerWidth))
	rc.AddPart(lipgloss.NewStyle().Width(innerWidth).Render(""))
	rc.AddPart(s.listLabelView(innerWidth))

	listView := t.Dialog.List.Height(s.list.Height()).Render(s.list.Render())
	rc.AddPart(listView)

	if details := s.detailsView(innerWidth); details != "" {
		rc.AddPart(details)
	}
	if installErrors := s.installErrorsView(innerWidth); installErrors != "" {
		rc.AddPart(installErrors)
	}
	if !s.loading {
		rc.AddPart(s.statusView(innerWidth))
	}

	helpText := s.help.View(s)
	if s.loading {
		label := strings.TrimSpace(s.statusLine)
		if label == "" {
			label = "Working..."
		}
		helpText = s.spinner.View() + " " + label
	}
	rc.Help = helpText

	view := rc.Render()
	cur := s.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (s *SkillsSHSearch) detailsView(width int) string {
	// Search and installed details are rendered inline in list items.
	return ""
}

func (s *SkillsSHSearch) statusView(width int) string {
	status := strings.TrimSpace(s.statusLine)
	if status == "" {
		if s.tab == skillsManagerTabSearch {
			status = "Type to search skills.sh."
		} else {
			status = "Manage installed skills (enable/disable/delete)."
		}
	}
	status = ansi.Truncate(status, max(0, width), "…")
	return s.com.Styles.Subtle.Width(width).Render(status)
}

func (s *SkillsSHSearch) installErrorsView(width int) string {
	if len(s.lastInstallErrors) == 0 {
		return ""
	}
	lines := []string{
		s.com.Styles.TagError.Render(installErrorsTitle),
	}
	lines = append(lines, s.lastInstallErrors...)
	return s.com.Styles.Dialog.NormalItem.Width(width).Render(strings.Join(lines, "\n"))
}

func (s *SkillsSHSearch) startInstallQueue(sources []string) {
	s.resetInstallState()
	s.lastInstallErrors = nil
	for _, source := range sources {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		s.installQueue = append(s.installQueue, source)
	}
	s.installTotal = len(s.installQueue)
	s.loading = s.installTotal > 0
}

func (s *SkillsSHSearch) popNextInstallSource() string {
	if len(s.installQueue) == 0 {
		return ""
	}
	next := s.installQueue[0]
	s.installQueue = s.installQueue[1:]
	return next
}

func (s *SkillsSHSearch) resetInstallState() {
	s.installQueue = nil
	s.installTotal = 0
	s.installDone = 0
	s.installOK = nil
	s.installFailed = nil
}

// ShortHelp implements help.KeyMap.
func (s *SkillsSHSearch) ShortHelp() []key.Binding {
	if s.tab == skillsManagerTabInstalled {
		return []key.Binding{
			s.keyMap.Next,
			s.keyMap.Previous,
			s.keyMap.Tab,
			s.keyMap.BackTab,
			s.keyMap.ActionPrev,
			s.keyMap.ActionNext,
			s.keyMap.ExecuteAction,
			s.keyMap.Close,
		}
	}
	return []key.Binding{
		s.keyMap.Next,
		s.keyMap.Previous,
		s.keyMap.Tab,
		s.keyMap.BackTab,
		s.keyMap.ActionPrev,
		s.keyMap.ActionNext,
		s.keyMap.ExecuteAction,
		s.keyMap.Close,
	}
}

// FullHelp implements help.KeyMap.
func (s *SkillsSHSearch) FullHelp() [][]key.Binding {
	if s.tab == skillsManagerTabInstalled {
		return [][]key.Binding{
			{s.keyMap.Next, s.keyMap.Previous, s.keyMap.Tab, s.keyMap.BackTab, s.keyMap.TabPrev, s.keyMap.TabNext, s.keyMap.ActionPrev, s.keyMap.ActionNext, s.keyMap.ExecuteAction},
			{s.keyMap.Enable, s.keyMap.Disable, s.keyMap.Delete, s.keyMap.FixPerms, s.keyMap.Update, s.keyMap.Sources, s.keyMap.Refresh, s.keyMap.Close},
		}
	}
	return [][]key.Binding{
		{s.keyMap.Next, s.keyMap.Previous, s.keyMap.Tab, s.keyMap.BackTab, s.keyMap.TabPrev, s.keyMap.TabNext, s.keyMap.ActionPrev, s.keyMap.ActionNext, s.keyMap.ExecuteAction},
		{s.keyMap.Select, s.keyMap.Install, s.keyMap.OpenURL, s.keyMap.Update, s.keyMap.Sources, s.keyMap.Refresh, s.keyMap.Close},
	}
}

func (s *SkillsSHSearch) tabsView(width int) string {
	labels := []string{"Installed", "Search"}
	activeIndex := 0
	if s.tab == skillsManagerTabSearch {
		activeIndex = 1
	}
	parts := make([]string, 0, len(labels))
	for i, label := range labels {
		style := s.com.Styles.HalfMuted.Padding(0, 1)
		if i == activeIndex {
			style = s.com.Styles.TagInfo
		}
		parts = append(parts, style.Render(label))
	}
	label := s.focusLabel(focusLabelView, s.focusArea == skillsFocusTabs)
	return s.com.Styles.Dialog.NormalItem.Width(width).Render(label + " " + strings.Join(parts, " "))
}

func (s *SkillsSHSearch) actionsView(width int) string {
	actions := s.currentActionLabels()
	if len(actions) == 0 {
		return ""
	}
	selected := s.currentActionIndex()
	parts := make([]string, 0, len(actions))
	for i, action := range actions {
		style := s.com.Styles.HalfMuted.Padding(0, 1)
		if i == selected {
			style = s.com.Styles.TagInfo
		}
		parts = append(parts, style.Render(action))
	}
	actionLabel := s.focusLabel(focusLabelAction, s.focusArea == skillsFocusActions)
	return s.com.Styles.Dialog.NormalItem.Width(width).Render(actionLabel + " " + strings.Join(parts, " "))
}

func (s *SkillsSHSearch) safetyInfoView(width int) string {
	return common.CollapsibleInfo(s.com.Styles, common.CollapsibleInfoOpts{
		Label:   s.focusLabel(focusLabelSafety, s.focusArea == skillsFocusSafety),
		Summary: safetySummaryText,
		Details: []string{
			safetyDetailLine1,
			safetyDetailLine2,
			safetyDetailLine3,
		},
		Expanded: s.showSafetyInfo,
		Focused:  s.focusArea == skillsFocusSafety,
	}, width)
}

func (s *SkillsSHSearch) inputView(width int) string {
	return s.com.Styles.Dialog.InputPrompt.Width(width).Render(s.input.View())
}

func (s *SkillsSHSearch) listLabelView(width int) string {
	labelText := focusLabelSkillsSelection
	if s.tab == skillsManagerTabSearch {
		labelText = focusLabelSkillSearchResults
	}
	label := s.focusLabel(labelText, s.focusArea == skillsFocusList)
	return s.com.Styles.Dialog.NormalItem.Width(width).Render(label)
}

func (s *SkillsSHSearch) focusLabel(label string, focused bool) string {
	if focused {
		return lipgloss.NewStyle().Foreground(s.com.Styles.GreenLight).Render(label)
	}
	return s.com.Styles.Subtle.Render(label)
}

func (s *SkillsSHSearch) currentActionLabels() []string {
	if s.tab == skillsManagerTabInstalled {
		return installedActionLabels
	}
	return searchActionLabels
}

func (s *SkillsSHSearch) currentActionIndex() int {
	if s.tab == skillsManagerTabInstalled {
		return s.actionIndexInstalled
	}
	return s.actionIndexSearch
}

func (s *SkillsSHSearch) setCurrentActionIndex(v int) {
	if s.tab == skillsManagerTabInstalled {
		s.actionIndexInstalled = v
		return
	}
	s.actionIndexSearch = v
}

func (s *SkillsSHSearch) shiftAction(delta int) {
	labels := s.currentActionLabels()
	if len(labels) == 0 {
		return
	}
	idx := s.currentActionIndex()
	idx = (idx + delta + len(labels)) % len(labels)
	s.setCurrentActionIndex(idx)
}

func (s *SkillsSHSearch) executeSelectedAction() Action {
	switch s.tab {
	case skillsManagerTabInstalled:
		switch s.actionIndexInstalled {
		case installedActionEnable:
			names := s.selectedInstalledNames()
			if len(names) == 0 {
				s.statusLine = "Select at least one installed skill first."
				return nil
			}
			return ActionSkillsSetDisabledBatch{Names: names, Disabled: false}
		case installedActionDisable:
			names := s.selectedInstalledNames()
			if len(names) == 0 {
				s.statusLine = "Select at least one installed skill first."
				return nil
			}
			return ActionSkillsSetDisabledBatch{Names: names, Disabled: true}
		case installedActionFixPerms:
			names := s.selectedInstalledNames()
			if len(names) == 0 {
				s.statusLine = "Select at least one installed skill first."
				return nil
			}
			return ActionSkillsFixPerms{Names: names}
		case installedActionDelete:
			names := s.selectedInstalledNames()
			if len(names) == 0 {
				s.statusLine = "Select at least one installed skill first."
				return nil
			}
			return ActionOpenConfirm{
				Title:   "Delete Skills",
				Message: fmt.Sprintf("Delete %d selected skill(s)?", len(names)),
				Payload: ActionSkillsDeleteBatch{Names: names},
			}
		case installedActionRefresh:
			return ActionSkillsInstalledRefreshRequest{}
		case installedActionSources:
			return ActionSkillsSHSources{}
		}
	case skillsManagerTabSearch:
		switch s.actionIndexSearch {
		case searchActionInstall:
			sources := s.selectedSources()
			if len(sources) == 0 {
				s.statusLine = "Select at least one skill first."
				return nil
			}
			s.startInstallQueue(sources)
			next := s.popNextInstallSource()
			if next == "" {
				s.loading = false
				s.statusLine = "No selected skills to install."
				return nil
			}
			s.statusLine = fmt.Sprintf("Installing 1/%d: %s", s.installTotal, next)
			return ActionSkillsSHInstallSource{Source: next}
		case searchActionOpenLink:
			item := s.focusedItem()
			if item == nil || strings.TrimSpace(item.result.DetailsURL) == "" {
				return nil
			}
			return ActionSkillsSHOpenDetails{URL: item.result.DetailsURL}
		case searchActionRefresh:
			s.requestSeq++
			s.activeRequestID = s.requestSeq
			return s.requestSearch(s.activeRequestID)
		case searchActionSources:
			return ActionSkillsSHSources{}
		}
	}
	return nil
}

type skillsSHSearchItem struct {
	t *styles.Styles

	result      SkillsSHSearchResult
	selected    bool
	focused     bool
	installed   bool
	showDetails bool
}

var _ list.Item = (*skillsSHSearchItem)(nil)

func (i *skillsSHSearchItem) Render(width int) string {
	checkbox := "[ ]"
	if i.selected {
		checkbox = "[x]"
	}

	infoParts := []string{i.result.Source}
	if i.result.Installs > 0 {
		infoParts = append(infoParts, fmt.Sprintf("%d installs", i.result.Installs))
	}
	if i.installed {
		infoParts = append(infoParts, "installed")
	}

	info := strings.Join(infoParts, " • ")
	titleWidth := max(0, width-lipgloss.Width(checkbox)-1)
	title := ansi.Truncate(i.result.Name, titleWidth, "…")
	line := checkbox + " " + title
	if info != "" {
		infoStyle := i.t.Subtle
		if i.focused {
			infoStyle = i.t.Base.Foreground(charmtone.Pepper)
		}
		line += "\n" + infoStyle.Width(width).Render(ansi.Truncate(info, width, "…"))
	}
	if i.showDetails || i.focused {
		state := "Not installed"
		if i.installed {
			state = "Installed"
		}
		detailsURL := strings.TrimSpace(i.result.DetailsURL)
		if detailsURL == "" {
			detailsURL = "unavailable"
		}
		detailLines := []string{
			fmt.Sprintf("Name: %s", i.result.Name),
			fmt.Sprintf("Source: %s", i.result.Source),
			fmt.Sprintf("Install: %s", i.result.InstallSource),
			fmt.Sprintf("Details: %s", detailsURL),
			fmt.Sprintf("State: %s", state),
		}
		indented := make([]string, 0, len(detailLines))
		for _, detailLine := range detailLines {
			indented = append(indented, "  │ "+ansi.Truncate(detailLine, max(0, width-4), "…"))
		}
		line += "\n\n" + strings.Join(indented, "\n")
	}

	style := i.t.Dialog.NormalItem.Width(width).MarginBottom(1)
	if i.focused {
		style = i.t.Dialog.SelectedItem.Width(width).MarginBottom(1)
	}
	return style.Render(line)
}

type skillsInstalledItem struct {
	t *styles.Styles

	name               string
	description        string
	path               string
	skillFile          string
	origin             string
	disabled           bool
	blocked            bool
	blockReasons       []string
	permissionWarnings []string
	checked            bool
	focused            bool
	showDetails        bool
}

var _ list.Item = (*skillsInstalledItem)(nil)

func (i *skillsInstalledItem) Render(width int) string {
	checkbox := "[ ]"
	if i.checked {
		checkbox = "[x]"
	}
	state := "enabled"
	stateStyle := i.t.TagInfo
	if i.disabled {
		state = "disabled"
		stateStyle = i.t.HalfMuted
	}
	if i.blocked {
		state = "blocked"
		stateStyle = i.t.TagError
	}
	status := stateStyle.Render(state)
	check := i.t.TagInfo.Render("checked ok")
	if i.blocked {
		check = i.t.TagError.Render("blocked")
	} else if len(i.permissionWarnings) > 0 {
		check = i.t.TagWarn.Render("perm warning")
	}
	title := ansi.Truncate(i.name, max(0, width-lipgloss.Width(status)-lipgloss.Width(check)-lipgloss.Width(checkbox)-3), "…")
	line := checkbox + " " + title + " " + status + " " + check

	infoParts := []string{}
	if strings.TrimSpace(i.description) != "" {
		infoParts = append(infoParts, ansi.Truncate(i.description, width, "…"))
	}
	if len(infoParts) > 0 {
		rendered := make([]string, 0, len(infoParts))
		infoStyle := i.t.Subtle
		if i.focused {
			infoStyle = i.t.Base.Foreground(charmtone.Pepper)
		}
		for _, p := range infoParts {
			rendered = append(rendered, infoStyle.Width(width).Render(ansi.Truncate(p, width, "…")))
		}
		line += "\n" + strings.Join(rendered, "\n")
	}
	if i.showDetails || i.focused {
		origin := i.origin
		if strings.TrimSpace(origin) == "" {
			origin = "(unknown)"
		}
		state := "Enabled"
		if i.disabled {
			state = "Disabled"
		}
		if i.blocked {
			state = "Blocked"
		}
		detailLines := []string{
			fmt.Sprintf("Name: %s", i.name),
			fmt.Sprintf("Path: %s", i.path),
			fmt.Sprintf("Skill File: %s", i.skillFile),
			fmt.Sprintf("Origin: %s", origin),
			fmt.Sprintf("State: %s", state),
		}
		if i.blocked && len(i.blockReasons) > 0 {
			detailLines = append(detailLines, "Blocked by: "+strings.Join(i.blockReasons, ", "))
		}
		if len(i.permissionWarnings) > 0 {
			detailLines = append(detailLines, "Permission warnings:")
			for _, warning := range i.permissionWarnings {
				detailLines = append(detailLines, "- "+warning)
			}
		}
		// Keep details inline without nested box styles so selected-row
		// background remains consistent.
		indented := make([]string, 0, len(detailLines))
		for _, detailLine := range detailLines {
			indented = append(indented, "  │ "+ansi.Truncate(detailLine, max(0, width-4), "…"))
		}
		line += "\n\n" + strings.Join(indented, "\n")
	}

	style := i.t.Dialog.NormalItem.Width(width).MarginBottom(1)
	if i.focused {
		style = i.t.Dialog.SelectedItem.Width(width).MarginBottom(1)
	}
	return style.Render(line)
}

func isSkillsSourceTracked(installSource string, tracked map[string]struct{}) bool {
	installSource = strings.TrimSpace(installSource)
	if installSource == "" {
		return false
	}
	if _, ok := tracked[installSource]; ok {
		return true
	}

	sourceParts := strings.Split(installSource, "/")
	for trackedSource := range tracked {
		trackedParts := strings.Split(trackedSource, "/")
		if len(trackedParts) == 3 && strings.HasPrefix(installSource, trackedSource+"/") {
			return true
		}
		if len(sourceParts) == 3 && strings.HasPrefix(trackedSource, installSource+"/") {
			return true
		}
	}
	return false
}

func firstInstallFailure(failed map[string]string) (string, string) {
	if len(failed) == 0 {
		return "", ""
	}
	sources := make([]string, 0, len(failed))
	for source := range failed {
		sources = append(sources, source)
	}
	slices.Sort(sources)
	source := sources[0]
	return source, failed[source]
}

func (s *SkillsSHSearch) selectedInstalledNames() []string {
	names := make([]string, 0, len(s.selectedInstalled))
	for name := range s.selectedInstalled {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func renderInstallFailures(failed map[string]string) []string {
	if len(failed) == 0 {
		return nil
	}
	sources := make([]string, 0, len(failed))
	for source := range failed {
		sources = append(sources, source)
	}
	slices.Sort(sources)

	const maxShown = 3
	lines := make([]string, 0, maxShown*2+1)
	for idx, source := range sources {
		if idx >= maxShown {
			break
		}
		msg := summarizeInstallError(failed[source], 160)
		lines = append(lines, fmt.Sprintf("%d. %s", idx+1, source))
		lines = append(lines, "   "+msg)
	}
	if len(sources) > maxShown {
		lines = append(lines, fmt.Sprintf("... and %d more errors", len(sources)-maxShown))
	}
	return lines
}

func summarizeInstallError(raw string, limit int) string {
	msg := strings.TrimSpace(raw)
	msg = strings.ReplaceAll(msg, "\n", " ")
	msg = strings.Join(strings.Fields(msg), " ")
	msg = strings.TrimPrefix(msg, exitStatusPrefix)
	msg = strings.TrimSpace(msg)
	if msg == "" {
		msg = "Unknown error"
	}
	return ansi.Truncate(msg, max(32, limit), "…")
}
