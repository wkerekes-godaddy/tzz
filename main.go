package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type zone struct {
	name     string
	location *time.Location
	use24h   bool
}

type tickMsg time.Time

type model struct {
	zones  []zone
	width  int
	height int
	now    time.Time
}

func defaultZones() []zone {
	load := func(name, tz string, use24h bool) zone {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			loc = time.UTC
		}
		return zone{name: name, location: loc, use24h: use24h}
	}

	return []zone{
		load("UTC", "UTC", true),
		load("Eastern", "America/New_York", false),
		load("Central", "America/Chicago", false),
		load("Mountain", "America/Denver", false),
		load("Pacific", "America/Los_Angeles", false),
		load("Alaska", "America/Anchorage", false),
		load("Hawaii", "Pacific/Honolulu", false),
	}
}

func tickEvery() tea.Cmd {
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
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

var (
	colors = []lipgloss.Color{
		lipgloss.Color("#FF6B6B"),
		lipgloss.Color("#4ECDC4"),
		lipgloss.Color("#45B7D1"),
		lipgloss.Color("#96CEB4"),
		lipgloss.Color("#FFEAA7"),
		lipgloss.Color("#DDA0DD"),
		lipgloss.Color("#FF8C42"),
	}

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6B6B")).
			Background(lipgloss.Color("#1a1a2e")).
			Padding(0, 1).
			MarginBottom(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")).
			Italic(true)
)

func clockBox(z zone, now time.Time, color lipgloss.Color, width int) string {
	t := now.In(z.location)

	var timeStr string
	if z.use24h {
		timeStr = t.Format("15:04:05")
	} else {
		timeStr = t.Format("3:04:05 PM")
	}

	dateStr := t.Format("Mon, Jan 2")

	_, offset := t.Zone()
	hours := offset / 3600
	sign := "+"
	if hours < 0 {
		sign = "-"
		hours = -hours
	}
	offsetStr := fmt.Sprintf("UTC%s%d", sign, hours)

	nameStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(color).
		Width(width).
		Align(lipgloss.Center)

	timeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Width(width).
		Align(lipgloss.Center)

	dateStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Width(width).
		Align(lipgloss.Center)

	offsetStyle := lipgloss.NewStyle().
		Foreground(color).
		Width(width).
		Align(lipgloss.Center)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		nameStyle.Render(z.name),
		"",
		timeStyle.Render(timeStr),
		dateStyle.Render(dateStr),
		"",
		offsetStyle.Render(offsetStr),
		"",
	)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Width(width).
		Padding(0, 1)

	return boxStyle.Render(content)
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	title := titleStyle.Render("  T Z Z  —  Time Zone Zen  ")

	boxWidth := 20
	maxPerRow := m.width / (boxWidth + 4)
	if maxPerRow < 1 {
		maxPerRow = 1
	}

	var rows []string
	for i := 0; i < len(m.zones); i += maxPerRow {
		end := i + maxPerRow
		if end > len(m.zones) {
			end = len(m.zones)
		}

		var boxes []string
		for j := i; j < end; j++ {
			color := colors[j%len(colors)]
			boxes = append(boxes, clockBox(m.zones[j], m.now, color, boxWidth))
		}

		row := lipgloss.JoinHorizontal(lipgloss.Top, boxes...)
		rows = append(rows, row)
	}

	clockGrid := lipgloss.JoinVertical(lipgloss.Left, rows...)

	sep := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#333333")).
		Render(strings.Repeat("─", min(m.width-1, 80)))

	help := helpStyle.Render("q/esc to quit")

	return lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		title,
		"",
		clockGrid,
		"",
		sep,
		help,
	)
}

func main() {
	m := model{
		zones: defaultZones(),
		now:   time.Now(),
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
