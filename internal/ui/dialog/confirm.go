package dialog

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/marang/franz-agent/internal/ui/common"
)

// ConfirmID is the identifier for the generic confirm dialog.
const ConfirmID = "confirm"

// Confirm represents a reusable yes/no confirmation dialog.
type Confirm struct {
	com   *common.Common
	title string
	text  string

	selectedNo bool
	payload    any

	keyMap struct {
		LeftRight,
		EnterSpace,
		Yes,
		No,
		Tab,
		Close key.Binding
	}
}

var _ Dialog = (*Confirm)(nil)

// NewConfirm creates a generic yes/no dialog.
func NewConfirm(com *common.Common, title, text string, payload any) *Confirm {
	c := &Confirm{
		com:        com,
		title:      title,
		text:       text,
		selectedNo: true,
		payload:    payload,
	}

	c.keyMap.LeftRight = key.NewBinding(
		key.WithKeys("left", "right"),
		key.WithHelp("←/→", "switch options"),
	)
	c.keyMap.EnterSpace = key.NewBinding(
		key.WithKeys("enter", " "),
		key.WithHelp("enter/space", "confirm"),
	)
	c.keyMap.Yes = key.NewBinding(
		key.WithKeys("y", "Y"),
		key.WithHelp("y", "yes"),
	)
	c.keyMap.No = key.NewBinding(
		key.WithKeys("n", "N"),
		key.WithHelp("n", "no"),
	)
	c.keyMap.Tab = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch options"),
	)
	c.keyMap.Close = CloseKey

	return c
}

// ID implements Dialog.
func (*Confirm) ID() string {
	return ConfirmID
}

// HandleMsg implements Dialog.
func (c *Confirm) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, c.keyMap.Close), key.Matches(msg, c.keyMap.No):
			return ActionClose{}
		case key.Matches(msg, c.keyMap.LeftRight, c.keyMap.Tab):
			c.selectedNo = !c.selectedNo
			return nil
		case key.Matches(msg, c.keyMap.Yes):
			return ActionConfirmChoice{Confirmed: true, Payload: c.payload}
		case key.Matches(msg, c.keyMap.EnterSpace):
			if c.selectedNo {
				return ActionClose{}
			}
			return ActionConfirmChoice{Confirmed: true, Payload: c.payload}
		}
	}
	return nil
}

// Draw implements Dialog.
func (c *Confirm) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	buttonOpts := []common.ButtonOpts{
		{Text: "No", Selected: c.selectedNo, Padding: 3},
		{Text: "Yes", Selected: !c.selectedNo, Padding: 3},
	}
	buttons := common.ButtonGroup(c.com.Styles, buttonOpts, " ")

	rc := NewRenderContext(c.com.Styles, defaultDialogMaxWidth)
	rc.Title = c.title
	rc.AddPart(c.com.Styles.Base.Render(c.text))
	rc.AddPart("")
	rc.AddPart(lipgloss.NewStyle().Width(defaultDialogMaxWidth - c.com.Styles.Dialog.View.GetHorizontalFrameSize()).AlignHorizontal(lipgloss.Center).Render(buttons))
	rc.Help = c.com.Styles.Subtle.Render("←/→ switch options • enter confirm • esc cancel")

	view := rc.Render()
	DrawCenter(scr, area, view)
	return nil
}

// ShortHelp implements help.KeyMap.
func (c *Confirm) ShortHelp() []key.Binding {
	return []key.Binding{c.keyMap.LeftRight, c.keyMap.EnterSpace, c.keyMap.Close}
}

// FullHelp implements help.KeyMap.
func (c *Confirm) FullHelp() [][]key.Binding {
	return [][]key.Binding{{c.keyMap.LeftRight, c.keyMap.EnterSpace, c.keyMap.Yes, c.keyMap.No, c.keyMap.Tab, c.keyMap.Close}}
}
