package model

import (
	"image"
	"slices"

	"charm.land/bubbles/v2/help"
)

const (
	sidebarWidth        = 30
	minSidebarMainWidth = 40
	landingHeaderHeight = 7
	compactHeaderHeight = 4
)

// generateLayout calculates the layout rectangles for all UI components based
// on the current UI state and terminal dimensions.
func (m *UI) generateLayout(w, h int) uiLayout {
	area := image.Rect(0, 0, w, h)
	helpHeight := 1
	editorHeight := m.textarea.Height() + editorHeightMargin

	var helpKeyMap help.KeyMap = m
	if m.status != nil && m.status.ShowingAll() {
		for _, row := range helpKeyMap.FullHelp() {
			helpHeight = max(helpHeight, len(row))
		}
	}
	helpHeight = clampInt(helpHeight, 0, area.Dy())

	appRect, helpRect := splitVerticalFromBottom(area, helpHeight)
	appRect = insetRect(appRect, 1, 1, 1, 1)
	helpRect.Min.Y = max(area.Min.Y, helpRect.Min.Y-1)

	if slices.Contains([]uiState{uiOnboarding, uiInitialize, uiLanding}, m.state) {
		appRect = insetRect(appRect, 1, 0, 1, 0)
	}

	uiLayout := uiLayout{
		area:   area,
		status: helpRect,
	}

	switch m.state {
	case uiOnboarding, uiInitialize:
		uiLayout.header, uiLayout.main = splitVerticalFromTop(appRect, landingHeaderHeight)
	case uiLanding:
		headerRect, mainRect := splitVerticalFromTop(appRect, landingHeaderHeight)
		mainRect, editorRect := splitVerticalFromBottom(mainRect, editorHeight)
		editorRect.Min.X -= 1
		editorRect.Max.X += 1
		editorRect = normalizeRect(editorRect)
		uiLayout.header = headerRect
		uiLayout.main = mainRect
		uiLayout.editor = editorRect
	case uiChat:
		if m.useCompactLayout(appRect.Dx()) {
			m.applyCompactChatLayout(&uiLayout, appRect, area, editorHeight)
		} else {
			m.applySidebarChatLayout(&uiLayout, appRect, sidebarWidth, editorHeight)
		}
	}

	return uiLayout
}

func (m *UI) applyCompactChatLayout(uiLayout *uiLayout, appRect image.Rectangle, area image.Rectangle, editorHeight int) {
	headerRect, mainRect := splitVerticalFromTop(appRect, compactHeaderHeight)
	detailsHeight := min(sessionDetailsMaxHeight, area.Dy()-1)
	sessionDetailsArea, _ := splitVerticalFromTop(appRect, detailsHeight)
	uiLayout.sessionDetails = sessionDetailsArea
	uiLayout.sessionDetails.Min.Y = min(uiLayout.sessionDetails.Max.Y, uiLayout.sessionDetails.Min.Y+compactHeaderHeight)
	mainRect.Min.Y = min(mainRect.Max.Y, mainRect.Min.Y+1)
	mainRect, editorRect := splitVerticalFromBottom(mainRect, editorHeight)
	mainRect.Max.X = max(mainRect.Min.X, mainRect.Max.X-1)
	uiLayout.header = headerRect
	m.applyPillsLayout(uiLayout, mainRect)
	uiLayout.main.Max.Y = max(uiLayout.main.Min.Y, uiLayout.main.Max.Y-1)
	uiLayout.editor = editorRect
}

func (m *UI) applySidebarChatLayout(uiLayout *uiLayout, appRect image.Rectangle, sidebarWidth int, editorHeight int) {
	mainRect, sideRect := splitHorizontalFromRight(appRect, sidebarWidth)
	sideRect.Min.X = min(sideRect.Max.X, sideRect.Min.X+1)
	mainRect, editorRect := splitVerticalFromBottom(mainRect, editorHeight)
	mainRect.Max.X = max(mainRect.Min.X, mainRect.Max.X-1)
	uiLayout.sidebar = sideRect
	m.applyPillsLayout(uiLayout, mainRect)
	uiLayout.main.Max.Y = max(uiLayout.main.Min.Y, uiLayout.main.Max.Y-1)
	uiLayout.editor = editorRect
}

func (m *UI) applyPillsLayout(uiLayout *uiLayout, mainRect image.Rectangle) {
	pillsHeight := m.pillsAreaHeight()
	if pillsHeight <= 0 {
		uiLayout.main = mainRect
		return
	}
	pillsHeight = min(pillsHeight, mainRect.Dy())
	uiLayout.main, uiLayout.pills = splitVerticalFromBottom(mainRect, pillsHeight)
}

func (m *UI) useCompactLayout(width int) bool {
	return m.isCompact || width < sidebarWidth+minSidebarMainWidth
}

func splitVerticalFromTop(area image.Rectangle, topHeight int) (image.Rectangle, image.Rectangle) {
	area = normalizeRect(area)
	topHeight = clampInt(topHeight, 0, area.Dy())
	top := image.Rect(area.Min.X, area.Min.Y, area.Max.X, area.Min.Y+topHeight)
	bottom := image.Rect(area.Min.X, top.Max.Y, area.Max.X, area.Max.Y)
	return top, bottom
}

func splitVerticalFromBottom(area image.Rectangle, bottomHeight int) (image.Rectangle, image.Rectangle) {
	area = normalizeRect(area)
	bottomHeight = clampInt(bottomHeight, 0, area.Dy())
	bottom := image.Rect(area.Min.X, area.Max.Y-bottomHeight, area.Max.X, area.Max.Y)
	top := image.Rect(area.Min.X, area.Min.Y, area.Max.X, bottom.Min.Y)
	return top, bottom
}

func splitHorizontalFromRight(area image.Rectangle, rightWidth int) (image.Rectangle, image.Rectangle) {
	area = normalizeRect(area)
	rightWidth = clampInt(rightWidth, 0, area.Dx())
	right := image.Rect(area.Max.X-rightWidth, area.Min.Y, area.Max.X, area.Max.Y)
	left := image.Rect(area.Min.X, area.Min.Y, right.Min.X, area.Max.Y)
	return left, right
}

func insetRect(area image.Rectangle, left, top, right, bottom int) image.Rectangle {
	area = normalizeRect(area)
	area.Min.X += left
	area.Min.Y += top
	area.Max.X -= right
	area.Max.Y -= bottom
	return normalizeRect(area)
}

func normalizeRect(area image.Rectangle) image.Rectangle {
	if area.Max.X < area.Min.X {
		area.Max.X = area.Min.X
	}
	if area.Max.Y < area.Min.Y {
		area.Max.Y = area.Min.Y
	}
	return area
}

func clampInt(value, minimum, maximum int) int {
	if maximum < minimum {
		return minimum
	}
	return min(max(value, minimum), maximum)
}
