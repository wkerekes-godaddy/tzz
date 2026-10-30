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
	Name string `yaml:"name"`
	TZ   string `yaml:"tz"`
}

type config struct {
	WorkHours  [2]int       `yaml:"work_hours"`
	NightHours [2]int       `yaml:"night_hours"`
	Use24h     bool         `yaml:"use_24h"`
	Zones      []zoneConfig `yaml:"zones"`
}

func defaultConfig() config {
	return config{
		WorkHours:  [2]int{9, 17},
		NightHours: [2]int{22, 6},
		Use24h:     false,
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
		return cfg
	}

	return cfg
}

type zone struct {
	name     string
	location *time.Location
	offset   int
}

func loadZones(zoneConfigs []zoneConfig) []zone {
	var zones []zone
	for _, zc := range zoneConfigs {
		loc, err := time.LoadLocation(zc.TZ)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: unknown timezone %q, skipping\n", zc.TZ)
			continue
		}
		_, offset := time.Now().In(loc).Zone()
		zones = append(zones, zone{name: zc.Name, location: loc, offset: offset / 3600})
	}
	return zones
}

type tickMsg time.Time

type model struct {
	zones      []zone
	cfg        config
	width      int
	height     int
	now        time.Time
	cursorHour int
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

func (m model) isWorkHours(hour int) bool {
	return hour >= m.cfg.WorkHours[0] && hour <= m.cfg.WorkHours[1]
}

func (m model) isNight(hour int) bool {
	start := m.cfg.NightHours[0]
	end := m.cfg.NightHours[1]
	if start > end {
		return hour >= start || hour < end
	}
	return hour >= start && hour < end
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
				if m.isNight(localH) {
					style = style.Background(nightBg)
				} else if m.isWorkHours(localH) {
					style = style.Background(workBg)
				}
			} else if m.isNight(localH) {
				style = style.Foreground(dimFg).Background(nightBg)
			} else if m.isWorkHours(localH) {
				style = style.Foreground(brightFg).Background(workBg)
			} else {
				style = style.Foreground(lipgloss.Color("#AAAAAA"))
			}

			b.WriteString(style.Render(fmt.Sprintf("%02d", localH)))
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

		nameStyle := lipgloss.NewStyle().Foreground(color).Bold(true).Width(labelWidth)
		timeStyle := lipgloss.NewStyle().Foreground(brightFg)

		var timeStr string
		if m.cfg.Use24h {
			timeStr = fmt.Sprintf("%02d:00", localH)
		} else {
			period := "am"
			if localH >= 12 {
				period = "pm"
			}
			display12 := localH % 12
			if display12 == 0 {
				display12 = 12
			}
			timeStr = fmt.Sprintf("%2d:00 %s", display12, period)
		}

		indicator := " "
		if m.isWorkHours(localH) {
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
	b.WriteString(legend.Render(fmt.Sprintf("  ● work hours (%d-%d)   ◑ night (%d:00-%d:00)",
		m.cfg.WorkHours[0], m.cfg.WorkHours[1],
		m.cfg.NightHours[0], m.cfg.NightHours[1])) + "\n")

	help := lipgloss.NewStyle().Foreground(helpColor).Italic(true)
	b.WriteString(help.Render("  ←/→ slide time   n = now   q = quit") + "\n")

	return b.String()
}

func main() {
	cfg := loadConfig()
	zones := loadZones(cfg.Zones)

	now := time.Now()
	m := model{
		zones:      zones,
		cfg:        cfg,
		now:        now,
		cursorHour: now.UTC().Hour(),
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
