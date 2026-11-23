package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHiddenTokenPasteAndCancellation(t *testing.T) {
	token := strings.Repeat("a", 43)
	for _, line := range []string{token + "\r", token + "\n", "b\b" + token + "\r"} {
		raw, err := readHiddenLine(strings.NewReader(line))
		if err != nil || !bytes.Equal(raw, []byte(token)) {
			t.Fatal("valid paste rejected")
		}
	}
	for _, line := range []string{token + "\x03", "abc\x04", token + "x\n", "short\r"} {
		raw, err := readHiddenLine(strings.NewReader(line))
		if err == nil || len(raw) != 0 {
			t.Fatal("invalid or cancelled input accepted")
		}
	}
}
