package main

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"
)

type stream interface {
	Init() tea.Cmd
	Handle(tea.Msg) tea.Cmd
	Current() (string, bool)
	Next() tea.Cmd
	Prev()
	Restart() tea.Cmd
	SupportsSeek() bool
	SupportsRestart() bool
	CanAdvance() bool
	Err() error
	Pos() int
	Total() (bool, int)
}

type eagerStream struct {
	words           []string
	idx             int
	supportsRestart bool
}

type streamInitError struct {
	msg       string
	showUsage bool
}

func (e streamInitError) Error() string {
	return e.msg
}

func buildStream(lazy bool, filePath string) (stream, error) {
	if lazy {
		reader, err := openInput(filePath)
		if err != nil {
			return nil, streamInitError{
				msg:       "Provide input via -file or stdin.",
				showUsage: true,
			}
		}
		return newLazyStream(reader, filePath), nil
	}

	text, err := readInput(filePath)
	if err != nil {
		return nil, streamInitError{
			msg:       "Provide input via -file or stdin.",
			showUsage: true,
		}
	}
	words := tokenize(text)
	if len(words) == 0 {
		return nil, streamInitError{
			msg:       "No words found in input.",
			showUsage: false,
		}
	}
	return newEagerStream(words, filePath != ""), nil
}

func newEagerStream(words []string, supportsRestart bool) *eagerStream {
	return &eagerStream{words: words, supportsRestart: supportsRestart}
}

func (s *eagerStream) Init() tea.Cmd {
	return nil
}

func (s *eagerStream) Handle(tea.Msg) tea.Cmd {
	return nil
}

func (s *eagerStream) Current() (string, bool) {
	if len(s.words) == 0 || s.idx < 0 || s.idx >= len(s.words) {
		return "", false
	}
	return s.words[s.idx], true
}

func (s *eagerStream) Next() tea.Cmd {
	if s.idx < len(s.words)-1 {
		s.idx++
	}
	return nil
}

func (s *eagerStream) Prev() {
	if s.idx > 0 {
		s.idx--
	}
}

func (s *eagerStream) Restart() tea.Cmd {
	if s.supportsRestart {
		s.idx = 0
	}
	return nil
}

func (s *eagerStream) SupportsSeek() bool {
	return true
}

func (s *eagerStream) SupportsRestart() bool {
	return s.supportsRestart
}

func (s *eagerStream) CanAdvance() bool {
	return len(s.words) > 0 && s.idx < len(s.words)-1
}

func (s *eagerStream) Err() error {
	return nil
}

func (s *eagerStream) Pos() int {
	if len(s.words) == 0 {
		return -1
	}
	return s.idx
}

func (s *eagerStream) Total() (bool, int) {
	return true, len(s.words)
}

type lazyStream struct {
	tokenizer       *tokenizer
	inputCloser     io.Closer
	filePath        string
	done            bool
	err             error
	waitingToken    bool
	hasCurrent      bool
	currentWord     string
	idx             int
	total           int
	supportsRestart bool
}

func newLazyStream(reader io.ReadCloser, filePath string) *lazyStream {
	return &lazyStream{
		tokenizer:       newTokenizer(reader),
		inputCloser:     reader,
		filePath:        filePath,
		idx:             -1,
		supportsRestart: filePath != "",
	}
}

func (s *lazyStream) Init() tea.Cmd {
	if s.tokenizer == nil {
		return nil
	}
	return s.requestToken()
}

func (s *lazyStream) Handle(msg tea.Msg) tea.Cmd {
	tm, ok := msg.(tokenMsg)
	if !ok {
		return nil
	}
	s.waitingToken = false
	if tm.err != nil {
		s.err = tm.err
		s.done = true
		s.closeInput()
		return nil
	}
	if tm.word == "" && tm.done {
		s.done = true
		s.total = s.idx + 1
		s.closeInput()
		return nil
	}
	if tm.word != "" {
		s.idx++
		s.hasCurrent = true
		s.currentWord = tm.word
	}
	if tm.done {
		s.done = true
		s.total = s.idx + 1
		s.closeInput()
	}
	return nil
}

func (s *lazyStream) Current() (string, bool) {
	if !s.hasCurrent {
		return "", false
	}
	return s.currentWord, true
}

func (s *lazyStream) Next() tea.Cmd {
	if s.done {
		return nil
	}
	return s.requestToken()
}

func (s *lazyStream) Prev() {
	panic("lazyStream Prev not supported")
}

func (s *lazyStream) Restart() tea.Cmd {
	if !s.supportsRestart {
		return nil
	}
	s.resetState()
	reader, err := openInput(s.filePath)
	if err != nil {
		s.err = err
		s.done = true
		return nil
	}
	s.inputCloser = reader
	s.tokenizer = newTokenizer(reader)
	return s.requestToken()
}

func (s *lazyStream) SupportsSeek() bool {
	return false
}

func (s *lazyStream) SupportsRestart() bool {
	return s.supportsRestart
}

func (s *lazyStream) CanAdvance() bool {
	return !s.done
}

func (s *lazyStream) Err() error {
	return s.err
}

func (s *lazyStream) Pos() int {
	if !s.hasCurrent {
		return -1
	}
	return s.idx
}

func (s *lazyStream) Total() (bool, int) {
	if s.done {
		return true, s.total
	}
	return false, 0
}

func (s *lazyStream) requestToken() tea.Cmd {
	if s.waitingToken || s.tokenizer == nil {
		return nil
	}
	s.waitingToken = true
	return tokenizeCmd(s.tokenizer)
}

func (s *lazyStream) closeInput() {
	if s.inputCloser != nil {
		_ = s.inputCloser.Close()
		s.inputCloser = nil
	}
}

func (s *lazyStream) resetState() {
	s.done = false
	s.err = nil
	s.waitingToken = false
	s.hasCurrent = false
	s.currentWord = ""
	s.idx = -1
	s.total = 0
	s.closeInput()
}
