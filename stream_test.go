package main

import (
	"io"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func runCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected cmd, got nil")
	}
	return cmd()
}

func TestEagerStreamBasics(t *testing.T) {
	s := newEagerStream([]string{"alpha", "beta"}, true)
	if got, ok := s.Current(); !ok || got != "alpha" {
		t.Fatalf("expected first word, got %q ok=%v", got, ok)
	}
	if !s.CanAdvance() {
		t.Fatalf("expected stream to be able to advance")
	}
	if !s.SupportsSeek() || !s.SupportsRestart() {
		t.Fatalf("expected seek/restart support")
	}
	if known, total := s.Total(); !known || total != 2 {
		t.Fatalf("expected total 2, got %v/%d", known, total)
	}

	s.Next()
	if got, _ := s.Current(); got != "beta" {
		t.Fatalf("expected second word, got %q", got)
	}
	if s.CanAdvance() {
		t.Fatalf("expected no further advance")
	}

	s.Prev()
	if got, _ := s.Current(); got != "alpha" {
		t.Fatalf("expected first word after prev, got %q", got)
	}

	s.Restart()
	if got, _ := s.Current(); got != "alpha" {
		t.Fatalf("expected first word after restart, got %q", got)
	}
}

func TestLazyStreamFlow(t *testing.T) {
	s := newLazyStream(io.NopCloser(strings.NewReader("one two")), "")
	msg := runCmd(t, s.Init())
	s.Handle(msg)
	if got, ok := s.Current(); !ok || got != "one" {
		t.Fatalf("expected first word, got %q ok=%v", got, ok)
	}
	if !s.CanAdvance() {
		t.Fatalf("expected stream to be able to advance")
	}
	if s.Pos() != 0 {
		t.Fatalf("expected position 0, got %d", s.Pos())
	}

	msg = runCmd(t, s.Next())
	s.Handle(msg)
	if got, ok := s.Current(); !ok || got != "two" {
		t.Fatalf("expected second word, got %q ok=%v", got, ok)
	}
	if s.Pos() != 1 {
		t.Fatalf("expected position 1, got %d", s.Pos())
	}
	if s.CanAdvance() {
		t.Fatalf("expected stream to be done")
	}
	if known, total := s.Total(); !known || total != 2 {
		t.Fatalf("expected total 2 after EOF, got %v/%d", known, total)
	}
}

func TestLazyStreamPrevPanics(t *testing.T) {
	s := newLazyStream(io.NopCloser(strings.NewReader("one")), "")
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from Prev")
		}
	}()
	s.Prev()
}

func TestLazyStreamRestartFile(t *testing.T) {
	tmp, err := os.CreateTemp("", "zippy-stream-*.txt")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(tmp.Name()) })
	if _, err := tmp.WriteString("hello world"); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("close temp: %v", err)
	}

	reader, err := os.Open(tmp.Name())
	if err != nil {
		t.Fatalf("open temp: %v", err)
	}
	s := newLazyStream(reader, tmp.Name())

	msg := runCmd(t, s.Init())
	s.Handle(msg)
	if got, ok := s.Current(); !ok || got != "hello" {
		t.Fatalf("expected first word, got %q ok=%v", got, ok)
	}

	msg = runCmd(t, s.Restart())
	s.Handle(msg)
	if got, ok := s.Current(); !ok || got != "hello" {
		t.Fatalf("expected first word after restart, got %q ok=%v", got, ok)
	}
}
