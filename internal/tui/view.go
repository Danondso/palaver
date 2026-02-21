package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Styles — initialized with synthwave defaults, overridden by applyTheme().
var (
	titleStyle        lipgloss.Style
	borderStyle       lipgloss.Style
	labelStyle        lipgloss.Style
	transcriptStyle   lipgloss.Style
	hotkeyStyle       lipgloss.Style
	quitStyle         lipgloss.Style
	idleBadge         lipgloss.Style
	recordingBadge    lipgloss.Style
	transcribingBadge lipgloss.Style
	errorBadge        lipgloss.Style
	bodyStyle         lipgloss.Style
	debugTitleStyle   lipgloss.Style
	debugRuleStyle    lipgloss.Style
	debugHeaderStyle  lipgloss.Style
	debugTimeStyle    lipgloss.Style
	debugCategoryStyle lipgloss.Style
	debugMsgStyle     lipgloss.Style
	debugSepStyle     lipgloss.Style
	visualizerStyle   lipgloss.Style
	visualizerLabelStyle lipgloss.Style
	statusOkStyle     lipgloss.Style
	statusBadStyle    lipgloss.Style
)

func init() {
	applyTheme(LoadTheme("synthwave"))
}

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
	b.WriteString(quitStyle.Render("Press q to quit  t: theme (" + m.themeName + ")"))

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
		return quitStyle.Render("Mic: ...  Backend: ...  Model: ...")
	}
	var mic, backend string
	if m.MicDetected {
		mic = statusOkStyle.Render("✓")
		if m.MicDeviceName != "" {
			mic += quitStyle.Render(" (" + m.MicDeviceName + ")")
		}
	} else {
		mic = statusBadStyle.Render("✗")
	}
	if m.BackendOnline {
		backend = statusOkStyle.Render("✓")
	} else {
		backend = statusBadStyle.Render("✗")
	}
	modelName := m.ModelName
	if modelName == "" {
		modelName = "n/a"
	}
	model := quitStyle.Render(modelName)
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
