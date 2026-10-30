package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

type zoneConfig struct {
	Name   string `yaml:"name"`
	TZ     string `yaml:"tz"`
	Use24h *bool  `yaml:"use_24h"`
}

type config struct {
	Primary       string       `yaml:"primary"`
	WorkHours     [2]int       `yaml:"work_hours"`
	NightHours    [2]int       `yaml:"night_hours"`
	ShowWorkHours *bool        `yaml:"show_work_hours"`
	ShowNight     *bool        `yaml:"show_night"`
	UTCAlways24h  *bool        `yaml:"utc_always_24h"`
	Zones         []zoneConfig `yaml:"zones"`
}

func boolPtr(b bool) *bool { return &b }

func defaultConfig() config {
	return config{
		WorkHours:     [2]int{9, 17},
		NightHours:    [2]int{22, 6},
		ShowWorkHours: boolPtr(true),
		ShowNight:     boolPtr(true),
		UTCAlways24h:  boolPtr(true),
		Zones: []zoneConfig{
			{Name: "UTC", TZ: "UTC"},
			{Name: "Eastern", TZ: "America/New_York"},
			{Name: "Central", TZ: "America/Chicago"},
			{Name: "Mountain", TZ: "America/Denver"},
			{Name: "Arizona", TZ: "America/Phoenix"},
			{Name: "Pacific", TZ: "America/Los_Angeles"},
			{Name: "Alaska", TZ: "America/Anchorage"},
			{Name: "Hawaii", TZ: "Pacific/Honolulu"},
		},
	}
}

func configPaths() []string {
	var paths []string

	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		if home, err := os.UserHomeDir(); err == nil {
			xdg = filepath.Join(home, ".config")
		}
	}
	if xdg != "" {
		paths = append(paths, filepath.Join(xdg, "tzz", "config.yaml"))
	}

	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".tzz.yaml"))
	}

	return paths
}

func loadConfig() config {
	cfg := defaultConfig()

	for _, path := range configPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", path, err)
			return defaultConfig()
		}
		if cfg.ShowWorkHours == nil {
			cfg.ShowWorkHours = boolPtr(true)
		}
		if cfg.ShowNight == nil {
			cfg.ShowNight = boolPtr(true)
		}
		if cfg.UTCAlways24h == nil {
			cfg.UTCAlways24h = boolPtr(true)
		}
		return cfg
	}

	return cfg
}

// localeUses24h returns the conventional time format for a timezone.
// US zones use 12h; UTC and most of the world use 24h.
func localeUses24h(tz string) bool {
	if tz == "UTC" || strings.HasPrefix(tz, "Etc/") {
		return true
	}
	// 12-hour countries: US, Canada, Australia, Philippines
	twelveHourPrefixes := []string{
		"America/New_York", "America/Chicago", "America/Denver",
		"America/Los_Angeles", "America/Phoenix", "America/Anchorage",
		"America/Adak", "America/Detroit", "America/Indiana",
		"America/Kentucky", "America/Menominee", "America/Nome",
		"America/North_Dakota", "America/Sitka", "America/Yakutat",
		"America/Boise", "America/Juneau",
		"Pacific/Honolulu", "Pacific/Guam", "Pacific/Pago_Pago",
		"America/Toronto", "America/Vancouver", "America/Edmonton",
		"America/Winnipeg", "America/Halifax", "America/St_Johns",
		"America/Regina",
		"Australia/",
		"Asia/Manila",
	}
	for _, prefix := range twelveHourPrefixes {
		if tz == prefix || strings.HasPrefix(tz, prefix) {
			return false
		}
	}
	return true
}

type zone struct {
	name       string
	tz         string
	location   *time.Location
	offset     int
	default24h bool // locale convention
	config24h  *bool
}

func loadZones(cfg config) ([]zone, int, bool) {
	var zones []zone
	var primary *zone
	primaryOffset := 0
	hasPrimary := cfg.Primary != ""

	for _, zc := range cfg.Zones {
		loc, err := time.LoadLocation(zc.TZ)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: unknown timezone %q, skipping\n", zc.TZ)
			continue
		}
		_, offset := time.Now().In(loc).Zone()
		z := zone{
			name:       zc.Name,
			tz:         zc.TZ,
			location:   loc,
			offset:     offset / 3600,
			default24h: localeUses24h(zc.TZ),
			config24h:  zc.Use24h,
		}
		if hasPrimary && strings.EqualFold(zc.Name, cfg.Primary) {
			primary = &z
			primaryOffset = z.offset
		} else {
			zones = append(zones, z)
		}
	}

	if primary != nil {
		zones = append([]zone{*primary}, zones...)
	}

	return zones, primaryOffset, hasPrimary
}

type timeFormatMode int

const (
	formatLocale timeFormatMode = iota // per-zone locale defaults
	formatAll24h
	formatAll12h
)

type tickMsg time.Time

type model struct {
	zones         []zone
	cfg           config
	primaryOffset int
	hasPrimary    bool
	width         int
	height        int
	now           time.Time
	cursorHour    int
	formatMode    timeFormatMode
}

func (m model) shouldUse24h(z zone) bool {
	// UTC override takes priority
	if *m.cfg.UTCAlways24h && (z.tz == "UTC" || strings.HasPrefix(z.tz, "Etc/")) {
		return true
	}
	switch m.formatMode {
	case formatAll24h:
		return true
	case formatAll12h:
		return false
	default:
		if z.config24h != nil {
			return *z.config24h
		}
		return z.default24h
	}
}

func tickEvery() tea.Cmd {
	return tea.Every(time.Minute, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Init() tea.Cmd {
	return tickEvery()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "left", "h":
			m.cursorHour = (m.cursorHour - 1 + 24) % 24
		case "right", "l":
			m.cursorHour = (m.cursorHour + 1) % 24
		case "n":
			m.cursorHour = m.now.UTC().Hour()
		case "t":
			m.formatMode = (m.formatMode + 1) % 3
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tickMsg:
		m.now = time.Time(msg)
		return m, tickEvery()
	}
	return m, nil
}

func hourInZone(utcHour, offsetHours int) int {
	h := (utcHour + offsetHours) % 24
	if h < 0 {
		h += 24
	}
	return h
}

func (m model) isWorkHoursUTC(utcHour int) bool {
	if !*m.cfg.ShowWorkHours {
		return false
	}
	primaryLocal := hourInZone(utcHour, m.primaryOffset)
	return primaryLocal >= m.cfg.WorkHours[0] && primaryLocal <= m.cfg.WorkHours[1]
}

func (m model) isNight(hour int) bool {
	if !*m.cfg.ShowNight {
		return false
	}
	start := m.cfg.NightHours[0]
	end := m.cfg.NightHours[1]
	if start > end {
		return hour >= start || hour < end
	}
	return hour >= start && hour < end
}

// formatHourCell formats an hour for the bar graph (compact, fits in 4 chars).
func formatHourCell(hour int, use24h bool) string {
	if use24h {
		return fmt.Sprintf("%02d", hour)
	}
	display := hour % 12
	if display == 0 {
		display = 12
	}
	suffix := "a"
	if hour >= 12 {
		suffix = "p"
	}
	return fmt.Sprintf("%d%s", display, suffix)
}

// formatHourFull formats an hour for the selected-time readout.
func formatHourFull(hour int, use24h bool) string {
	if use24h {
		return fmt.Sprintf("%02d:00", hour)
	}
	display := hour % 12
	if display == 0 {
		display = 12
	}
	period := "am"
	if hour >= 12 {
		period = "pm"
	}
	return fmt.Sprintf("%d:00 %s", display, period)
}

var (
	dimFg      = lipgloss.Color("#555555")
	brightFg   = lipgloss.Color("#FFFFFF")
	workBg     = lipgloss.Color("#1a3a1a")
	nightBg    = lipgloss.Color("#1a1a2e")
	cursorBg   = lipgloss.Color("#FF6B6B")
	cursorFg   = lipgloss.Color("#000000")
	nowMarker  = lipgloss.Color("#4ECDC4")
	labelColor = lipgloss.Color("#45B7D1")
	titleColor = lipgloss.Color("#FF6B6B")
	helpColor  = lipgloss.Color("#555555")

	zoneColors = []lipgloss.Color{
		lipgloss.Color("#FF6B6B"),
		lipgloss.Color("#4ECDC4"),
		lipgloss.Color("#45B7D1"),
		lipgloss.Color("#96CEB4"),
		lipgloss.Color("#FFEAA7"),
		lipgloss.Color("#DDA0DD"),
		lipgloss.Color("#FF8C42"),
	}
)

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(titleColor).
		Render("TZZ — Time Zone Zen")
	b.WriteString("\n  " + title + "\n\n")

	labelWidth := 10
	cellWidth := 4
	availWidth := m.width - labelWidth - 2
	maxHours := availWidth / cellWidth
	if maxHours > 24 {
		maxHours = 24
	}
	if maxHours < 12 {
		maxHours = 12
	}

	startOffset := maxHours / 2
	startUTC := (m.cursorHour - startOffset + 24) % 24

	nowUTCHour := m.now.UTC().Hour()

	// Header row — always 24h (it's the UTC reference axis)
	headerLabel := lipgloss.NewStyle().
		Width(labelWidth).
		Foreground(dimFg).
		Render("")
	b.WriteString("  " + headerLabel)

	for i := 0; i < maxHours; i++ {
		utcH := (startUTC + i) % 24
		style := lipgloss.NewStyle().Width(cellWidth).Align(lipgloss.Center)
		if utcH == m.cursorHour {
			style = style.Foreground(cursorFg).Background(cursorBg).Bold(true)
		} else if utcH == nowUTCHour {
			style = style.Foreground(nowMarker).Bold(true)
			if m.isWorkHoursUTC(utcH) {
				style = style.Background(workBg)
			}
		} else if m.isWorkHoursUTC(utcH) {
			style = style.Foreground(lipgloss.Color("#AAAAAA")).Background(workBg)
		} else {
			style = style.Foreground(dimFg)
		}
		b.WriteString(style.Render(fmt.Sprintf("%02d", utcH)))
	}
	b.WriteString("\n")

	sep := lipgloss.NewStyle().Foreground(dimFg).Render(
		"  " + strings.Repeat("─", labelWidth+maxHours*cellWidth))
	b.WriteString(sep + "\n")

	for zi, z := range m.zones {
		color := zoneColors[zi%len(zoneColors)]
		use24h := m.shouldUse24h(z)

		label := lipgloss.NewStyle().
			Width(labelWidth).
			Foreground(color).
			Bold(true).
			Render(z.name)
		b.WriteString("  " + label)

		for i := 0; i < maxHours; i++ {
			utcH := (startUTC + i) % 24
			localH := hourInZone(utcH, z.offset)

			style := lipgloss.NewStyle().Width(cellWidth).Align(lipgloss.Center)

			if utcH == m.cursorHour {
				style = style.Foreground(cursorFg).Background(cursorBg).Bold(true)
			} else if utcH == nowUTCHour {
				style = style.Foreground(nowMarker).Bold(true)
				if m.isWorkHoursUTC(utcH) {
					style = style.Background(workBg)
				} else if m.isNight(localH) {
					style = style.Background(nightBg)
				}
			} else if m.isWorkHoursUTC(utcH) {
				style = style.Foreground(brightFg).Background(workBg)
			} else if m.isNight(localH) {
				style = style.Foreground(dimFg).Background(nightBg)
			} else {
				style = style.Foreground(lipgloss.Color("#AAAAAA"))
			}

			b.WriteString(style.Render(formatHourCell(localH, use24h)))
		}
		b.WriteString("\n")
	}

	b.WriteString(sep + "\n\n")

	selectedLabel := lipgloss.NewStyle().
		Foreground(labelColor).
		Bold(true).
		Render("  Selected:")
	b.WriteString(selectedLabel + "\n")

	for zi, z := range m.zones {
		color := zoneColors[zi%len(zoneColors)]
		localH := hourInZone(m.cursorHour, z.offset)
		use24h := m.shouldUse24h(z)

		nameStyle := lipgloss.NewStyle().Foreground(color).Bold(true).Width(labelWidth)
		timeStyle := lipgloss.NewStyle().Foreground(brightFg)

		timeStr := formatHourFull(localH, use24h)

		indicator := " "
		if m.isWorkHoursUTC(m.cursorHour) {
			indicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#96CEB4")).Render("●")
		} else if m.isNight(localH) {
			indicator = lipgloss.NewStyle().Foreground(dimFg).Render("◑")
		}

		b.WriteString(fmt.Sprintf("  %s %s %s\n",
			nameStyle.Render(z.name),
			timeStyle.Render(timeStr),
			indicator,
		))
	}

	b.WriteString("\n")

	nowStr := lipgloss.NewStyle().Foreground(nowMarker).Render(
		fmt.Sprintf("  Now: %s UTC", m.now.UTC().Format("15:04")))
	b.WriteString(nowStr + "\n\n")

	legend := lipgloss.NewStyle().Foreground(helpColor)
	var legendParts []string
	if *m.cfg.ShowWorkHours {
		workLabel := fmt.Sprintf("● work hours (%d-%d", m.cfg.WorkHours[0], m.cfg.WorkHours[1])
		if m.hasPrimary && len(m.zones) > 0 {
			workLabel += " " + m.zones[0].name
		}
		workLabel += ")"
		legendParts = append(legendParts, workLabel)
	}
	if *m.cfg.ShowNight {
		legendParts = append(legendParts, fmt.Sprintf("◑ night (%d:00-%d:00)",
			m.cfg.NightHours[0], m.cfg.NightHours[1]))
	}
	if len(legendParts) > 0 {
		b.WriteString(legend.Render("  "+strings.Join(legendParts, "   ")) + "\n")
	}

	formatLabel := "locale"
	switch m.formatMode {
	case formatAll24h:
		formatLabel = "24h"
	case formatAll12h:
		formatLabel = "12h"
	}

	help := lipgloss.NewStyle().Foreground(helpColor).Italic(true)
	b.WriteString(help.Render(fmt.Sprintf("  ←/→ slide time   n = now   t = format [%s]   q = quit", formatLabel)) + "\n")

	return b.String()
}

func main() {
	cfg := loadConfig()
	zones, primaryOffset, hasPrimary := loadZones(cfg)

	now := time.Now()
	m := model{
		zones:         zones,
		cfg:           cfg,
		primaryOffset: primaryOffset,
		hasPrimary:    hasPrimary,
		now:           now,
		cursorHour:    now.UTC().Hour(),
		formatMode:    formatLocale,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
