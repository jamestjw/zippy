package main

import (
	"bufio"
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

type tokenMsg struct {
	word string
	done bool
	err  error
}

type model struct {
	words          []string
	idx            int
	running        bool
	wpm            int
	width          int
	height         int
	streamDone     bool
	streamErr      error
	tokenizer      *tokenizer
	inputCloser    io.Closer
	lazy           bool
	waitingToken   bool
	pendingAdvance bool
	hasCurrent     bool
	currentWord    string
	filePath       string
}

func (m model) Init() tea.Cmd {
	if m.tokenizer == nil {
		return nil
	}
	if m.lazy {
		return m.requestToken(true)
	}
	return tokenizeCmd(m.tokenizer)
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
			if m.lazy {
				return m, nil
			}
			if m.idx < len(m.words)-1 {
				m.idx++
			}
			return m, nil
		case "left", "h":
			if m.lazy {
				return m, nil
			}
			if m.idx > 0 {
				m.idx--
			}
			return m, nil
		case "r":
			// Restart is only available for file input; stdin cannot be replayed.
			if m.filePath == "" {
				return m, nil
			}
			if m.lazy {
				return m, m.restartStream()
			}
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
		if m.lazy {
			if m.streamDone {
				m.running = false
				return m, nil
			}
			return m, m.requestToken(true)
		}
		if m.idx < len(m.words)-1 {
			m.idx++
			return m, tickCmd(m.wordInterval())
		}
		if m.streamDone {
			m.running = false
			return m, nil
		}
		return m, tickCmd(m.wordInterval())
	case tokenMsg:
		if m.lazy {
			return m.handleLazyToken(msg)
		}
		if msg.err != nil {
			m.streamErr = msg.err
			m.streamDone = true
			return m, nil
		}
		if msg.word != "" {
			m.words = append(m.words, msg.word)
		}
		if msg.done {
			m.streamDone = true
			if m.inputCloser != nil {
				_ = m.inputCloser.Close()
				m.inputCloser = nil
			}
			return m, nil
		}
		return m, tokenizeCmd(m.tokenizer)
	}

	return m, nil
}

func (m model) View() string {
	if m.streamErr != nil {
		return fmt.Sprintf("Error: %v", m.streamErr)
	}
	if m.lazy {
		if !m.hasCurrent && m.streamDone {
			return "No words to display."
		}
		if !m.hasCurrent {
			return "Loading..."
		}
	} else if len(m.words) == 0 && m.streamDone {
		return "No words to display."
	}
	if !m.lazy && len(m.words) == 0 {
		return "Loading..."
	}
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	contentHeight := m.height
	if contentHeight > 1 {
		contentHeight--
	}

	word := m.currentWord
	if !m.lazy {
		word = m.words[m.idx]
	}
	block := formatWord(word, m.width)
	body := lipgloss.Place(m.width, contentHeight, lipgloss.Left, lipgloss.Center, block)

	total := "?"
	if m.streamDone && !m.lazy {
		total = fmt.Sprintf("%d", len(m.words))
	}
	if m.streamDone && m.lazy {
		total = fmt.Sprintf("%d", m.idx+1)
	}
	controls := "space: play/pause  +/-: speed"
	if !m.lazy {
		controls += "  h/l: back/forward"
	}
	if m.filePath != "" {
		controls += "  r: restart"
	}
	controls += "  q: quit"
	status := fmt.Sprintf("WPM %d  %d/%s  %s", m.wpm, m.idx+1, total, controls)
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

func (m *model) requestToken(advance bool) tea.Cmd {
	if m.waitingToken || m.tokenizer == nil {
		return nil
	}
	m.pendingAdvance = advance
	m.waitingToken = true
	return tokenizeCmd(m.tokenizer)
}

func (m *model) handleLazyToken(msg tokenMsg) (tea.Model, tea.Cmd) {
	m.waitingToken = false
	if msg.err != nil {
		m.streamErr = msg.err
		m.streamDone = true
		return m, nil
	}
	if msg.word == "" && msg.done {
		m.streamDone = true
		m.closeInput()
		if m.running {
			m.running = false
		}
		return m, nil
	}
	if msg.word != "" {
		if m.pendingAdvance {
			m.idx++
		}
		m.pendingAdvance = false
		m.hasCurrent = true
		m.currentWord = msg.word
		if msg.done {
			m.streamDone = true
			m.closeInput()
		}
		if m.running {
			return m, tickCmd(m.wordInterval())
		}
	}
	if msg.done {
		m.streamDone = true
		m.closeInput()
	}
	return m, nil
}

type tokenizer struct {
	reader *bufio.Reader
	buf    strings.Builder
	done   bool
}

func newTokenizer(r io.Reader) *tokenizer {
	return &tokenizer{reader: bufio.NewReader(r)}
}

func (t *tokenizer) next() (string, bool, error) {
	if t.done {
		return "", true, nil
	}

	for {
		r, _, err := t.reader.ReadRune()
		if err != nil {
			if err == io.EOF {
				if t.buf.Len() > 0 {
					token := t.buf.String()
					t.buf.Reset()
					t.done = true
					return token, true, nil
				}
				t.done = true
				return "", true, nil
			}
			return "", true, err
		}
		if unicode.IsSpace(r) {
			if t.buf.Len() > 0 {
				token := t.buf.String()
				t.buf.Reset()
				return token, false, nil
			}
			continue
		}
		t.buf.WriteRune(r)
	}
}

func tokenizeCmd(t *tokenizer) tea.Cmd {
	return func() tea.Msg {
		word, done, err := t.next()
		return tokenMsg{word: word, done: done, err: err}
	}
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

func (m *model) closeInput() {
	if m.inputCloser != nil {
		_ = m.inputCloser.Close()
		m.inputCloser = nil
	}
}

func openInput(filePath string) (io.ReadCloser, error) {
	if filePath != "" {
		file, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		return file, nil
	}

	info, err := os.Stdin.Stat()
	if err != nil {
		return nil, err
	}

	// If stdin is a terminal (not a pipe/file), treat it as "no input provided".
	if info.Mode()&os.ModeCharDevice != 0 {
		return nil, fmt.Errorf("no input provided")
	}

	return io.NopCloser(os.Stdin), nil
}

func (m *model) restartStream() tea.Cmd {
	m.streamErr = nil
	m.streamDone = false
	m.waitingToken = false
	m.pendingAdvance = false
	m.hasCurrent = false
	m.currentWord = ""
	m.idx = -1
	m.closeInput()
	reader, err := openInput(m.filePath)
	if err != nil {
		m.streamErr = err
		m.streamDone = true
		return nil
	}
	m.inputCloser = reader
	m.tokenizer = newTokenizer(reader)
	return m.requestToken(true)
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

	reader, err := openInput(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Provide input via -file or stdin.")
		flag.PrintDefaults()
		os.Exit(1)
	}

	startIdx := 0
	if lazy {
		startIdx = -1
	}
	p := tea.NewProgram(model{
		wpm:         startWPM,
		tokenizer:   newTokenizer(reader),
		inputCloser: reader,
		lazy:        lazy,
		idx:         startIdx,
		filePath:    file,
	})
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
