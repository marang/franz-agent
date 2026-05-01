package model

import (
	"image"
	"strconv"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textarea"
	"github.com/marang/franz-agent/internal/ui/chat"
	"github.com/marang/franz-agent/internal/ui/common"
)

// testMessageItem is a minimal chat item used to populate the chat list
// without pulling in full message rendering machinery.
type testMessageItem struct {
	id   string
	text string
}

func (m testMessageItem) ID() string           { return m.id }
func (m testMessageItem) Render(int) string    { return m.text }
func (m testMessageItem) RawRender(int) string { return m.text }

var _ chat.MessageItem = testMessageItem{}

// newTestUI builds a focused uiChat model with dynamic textarea sizing enabled.
// It intentionally keeps dependencies minimal so layout behavior can be tested
// in isolation.
func newTestUI() *UI {
	com := common.DefaultCommon(nil)

	ta := textarea.New()
	ta.SetStyles(com.Styles.TextArea)
	ta.ShowLineNumbers = false
	ta.CharLimit = -1
	ta.SetVirtualCursor(false)
	ta.DynamicHeight = true
	ta.MinHeight = TextareaMinHeight
	ta.MaxHeight = TextareaMaxHeight
	ta.Focus()

	u := &UI{
		com:      com,
		status:   NewStatus(com, nil),
		chat:     NewChat(com),
		textarea: ta,
		state:    uiChat,
		focus:    uiFocusEditor,
		width:    140,
		height:   45,
	}

	return u
}

func TestUpdateLayoutAndSize_EditorGrowthShrinksChat(t *testing.T) {
	t.Parallel()

	// Baseline layout at min textarea height.
	u := newTestUI()
	u.updateLayoutAndSize()

	initialEditorHeight := u.layout.editor.Dy()
	initialChatHeight := u.layout.main.Dy()

	// Increase textarea content enough to trigger growth, then run the
	// same resize hook used in the real update path.
	prevHeight := u.textarea.Height()
	u.textarea.SetValue(strings.Repeat("line\n", 8))
	u.textarea.MoveToEnd()
	_ = u.handleTextareaHeightChange(prevHeight)

	if got := u.layout.editor.Dy(); got <= initialEditorHeight {
		t.Fatalf("expected editor to grow: got %d, want > %d", got, initialEditorHeight)
	}

	if got := u.layout.main.Dy(); got >= initialChatHeight {
		t.Fatalf("expected chat to shrink: got %d, want < %d", got, initialChatHeight)
	}
}

func TestHandleTextareaHeightChange_FollowModeStaysAtBottom(t *testing.T) {
	t.Parallel()

	// Use enough messages to make the chat scrollable so AtBottom/Follow
	// assertions are meaningful.
	u := newTestUI()

	msgs := make([]chat.MessageItem, 0, 60)
	for i := range 60 {
		msgs = append(msgs, testMessageItem{
			id:   "m-" + strconv.Itoa(i),
			text: "message " + strconv.Itoa(i),
		})
	}
	u.chat.SetMessages(msgs...)
	u.updateLayoutAndSize()

	// Enter follow mode and verify we're anchored at the bottom first.
	u.chat.ScrollToBottom()
	if !u.chat.AtBottom() {
		t.Fatal("expected chat to start at bottom")
	}

	// Grow the editor; follow mode should keep the chat pinned to the end
	// even as the chat viewport shrinks.
	prevHeight := u.textarea.Height()
	u.textarea.SetValue(strings.Repeat("line\n", 10))
	u.textarea.MoveToEnd()
	_ = u.handleTextareaHeightChange(prevHeight)

	if !u.chat.Follow() {
		t.Fatal("expected follow mode to remain enabled")
	}
	if !u.chat.AtBottom() {
		t.Fatal("expected chat to remain at bottom after editor resize in follow mode")
	}
}

func TestGenerateLayout_NarrowTerminalUsesValidCompactRects(t *testing.T) {
	t.Parallel()

	for _, size := range []struct {
		width  int
		height int
	}{
		{width: 80, height: 24},
		{width: 60, height: 16},
		{width: 40, height: 10},
	} {
		t.Run(strconv.Itoa(size.width)+"x"+strconv.Itoa(size.height), func(t *testing.T) {
			t.Parallel()

			u := newTestUI()
			layout := u.generateLayout(size.width, size.height)
			area := image.Rect(0, 0, size.width, size.height)

			requireValidRectInArea(t, layout.area, area)
			requireValidRectInArea(t, layout.status, area)
			requireValidRectInArea(t, layout.header, area)
			requireValidRectInArea(t, layout.main, area)
			requireValidRectInArea(t, layout.editor, area)
			requireValidRectInArea(t, layout.pills, area)
			requireValidRectInArea(t, layout.sessionDetails, area)
			requireValidRectInArea(t, layout.sidebar, area)

			if size.width < sidebarWidth+minSidebarMainWidth {
				if layout.sidebar.Dx() != 0 {
					t.Fatalf("expected narrow layout to omit sidebar, got width %d", layout.sidebar.Dx())
				}
				if layout.header.Dy() == 0 {
					t.Fatal("expected compact header in narrow layout")
				}
			}
		})
	}
}

func requireValidRectInArea(t *testing.T, rect, area image.Rectangle) {
	t.Helper()

	if rect.Dx() < 0 || rect.Dy() < 0 {
		t.Fatalf("invalid rectangle %v", rect)
	}
	if rect.Min.X < area.Min.X || rect.Max.X > area.Max.X || rect.Min.Y < area.Min.Y || rect.Max.Y > area.Max.Y {
		t.Fatalf("rectangle %v is outside area %v", rect, area)
	}
}
