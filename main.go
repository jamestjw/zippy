package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	statusGray = "#777777"
	pivotRed   = "#FF3B30"
)

type tickMsg struct{}

type model struct {
	words   []string
	idx     int
	running bool
	wpm     int
	width   int
	height  int
}

func (m model) Init() tea.Cmd {
	return nil
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
			if m.idx < len(m.words)-1 {
				m.idx++
			}
			return m, nil
		case "left", "h":
			if m.idx > 0 {
				m.idx--
			}
			return m, nil
		case "r":
			m.idx = 0
			if m.running {
				return m, tickCmd(m.wordInterval())
			}
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tickMsg:
		if !m.running {
			return m, nil
		}
		if m.idx < len(m.words)-1 {
			m.idx++
			return m, tickCmd(m.wordInterval())
		}
		m.running = false
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	if len(m.words) == 0 {
		return "No words to display."
	}
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	contentHeight := m.height
	if contentHeight > 1 {
		contentHeight--
	}

	word := m.words[m.idx]
	block := formatWord(word, m.width)
	body := lipgloss.Place(m.width, contentHeight, lipgloss.Left, lipgloss.Center, block)

	status := fmt.Sprintf("WPM %d  %d/%d  space: play/pause  +/-: speed  h/l: back/forward  r: restart  q: quit", m.wpm, m.idx+1, len(m.words))
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

func readInput(filePath string) (string, error) {
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	info, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}

	// If stdin is a terminal (not a pipe/file), treat it as "no input provided".
	if info.Mode()&os.ModeCharDevice != 0 {
		return "", fmt.Errorf("no input provided")
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func tokenize(text string) []string {
	var tokens []string
	var b strings.Builder
	for _, r := range text {
		if unicode.IsSpace(r) {
			if b.Len() > 0 {
				tokens = append(tokens, b.String())
				b.Reset()
			}
			continue
		}
		b.WriteRune(r)
	}
	if b.Len() > 0 {
		tokens = append(tokens, b.String())
	}
	return tokens
}

func main() {
	var (
		startWPM int
		wpm      int
		file     string
	)
	flag.IntVar(&startWPM, "start-wpm", 500, "starting words per minute")
	flag.IntVar(&wpm, "wpm", 0, "alias for -start-wpm")
	flag.StringVar(&file, "file", "", "path to input text")
	flag.Parse()

	if wpm > 0 {
		startWPM = wpm
	}

	text, err := readInput(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Provide input via -file or stdin.")
		flag.PrintDefaults()
		os.Exit(1)
	}

	words := tokenize(text)
	if len(words) == 0 {
		fmt.Fprintln(os.Stderr, "No words found in input.")
		os.Exit(1)
	}

	p := tea.NewProgram(model{
		words: words,
		wpm:   startWPM,
	})
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
