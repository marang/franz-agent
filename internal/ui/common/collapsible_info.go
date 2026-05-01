package common

import (
	"strings"

	"github.com/marang/franz-agent/internal/ui/styles"
)

// CollapsibleInfoOpts configures a reusable collapsible info block.
type CollapsibleInfoOpts struct {
	Label    string
	Summary  string
	Details  []string
	Expanded bool
	Focused  bool
}

// CollapsibleInfo renders a generic collapsible info section with a focusable
// [+]/[-] indicator.
func CollapsibleInfo(t *styles.Styles, opts CollapsibleInfoOpts, width int) string {
	prefix := "[+]"
	if opts.Expanded {
		prefix = "[-]"
	}
	if opts.Focused {
		prefix = t.TagInfo.Render(prefix)
	} else {
		prefix = t.HalfMuted.Render(prefix)
	}

	lines := []string{
		opts.Label + " " + prefix + " " + opts.Summary,
	}
	if opts.Expanded {
		for _, detail := range opts.Details {
			detail = strings.TrimSpace(detail)
			if detail == "" {
				continue
			}
			lines = append(lines, "  "+detail)
		}
	}

	return t.Dialog.NormalItem.Width(width).Render(strings.Join(lines, "\n"))
}
