package attachments

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/marang/franz-agent/internal/message"
)

func TestRendererRenderNarrowWidthDoesNotOverflow(t *testing.T) {
	t.Parallel()

	renderer := NewRenderer(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	rendered := renderer.Render([]message.Attachment{
		{FileName: "first-long-file-name.txt"},
		{FileName: "second-long-file-name.txt"},
		{FileName: "third-long-file-name.txt"},
	}, false, 6)

	if got := lipgloss.Width(rendered); got > maxFilename {
		t.Fatalf("expected narrow attachment row to stay bounded, got width %d: %q", got, rendered)
	}
	if rendered == "" {
		t.Fatal("expected collapsed attachment count")
	}
}

func TestRendererRenderZeroWidthIsEmpty(t *testing.T) {
	t.Parallel()

	renderer := NewRenderer(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	rendered := renderer.Render([]message.Attachment{{FileName: "file.txt"}}, false, 0)

	if rendered != "" {
		t.Fatalf("expected empty render for zero width, got %q", rendered)
	}
}
