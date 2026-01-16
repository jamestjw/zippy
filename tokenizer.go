package main

import (
	"bufio"
	"io"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
)

type tokenMsg struct {
	word string
	done bool
	err  error
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
