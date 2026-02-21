package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// 80s Miami / Synthwave color palette
var (
	hotPink      = lipgloss.Color("#FF6AC1")
	cyan         = lipgloss.Color("#00E5FF")
	purple       = lipgloss.Color("#B388FF")
	coral        = lipgloss.Color("#FF8A80")
	teal         = lipgloss.Color("#64FFDA")
	sunsetOrange = lipgloss.Color("#FFAB40")
	darkBg       = lipgloss.Color("#1A1A2E")
	softWhite    = lipgloss.Color("#E0E0E0")
	dimmed       = lipgloss.Color("#666666")
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(hotPink).
			Background(darkBg).
			MarginBottom(1)

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cyan).
			Padding(1, 2).
			Background(darkBg)

	labelStyle = lipgloss.NewStyle().
			Foreground(cyan).
			Background(darkBg).
			Bold(true)

	transcriptStyle = lipgloss.NewStyle().
			Foreground(purple).
			Background(darkBg).
			Italic(true)

	hotkeyStyle = lipgloss.NewStyle().
			Foreground(cyan).
			Background(darkBg)

	quitStyle = lipgloss.NewStyle().
			Foreground(dimmed).
			Background(darkBg)

	idleBadge = lipgloss.NewStyle().
			Foreground(teal).
			Background(darkBg).
			Bold(true)

	recordingBadge = lipgloss.NewStyle().
			Foreground(hotPink).
			Background(darkBg).
			Bold(true)

	transcribingBadge = lipgloss.NewStyle().
				Foreground(sunsetOrange).
				Background(darkBg).
				Bold(true)

	errorBadge = lipgloss.NewStyle().
			Foreground(coral).
			Background(darkBg).
			Bold(true)

	bodyStyle = lipgloss.NewStyle().
			Foreground(softWhite).
			Background(darkBg)

	debugTitleStyle = lipgloss.NewStyle().
			Foreground(dimmed).
			Background(darkBg).
			Bold(true)

	debugRuleStyle = lipgloss.NewStyle().
			Foreground(dimmed).
			Background(darkBg)

	debugHeaderStyle = lipgloss.NewStyle().
				Foreground(dimmed).
				Background(darkBg).
				Bold(true)

	debugTimeStyle = lipgloss.NewStyle().
			Foreground(dimmed).
			Background(darkBg)

	debugCategoryStyle = lipgloss.NewStyle().
				Foreground(sunsetOrange).
				Background(darkBg)

	debugMsgStyle = lipgloss.NewStyle().
			Foreground(dimmed).
			Background(darkBg)

	debugSepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#444444")).
			Background(darkBg)

	visualizerStyle = lipgloss.NewStyle().
			Foreground(hotPink).
			Background(darkBg)

	visualizerLabelStyle = lipgloss.NewStyle().
				Foreground(dimmed).
				Background(darkBg)

	statusOkStyle = lipgloss.NewStyle().
			Foreground(teal).
			Background(darkBg).
			Bold(true)

	statusBadStyle = lipgloss.NewStyle().
			Foreground(coral).
			Background(darkBg).
			Bold(true)
)

// panelWidth is the total outer width of the main panel.
// borderStyle has: border (1+1) = 2, padding (2+2) = 4, total chrome = 6.
// Width() in lipgloss sets width including padding but excluding border.
// So we pass panelWidth - 2 (border) to Width(), and the actual text area
// is panelWidth - 6 (border + padding).
const panelWidth = 80
const panelWidthForStyle = panelWidth - 2  // passed to borderStyle.Width()
const panelContentWidth = panelWidth - 6   // actual usable text area

// View renders the TUI.
func (m Model) View() string {
	var b strings.Builder

	// Title — centered with color bars extending to panel edges
	titleText := "  PALAVER  "
	barTotal := panelContentWidth - len(titleText)
	barLeft := barTotal / 2
	barRight := barTotal - barLeft
	title := strings.Repeat("▓", barLeft) + titleText + strings.Repeat("▓", barRight)
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n\n")

	// Status / Visualizer
	b.WriteString(labelStyle.Render("Status:  "))
	b.WriteString(m.renderBadge())
	if m.State == StateRecording {
		b.WriteString(bodyStyle.Render("  "))
		b.WriteString(m.renderVisualizer())
	}
	b.WriteString("\n\n")

	// Last transcription (word-wrapped)
	b.WriteString(labelStyle.Render("Last transcription:"))
	b.WriteString("\n")
	if m.LastTranscript != "" {
		wrapped := transcriptStyle.Width(panelContentWidth).Render(fmt.Sprintf("%q", m.LastTranscript))
		b.WriteString(wrapped)
	} else {
		b.WriteString(bodyStyle.Render("(none yet)"))
	}
	b.WriteString("\n\n")

	// Hotkey info
	keyName := strings.TrimPrefix(m.HotkeyName, "KEY_")
	b.WriteString(hotkeyStyle.Render(fmt.Sprintf("Hotkey: %s (hold to record)", keyName)))
	b.WriteString("\n")
	b.WriteString(quitStyle.Render("Press q to quit"))

	// Debug sub-panel (inside main panel)
	if m.DebugMode || len(m.DebugEntries) > 0 {
		b.WriteString("\n\n")
		b.WriteString(m.renderDebugPanel())
	}

	return borderStyle.Width(panelWidthForStyle).Render(b.String())
}

const debugPanelMaxLines = 5

// Debug table column widths. Row content must fit within panelContentWidth.
const (
	colTimeWidth     = 15
	colCategoryWidth = 10
	colSepWidth      = 3 // " │ "
	colMsgWidth      = panelContentWidth - colTimeWidth - colCategoryWidth - colSepWidth*2
)

func (m Model) renderDebugPanel() string {
	sep := debugSepStyle.Render(" │ ")
	rule := debugRuleStyle.Render(strings.Repeat("─", panelContentWidth))

	var db strings.Builder

	// Title + divider
	db.WriteString(debugTitleStyle.Render("Debug"))
	db.WriteString("\n")
	db.WriteString(rule)
	db.WriteString("\n")

	// Header row
	db.WriteString(
		debugHeaderStyle.Width(colTimeWidth).Render("TIME") +
			sep +
			debugHeaderStyle.Width(colCategoryWidth).Render("TYPE") +
			sep +
			debugHeaderStyle.Width(colMsgWidth).Render("MESSAGE"))
	db.WriteString("\n")
	db.WriteString(rule)

	// Data rows
	entries := m.DebugEntries
	if len(entries) > debugPanelMaxLines {
		entries = entries[len(entries)-debugPanelMaxLines:]
	}
	for _, entry := range entries {
		timeStr := entry.Time
		if len(timeStr) > colTimeWidth {
			timeStr = timeStr[:colTimeWidth]
		}

		cat := entry.Category
		if len(cat) > colCategoryWidth {
			cat = cat[:colCategoryWidth]
		}

		msg := entry.Message
		if len(msg) > colMsgWidth {
			msg = msg[:colMsgWidth-3] + "..."
		}

		db.WriteString("\n")
		db.WriteString(
			debugTimeStyle.Width(colTimeWidth).Render(timeStr) +
				sep +
				debugCategoryStyle.Width(colCategoryWidth).Render(cat) +
				sep +
				debugMsgStyle.Width(colMsgWidth).Render(msg))
	}

	return db.String()
}

const visualizerWidth = 20

func (m Model) renderVisualizer() string {
	scaled := math.Sqrt(m.AudioLevel)
	filled := int(math.Round(scaled * float64(visualizerWidth)))
	if filled > visualizerWidth {
		filled = visualizerWidth
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", visualizerWidth-filled)
	return visualizerLabelStyle.Render("Mic  ") + visualizerStyle.Render(bar)
}

func (m Model) renderStatusBar() string {
	if !m.statusChecked {
		return quitStyle.Render("Mic: ...  Backend: ...  Model: ") + quitStyle.Render(m.Config.Transcription.Model)
	}
	var mic, backend string
	if m.MicDetected {
		mic = statusOkStyle.Render("✓")
		if m.MicDeviceName != "" {
			mic += quitStyle.Render(" ("+m.MicDeviceName+")")
		}
	} else {
		mic = statusBadStyle.Render("✗")
	}
	if m.BackendOnline {
		backend = statusOkStyle.Render("✓")
	} else {
		backend = statusBadStyle.Render("✗")
	}
	model := quitStyle.Render(m.Config.Transcription.Model)
	return quitStyle.Render("Mic: ") + mic + quitStyle.Render("  Backend: ") + backend + quitStyle.Render("  Model: ") + model
}

func (m Model) renderBadge() string {
	switch m.State {
	case StateRecording:
		return recordingBadge.Render("● Recording...")
	case StateTranscribing:
		return transcribingBadge.Render("● Transcribing...")
	case StateError:
		errText := m.LastError
		if len(errText) > 50 {
			errText = errText[:50] + "..."
		}
		return errorBadge.Render(fmt.Sprintf("● Error: %s", errText))
	default:
		return idleBadge.Render("● Idle")
	}
}
