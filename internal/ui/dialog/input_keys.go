package dialog

import tea "charm.land/bubbletea/v2"

func isCtrlBackspace(msg tea.KeyPressMsg) bool {
	return msg.String() == "ctrl+backspace"
}
