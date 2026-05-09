package claude

import (
	"context"
	"strings"
	"testing"
)

func TestSynthesizeKnowledgeRejectsEmptyObservations(t *testing.T) {
	c := &Client{}
	for _, input := range []string{"", "   ", "\n\t"} {
		if _, err := c.SynthesizeKnowledge(context.Background(), input); err == nil {
			t.Errorf("expected error for empty input %q, got nil", input)
		}
	}
}

func TestSynthesizeKnowledgeRequestShapesPayload(t *testing.T) {
	parts, msgs := synthesizeKnowledgeRequest("village at 142,68,-231; iron vein at 90,40,0")

	if parts.Stable == "" {
		t.Error("stable system prompt must not be empty")
	}
	for _, want := range []string{"world-knowledge", "coordinates", "hostile"} {
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
	if !strings.Contains(body, "village at") {
		t.Errorf("user message should embed the observations, got:\n%s", body)
	}
}

func TestSynthesizeKnowledgeHappyPath(t *testing.T) {
	ModelWriter = "test-writer"
	t.Cleanup(func() { ModelWriter = "" })

	c := newTestClient(t, textResponseEvents(t, "Village at 142,68,-231 with iron vein nearby."))
	got, err := c.SynthesizeKnowledge(context.Background(), "village at 142,68,-231; iron vein at 90,40,0")
	if err != nil {
		t.Fatalf("SynthesizeKnowledge: %v", err)
	}
	if got != "Village at 142,68,-231 with iron vein nearby." {
		t.Errorf("got %q", got)
	}
}
