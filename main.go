package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	statusGray = "#777777"
	pivotRed   = "#FF3B30"
)

type tickMsg struct{}

type model struct {
	stream  stream
	running bool
	wpm     int
	width   int
	height  int
}

func (m model) Init() tea.Cmd {
	if m.stream == nil {
		return nil
	}
	return m.stream.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case " ":
			m.running = !m.running
			if m.running {
				return m, tickCmd(m.wordInterval())
			}
			return m, nil
		case "+", "=", "up":
			m.adjustWPM(25)
			if m.running {
				return m, tickCmd(m.wordInterval())
			}
			return m, nil
		case "-", "_", "down":
			m.adjustWPM(-25)
			if m.running {
				return m, tickCmd(m.wordInterval())
			}
			return m, nil
		case "right", "l":
			if m.stream == nil || !m.stream.SupportsSeek() {
				return m, nil
			}
			m.stream.Next()
			return m, nil
		case "left", "h":
			if m.stream == nil || !m.stream.SupportsSeek() {
				return m, nil
			}
			m.stream.Prev()
			return m, nil
		case "r":
			// Restart is only available for file input; stdin cannot be replayed.
			if m.stream == nil || !m.stream.SupportsRestart() {
				return m, nil
			}
			cmd := m.stream.Restart()
			if m.running && cmd == nil {
				return m, tickCmd(m.wordInterval())
			}
			return m, cmd
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tickMsg:
		if !m.running {
			return m, nil
		}
		if m.stream == nil || !m.stream.CanAdvance() {
			m.running = false
			return m, nil
		}
		cmd := m.stream.Next()
		if cmd != nil {
			return m, cmd
		}
		return m, tickCmd(m.wordInterval())
	case tokenMsg:
		if m.stream == nil {
			return m, nil
		}
		cmd := m.stream.Handle(msg)
		if cmd != nil {
			return m, cmd
		}
		if m.running {
			if _, ok := m.stream.Current(); ok {
				return m, tickCmd(m.wordInterval())
			}
			if !m.stream.CanAdvance() {
				m.running = false
			}
		}
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	if m.stream == nil {
		return "No words to display."
	}
	if err := m.stream.Err(); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	word, ok := m.stream.Current()
	if !ok {
		if !m.stream.CanAdvance() {
			return "No words to display."
		}
		return "Loading..."
	}
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	contentHeight := m.height
	if contentHeight > 1 {
		contentHeight--
	}

	block := formatWord(word, m.width)
	body := lipgloss.Place(m.width, contentHeight, lipgloss.Left, lipgloss.Center, block)

	total := "?"
	if known, count := m.stream.Total(); known {
		total = fmt.Sprintf("%d", count)
	}
	controls := "space: play/pause  +/-: speed"
	if m.stream.SupportsSeek() {
		controls += "  h/l: back/forward"
	}
	if m.stream.SupportsRestart() {
		controls += "  r: restart"
	}
	controls += "  q: quit"
	status := fmt.Sprintf("WPM %d  %d/%s  %s", m.wpm, m.stream.Pos()+1, total, controls)
	statusLine := lipgloss.NewStyle().Foreground(lipgloss.Color(statusGray)).Render(truncate(status, m.width))

	if contentHeight < m.height {
		return body + "\n" + statusLine
	}
	return body
}

func (m model) wordInterval() time.Duration {
	if m.wpm <= 0 {
		return time.Second
	}
	return time.Minute / time.Duration(m.wpm)
}

func tickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m *model) adjustWPM(delta int) {
	m.wpm += delta
	if m.wpm < 50 {
		m.wpm = 50
	}
	if m.wpm > 1200 {
		m.wpm = 1200
	}
}

func formatWord(word string, width int) string {
	if width <= 0 {
		return word
	}
	runes := []rune(word)
	if len(runes) == 0 {
		return ""
	}

	pivot := pivotIndex(len(runes))
	if pivot >= len(runes) {
		pivot = len(runes) - 1
	}

	leftRunes := runes[:pivot]
	pivotRune := string(runes[pivot])
	rightRunes := runes[pivot+1:]

	pivotStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(pivotRed)).Bold(true)

	left := string(leftRunes)
	right := string(rightRunes)

	center := width / 2
	leftPad := max(center-lipgloss.Width(left), 0)

	padding := strings.Repeat(" ", leftPad)
	line := padding + left + pivotStyle.Render(pivotRune) + right
	return line
}

func pivotIndex(length int) int {
	switch {
	case length <= 1:
		return 0
	case length <= 5:
		return 1
	case length <= 9:
		return 2
	case length <= 13:
		return 3
	default:
		return 4
	}
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	return string(runes[:width])
}

func main() {
	var (
		startWPM int
		wpm      int
		file     string
		lazy     bool
	)
	flag.IntVar(&startWPM, "start-wpm", 500, "starting words per minute")
	flag.IntVar(&wpm, "wpm", 0, "alias for -start-wpm")
	flag.StringVar(&file, "file", "", "path to input text")
	flag.BoolVar(&lazy, "lazy", false, "stream tokens lazily without buffering; disables back/forward")
	flag.Parse()

	if wpm > 0 {
		startWPM = wpm
	}

	stream, err := buildStream(lazy, file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if initErr, ok := err.(streamInitError); ok && initErr.showUsage {
			flag.PrintDefaults()
		}
		os.Exit(1)
	}

	p := tea.NewProgram(model{
		wpm:    startWPM,
		stream: stream,
	})
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
