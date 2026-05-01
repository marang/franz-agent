package model

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/marang/franz-agent/internal/config"
	"github.com/marang/franz-agent/internal/csync"
	"github.com/marang/franz-agent/internal/fsext"
	"github.com/marang/franz-agent/internal/lsp"
	"github.com/marang/franz-agent/internal/session"
	"github.com/marang/franz-agent/internal/ui/common"
	"github.com/marang/franz-agent/internal/ui/styles"
)

const (
	headerDiag   = "╱"
	leftPadding  = 1
	rightPadding = 1
)

type header struct {
	// cached logo and compact logo
	logo               string
	compactLogo        string
	compactAttribution string

	com     *common.Common
	width   int
	compact bool
}

// newHeader creates a new header model.
func newHeader(com *common.Common) *header {
	h := &header{
		com: com,
	}
	t := com.Styles
	h.compactLogo = styles.ApplyBoldForegroundGrad(t, "┏━╸┏━┓┏━┓┏┓╻╺━┓\n┣╸ ┣┳┛┣━┫┃┗┫┏━┛\n╹  ╹┗╸╹ ╹╹ ╹┗━╸", t.LogoTitleColorA, t.LogoTitleColorB)
	h.compactAttribution = t.HalfMuted.Render("Based on Crush")
	return h
}

// drawHeader draws the header for the given session.
func (h *header) drawHeader(
	scr uv.Screen,
	area uv.Rectangle,
	session *session.Session,
	compact bool,
	detailsOpen bool,
	width int,
) {
	t := h.com.Styles
	if width != h.width || compact != h.compact {
		h.logo = renderLogo(h.com.Styles, compact, width)
	}

	h.width = width
	h.compact = compact

	if !compact || session == nil || h.com.App == nil {
		uv.NewStyledString(h.logo).Draw(scr, area)
		return
	}

	if session.ID == "" {
		return
	}

	headerWidth := max(0, width-leftPadding-rightPadding)
	logoLines := strings.Split(h.compactLogo, "\n")
	logoWidth := 0
	for _, line := range logoLines {
		logoWidth = max(logoWidth, lipgloss.Width(line))
	}

	renderedLines := make([]string, 0, len(logoLines)+1)
	for _, line := range logoLines {
		filled := line
		if rem := max(0, headerWidth-logoWidth-1); rem > 0 {
			filled += " " + t.Header.Diagonals.Render(strings.Repeat(headerDiag, rem))
		}
		renderedLines = append(renderedLines, ansi.Truncate(filled, headerWidth, ""))
	}

	details := renderHeaderDetails(
		h.com,
		session,
		h.com.App.LSPManager.Clients(),
		detailsOpen,
		headerWidth,
	)
	detailsLine := ansi.Truncate(details+" "+h.compactAttribution, headerWidth, "…")
	renderedLines = append(renderedLines, detailsLine)

	view := uv.NewStyledString(
		t.Base.Padding(0, rightPadding, 0, leftPadding).Render(strings.Join(renderedLines, "\n")),
	)
	view.Draw(scr, area)
}

// renderHeaderDetails renders the details section of the header.
func renderHeaderDetails(
	com *common.Common,
	session *session.Session,
	lspClients *csync.Map[string, *lsp.Client],
	detailsOpen bool,
	availWidth int,
) string {
	t := com.Styles

	var parts []string

	errorCount := 0
	for l := range lspClients.Seq() {
		errorCount += l.GetDiagnosticCounts().Error
	}

	if errorCount > 0 {
		parts = append(parts, t.LSP.ErrorDiagnostic.Render(fmt.Sprintf("%s%d", styles.LSPErrorIcon, errorCount)))
	}

	agentCfg := com.Config().Agents[config.AgentCoder]
	model := com.Config().GetModelByType(agentCfg.Model)
	percentage := (float64(session.CompletionTokens+session.PromptTokens) / float64(model.ContextWindow)) * 100
	formattedPercentage := t.Header.Percentage.Render(fmt.Sprintf("%d%%", int(percentage)))
	parts = append(parts, formattedPercentage)

	const keystroke = "ctrl+d"
	if detailsOpen {
		parts = append(parts, t.Header.Keystroke.Render(keystroke)+t.Header.KeystrokeTip.Render(" close"))
	} else {
		parts = append(parts, t.Header.Keystroke.Render(keystroke)+t.Header.KeystrokeTip.Render(" open "))
	}

	dot := t.Header.Separator.Render(" • ")
	metadata := strings.Join(parts, dot)
	metadata = dot + metadata

	const dirTrimLimit = 4
	cwd := fsext.DirTrim(fsext.PrettyPath(com.Store().WorkingDir()), dirTrimLimit)
	cwd = t.Header.WorkingDir.Render(cwd)

	result := cwd + metadata
	return ansi.Truncate(result, max(0, availWidth), "…")
}
