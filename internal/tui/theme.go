package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Danondso/palaver/internal/config"
)

// Theme defines the color palette for the TUI.
type Theme struct {
	Name       string
	Primary    lipgloss.Color // title, recording badge, visualizer bar
	Secondary  lipgloss.Color // labels, hotkey text, border
	Accent     lipgloss.Color // transcript text
	Error      lipgloss.Color // error badge, status bad
	Success    lipgloss.Color // idle badge, status ok
	Warning    lipgloss.Color // transcribing badge, debug category
	Background lipgloss.Color // panel background
	Text       lipgloss.Color // body text
	Dimmed     lipgloss.Color // quit text, debug text, visualizer label
	Separator  lipgloss.Color // debug separator
}

var themes = map[string]Theme{
	"synthwave": {
		Name:       "Synthwave",
		Primary:    lipgloss.Color("#FF6AC1"),
		Secondary:  lipgloss.Color("#00E5FF"),
		Accent:     lipgloss.Color("#B388FF"),
		Error:      lipgloss.Color("#FF8A80"),
		Success:    lipgloss.Color("#64FFDA"),
		Warning:    lipgloss.Color("#FFAB40"),
		Background: lipgloss.Color("#1A1A2E"),
		Text:       lipgloss.Color("#E0E0E0"),
		Dimmed:     lipgloss.Color("#666666"),
		Separator:  lipgloss.Color("#444444"),
	},
	"everforest": {
		Name:       "Everforest",
		Primary:    lipgloss.Color("#A7C080"),
		Secondary:  lipgloss.Color("#7FBBB3"),
		Accent:     lipgloss.Color("#D699B6"),
		Error:      lipgloss.Color("#E67E80"),
		Success:    lipgloss.Color("#83C092"),
		Warning:    lipgloss.Color("#DBBC7F"),
		Background: lipgloss.Color("#2D353B"),
		Text:       lipgloss.Color("#D3C6AA"),
		Dimmed:     lipgloss.Color("#859289"),
		Separator:  lipgloss.Color("#4F585E"),
	},
	"gruvbox": {
		Name:       "Gruvbox",
		Primary:    lipgloss.Color("#FB4934"),
		Secondary:  lipgloss.Color("#83A598"),
		Accent:     lipgloss.Color("#D3869B"),
		Error:      lipgloss.Color("#FB4934"),
		Success:    lipgloss.Color("#B8BB26"),
		Warning:    lipgloss.Color("#FABD2F"),
		Background: lipgloss.Color("#282828"),
		Text:       lipgloss.Color("#EBDBB2"),
		Dimmed:     lipgloss.Color("#928374"),
		Separator:  lipgloss.Color("#504945"),
	},
	"monochrome": {
		Name:       "Monochrome",
		Primary:    lipgloss.Color("#FFFFFF"),
		Secondary:  lipgloss.Color("#CCCCCC"),
		Accent:     lipgloss.Color("#AAAAAA"),
		Error:      lipgloss.Color("#FF0000"),
		Success:    lipgloss.Color("#FFFFFF"),
		Warning:    lipgloss.Color("#CCCCCC"),
		Background: lipgloss.Color("#000000"),
		Text:       lipgloss.Color("#FFFFFF"),
		Dimmed:     lipgloss.Color("#888888"),
		Separator:  lipgloss.Color("#444444"),
	},
}

// themeOrder defines the fixed cycle order for theme toggling.
var themeOrder = []string{"synthwave", "everforest", "gruvbox", "monochrome"}

// ThemeNames returns the names of all available built-in themes in cycle order.
func ThemeNames() []string {
	return themeOrder
}

// LoadTheme returns the theme with the given name (case-insensitive).
// Falls back to synthwave if the name is not recognized.
func LoadTheme(name string) Theme {
	if t, ok := themes[strings.ToLower(name)]; ok {
		return t
	}
	return themes["synthwave"]
}

// NextTheme returns the theme after the given one in the cycle order.
func NextTheme(current string) Theme {
	current = strings.ToLower(current)
	for i, name := range themeOrder {
		if name == current {
			next := themeOrder[(i+1)%len(themeOrder)]
			return themes[next]
		}
	}
	return themes[themeOrder[0]]
}

// builtinThemes is the set of theme keys that cannot be overridden by custom themes.
var builtinThemes = map[string]bool{
	"synthwave":  true,
	"everforest": true,
	"gruvbox":    true,
	"monochrome": true,
}

// RegisterCustomThemes converts config custom themes into the internal themes
// map and appends them to the theme cycle order. Silently skips entries with
// empty names or names that collide with built-in themes.
func RegisterCustomThemes(custom []config.CustomTheme) {
	for _, ct := range custom {
		key := strings.ToLower(ct.Name)
		if key == "" || builtinThemes[key] {
			continue
		}
		if _, exists := themes[key]; exists {
			continue
		}
		themes[key] = Theme{
			Name:       ct.Name,
			Primary:    lipgloss.Color(ct.Primary),
			Secondary:  lipgloss.Color(ct.Secondary),
			Accent:     lipgloss.Color(ct.Accent),
			Error:      lipgloss.Color(ct.Error),
			Success:    lipgloss.Color(ct.Success),
			Warning:    lipgloss.Color(ct.Warning),
			Background: lipgloss.Color(ct.Background),
			Text:       lipgloss.Color(ct.Text),
			Dimmed:     lipgloss.Color(ct.Dimmed),
			Separator:  lipgloss.Color(ct.Separator),
		}
		themeOrder = append(themeOrder, key)
	}
}

// applyTheme updates all TUI style variables to use the given theme's colors.
func applyTheme(t Theme) {
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Background(t.Background).
		MarginBottom(1)

	borderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Secondary).
		Padding(1, 2).
		Background(t.Background)

	labelStyle = lipgloss.NewStyle().
		Foreground(t.Secondary).
		Background(t.Background).
		Bold(true)

	transcriptStyle = lipgloss.NewStyle().
		Foreground(t.Accent).
		Background(t.Background).
		Italic(true)

	hotkeyStyle = lipgloss.NewStyle().
		Foreground(t.Secondary).
		Background(t.Background)

	quitStyle = lipgloss.NewStyle().
		Foreground(t.Dimmed).
		Background(t.Background)

	idleBadge = lipgloss.NewStyle().
		Foreground(t.Success).
		Background(t.Background).
		Bold(true)

	recordingBadge = lipgloss.NewStyle().
		Foreground(t.Primary).
		Background(t.Background).
		Bold(true)

	transcribingBadge = lipgloss.NewStyle().
		Foreground(t.Warning).
		Background(t.Background).
		Bold(true)

	errorBadge = lipgloss.NewStyle().
		Foreground(t.Error).
		Background(t.Background).
		Bold(true)

	bodyStyle = lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.Background)

	debugTitleStyle = lipgloss.NewStyle().
		Foreground(t.Dimmed).
		Background(t.Background).
		Bold(true)

	debugRuleStyle = lipgloss.NewStyle().
		Foreground(t.Dimmed).
		Background(t.Background)

	debugHeaderStyle = lipgloss.NewStyle().
		Foreground(t.Dimmed).
		Background(t.Background).
		Bold(true)

	debugTimeStyle = lipgloss.NewStyle().
		Foreground(t.Dimmed).
		Background(t.Background)

	debugCategoryStyle = lipgloss.NewStyle().
		Foreground(t.Warning).
		Background(t.Background)

	debugMsgStyle = lipgloss.NewStyle().
		Foreground(t.Dimmed).
		Background(t.Background)

	debugSepStyle = lipgloss.NewStyle().
		Foreground(t.Separator).
		Background(t.Background)

	visualizerStyle = lipgloss.NewStyle().
		Foreground(t.Primary).
		Background(t.Background)

	visualizerLabelStyle = lipgloss.NewStyle().
		Foreground(t.Dimmed).
		Background(t.Background)

	statusOkStyle = lipgloss.NewStyle().
		Foreground(t.Success).
		Background(t.Background).
		Bold(true)

	statusBadStyle = lipgloss.NewStyle().
		Foreground(t.Error).
		Background(t.Background).
		Bold(true)
}
