package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestWrapLines_empty(t *testing.T) {
	if got := wrapLines("", 40, 4); got != "" {
		t.Errorf("empty input: got %q, want empty", got)
	}
}

func TestWrapLines_zeroWidth(t *testing.T) {
	in := "hello world this is a line"
	if got := wrapLines(in, 0, 4); got != in {
		t.Errorf("width=0: got %q, want %q", got, in)
	}
	if got := wrapLines(in, -1, 4); got != in {
		t.Errorf("width<0: got %q, want %q", got, in)
	}
}

func TestWrapLines_indentGEWidth(t *testing.T) {
	in := "hello world"
	if got := wrapLines(in, 10, 10); got != in {
		t.Errorf("indent>=width: got %q, want %q", got, in)
	}
	if got := wrapLines(in, 10, 20); got != in {
		t.Errorf("indent>width: got %q, want %q", got, in)
	}
}

func TestWrapLines_shortLineUnchanged(t *testing.T) {
	in := "short line"
	if got := wrapLines(in, 40, 4); got != in {
		t.Errorf("short line modified: got %q, want %q", got, in)
	}
}

func TestWrapLines_exactWidth(t *testing.T) {
	in := strings.Repeat("x", 40)
	if got := wrapLines(in, 40, 4); got != in {
		t.Errorf("exact-width line wrapped: got %q, want %q", got, in)
	}
}

func TestWrapLines_wrapsWithHangingIndent(t *testing.T) {
	// Line is clearly over 20 cells; expect a wrap with continuation indented 4.
	in := "one two three four five six seven eight"
	got := wrapLines(in, 20, 4)
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d: %q", len(lines), got)
	}
	if strings.HasPrefix(lines[0], " ") {
		t.Errorf("first line should not be indented: %q", lines[0])
	}
	for i, l := range lines[1:] {
		if !strings.HasPrefix(l, "    ") {
			t.Errorf("continuation line %d not indented by 4: %q", i+1, l)
		}
		if ansi.StringWidth(l) > 20 {
			t.Errorf("continuation line %d exceeds width 20: %q (width=%d)", i+1, l, ansi.StringWidth(l))
		}
	}
}

func TestWrapLines_preservesANSI(t *testing.T) {
	// Plain input wrapped at a small width; ansi.Wrap should leave the raw
	// characters intact (no escape codes to preserve here, but the roundtrip
	// should still produce valid text).
	red := "\x1b[31m"
	reset := "\x1b[0m"
	in := red + "this is a long message with styling" + reset
	got := wrapLines(in, 15, 3)
	// Stripped width of each line must be <= 15.
	for i, l := range strings.Split(got, "\n") {
		if w := ansi.StringWidth(l); w > 15 {
			t.Errorf("line %d exceeds width 15: %q (width=%d)", i, l, w)
		}
	}
	// The colour code should still appear somewhere in the output.
	if !strings.Contains(got, red) {
		t.Errorf("ANSI color code missing from output: %q", got)
	}
}

func TestWrapLines_multilineInput(t *testing.T) {
	in := "short\nanother very long line that needs to be wrapped aggressively\nshort again"
	got := wrapLines(in, 20, 4)
	lines := strings.Split(got, "\n")
	if lines[0] != "short" {
		t.Errorf("line 0 should be unchanged: got %q", lines[0])
	}
	if lines[len(lines)-1] != "short again" {
		t.Errorf("last line should be unchanged: got %q", lines[len(lines)-1])
	}
	// Middle: first line of middle should be un-indented, any continuations
	// should be indented by 4.
	foundContinuation := false
	for _, l := range lines[1 : len(lines)-1] {
		if strings.HasPrefix(l, "    ") {
			foundContinuation = true
			if w := ansi.StringWidth(l); w > 20 {
				t.Errorf("indented continuation exceeds width 20: %q (width=%d)", l, w)
			}
		}
	}
	if !foundContinuation {
		t.Errorf("expected at least one indented continuation in output: %q", got)
	}
}

func TestWrapLines_longUnbreakableToken(t *testing.T) {
	// A token longer than the wrap width must not cause an infinite loop or
	// produce lines exceeding the width; ansi.Wrap hard-breaks it.
	in := "prefix thisisaverylongunbreakabletoken suffix"
	got := wrapLines(in, 10, 2)
	for i, l := range strings.Split(got, "\n") {
		if w := ansi.StringWidth(l); w > 10 {
			t.Errorf("line %d exceeds width 10: %q (width=%d)", i, l, w)
		}
	}
}
