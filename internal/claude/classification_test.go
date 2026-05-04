package claude

import (
	"context"
	"strings"
	"testing"

	"github.com/mab-go/golem/internal/grpc/pb"
)

func TestClassifyEventsEmptyShortCircuits(t *testing.T) {
	c := &Client{}
	got, err := c.ClassifyEvents(context.Background(), nil)
	if err != nil {
		t.Fatalf("empty input should short-circuit, got err=%v", err)
	}
	if got != nil {
		t.Errorf("expected nil result, got %+v", got)
	}
}

func TestClassifyEventsRequestShapesPayload(t *testing.T) {
	events := []*pb.GameEvent{
		{
			Type:        pb.EventType_EVENT_BOT_DEATH,
			Timestamp:   1000,
			Priority:    pb.EventPriority_EVENT_PRIORITY_CRITICAL,
			Description: "bot fell in lava",
		},
		{
			Type:        pb.EventType_EVENT_CHAT_MESSAGE,
			Timestamp:   2000,
			Priority:    pb.EventPriority_EVENT_PRIORITY_NORMAL,
			Description: "alice: hi golem",
			PlayerName:  "alice",
		},
	}
	parts, msgs := classifyEventsRequest(events)

	if parts.Stable == "" {
		t.Error("stable system prompt missing")
	}
	for _, want := range []string{"JSON array", "priority", "critical"} {
		if !strings.Contains(parts.Stable, want) {
			t.Errorf("system prompt missing %q", want)
		}
	}
	if len(msgs) != 1 || len(msgs[0].Content) != 1 {
		t.Fatalf("unexpected message shape: %+v", msgs)
	}
	body := msgs[0].Content[0].Text
	if !strings.Contains(body, "[0]") || !strings.Contains(body, "[1]") {
		t.Errorf("expected indices in body, got:\n%s", body)
	}
	if !strings.Contains(body, "bot fell in lava") || !strings.Contains(body, "alice") {
		t.Errorf("event content missing from body:\n%s", body)
	}
}

func TestMergeClassificationsHappyPath(t *testing.T) {
	events := []*pb.GameEvent{
		{Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_LOW},
		{Type: pb.EventType_EVENT_BOT_DEATH, Priority: pb.EventPriority_EVENT_PRIORITY_NORMAL},
		{Type: pb.EventType_EVENT_CHAT_MESSAGE, Priority: pb.EventPriority_EVENT_PRIORITY_NORMAL},
	}
	resp := `[
	  {"index": 0, "priority": "low", "reason": "routine spawn"},
	  {"index": 1, "priority": "critical", "reason": "died in lava"},
	  {"index": 2, "priority": "high", "reason": "addressed to bot"}
	]`

	got := mergeClassifications(events, resp)
	if len(got) != 3 {
		t.Fatalf("expected 3 classified events, got %d", len(got))
	}
	if got[0].Priority != pb.EventPriority_EVENT_PRIORITY_LOW {
		t.Errorf("row 0: want LOW, got %v", got[0].Priority)
	}
	if got[1].Priority != pb.EventPriority_EVENT_PRIORITY_CRITICAL {
		t.Errorf("row 1: want CRITICAL, got %v", got[1].Priority)
	}
	if got[1].Reason != "died in lava" {
		t.Errorf("row 1: want reason 'died in lava', got %q", got[1].Reason)
	}
	if got[2].Priority != pb.EventPriority_EVENT_PRIORITY_HIGH {
		t.Errorf("row 2: want HIGH, got %v", got[2].Priority)
	}
}

func TestMergeClassificationsToleratesProseWrapping(t *testing.T) {
	events := []*pb.GameEvent{
		{Type: pb.EventType_EVENT_BOT_DEATH, Priority: pb.EventPriority_EVENT_PRIORITY_NORMAL},
	}
	// Model emits prose before and after the array.
	resp := `Sure, here you go:
	[{"index": 0, "priority": "critical", "reason": "fatal"}]
	Let me know if you need more.`
	got := mergeClassifications(events, resp)
	if got[0].Priority != pb.EventPriority_EVENT_PRIORITY_CRITICAL {
		t.Errorf("want CRITICAL, got %v", got[0].Priority)
	}
}

func TestMergeClassificationsFallsBackOnMalformedJSON(t *testing.T) {
	events := []*pb.GameEvent{
		{Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_HIGH},
	}
	got := mergeClassifications(events, "not even remotely json")
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	if got[0].Priority != pb.EventPriority_EVENT_PRIORITY_HIGH {
		t.Errorf("expected fallback to original HIGH priority, got %v", got[0].Priority)
	}
}

func TestMergeClassificationsIgnoresOutOfRangeIndex(t *testing.T) {
	events := []*pb.GameEvent{
		{Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_LOW},
	}
	resp := `[
	  {"index": 0, "priority": "high", "reason": "ok"},
	  {"index": 5, "priority": "critical", "reason": "bogus"}
	]`
	got := mergeClassifications(events, resp)
	if got[0].Priority != pb.EventPriority_EVENT_PRIORITY_HIGH {
		t.Errorf("want HIGH, got %v", got[0].Priority)
	}
}

func TestMergeClassificationsIgnoresUnknownPriority(t *testing.T) {
	events := []*pb.GameEvent{
		{Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_NORMAL},
	}
	resp := `[{"index": 0, "priority": "URGENT", "reason": "bad value"}]`
	got := mergeClassifications(events, resp)
	if got[0].Priority != pb.EventPriority_EVENT_PRIORITY_NORMAL {
		t.Errorf("expected fallback to NORMAL, got %v", got[0].Priority)
	}
}

func TestFallbackClassifiedPreservesLength(t *testing.T) {
	events := []*pb.GameEvent{
		{Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_LOW},
		nil,
		{Type: pb.EventType_EVENT_BOT_DEATH, Priority: pb.EventPriority_EVENT_PRIORITY_CRITICAL},
	}
	got := fallbackClassified(events)
	if len(got) != 3 {
		t.Fatalf("expected 3, got %d", len(got))
	}
	if got[0].Priority != pb.EventPriority_EVENT_PRIORITY_LOW {
		t.Errorf("row 0: got %v", got[0].Priority)
	}
	if got[2].Priority != pb.EventPriority_EVENT_PRIORITY_CRITICAL {
		t.Errorf("row 2: got %v", got[2].Priority)
	}
}
