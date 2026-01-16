package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

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
