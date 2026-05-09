package claude

import (
	"context"
	"strings"
	"testing"
)

func TestCompactJournalRejectsEmptyEntries(t *testing.T) {
	c := &Client{}
	for _, input := range []string{"", "   ", "\n\t"} {
		if _, err := c.CompactJournal(context.Background(), input); err == nil {
			t.Errorf("expected error for empty entries %q, got nil", input)
		}
	}
}

func TestCompactJournalRequestShapesPayload(t *testing.T) {
	parts, msgs := compactJournalRequest("## 2026-04-16\n\nMet a zombie. Won.")

	if parts.Stable == "" {
		t.Error("stable system prompt must not be empty")
	}
	for _, want := range []string{"compact", "first-person", "milestones"} {
		if !strings.Contains(strings.ToLower(parts.Stable), want) {
			t.Errorf("system prompt missing expected keyword %q:\n%s", want, parts.Stable)
		}
	}
	if parts.Dynamic != "" {
		t.Errorf("Dynamic should be empty so the whole prompt is cacheable, got %q", parts.Dynamic)
	}
	if len(msgs) != 1 || msgs[0].Role != RoleUser {
		t.Fatalf("expected one user message, got %+v", msgs)
	}
	if len(msgs[0].Content) != 1 || msgs[0].Content[0].Type != BlockText {
		t.Fatalf("expected a single text block, got %+v", msgs[0].Content)
	}
	body := msgs[0].Content[0].Text
	if !strings.Contains(body, "zombie") {
		t.Errorf("user message should embed the journal entries, got:\n%s", body)
	}
}

func TestCompactJournalHappyPath(t *testing.T) {
	ModelWriter = "test-writer"
	t.Cleanup(func() { ModelWriter = "" })

	c := newTestClient(t, textResponseEvents(t, "Explored a cave and found iron."))
	got, err := c.CompactJournal(context.Background(), "## Day 1\n\nWent into cave. Found iron ore.")
	if err != nil {
		t.Fatalf("CompactJournal: %v", err)
	}
	if got != "Explored a cave and found iron." {
		t.Errorf("got %q", got)
	}
}
