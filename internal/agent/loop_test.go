package agent

import (
	"strings"
	"testing"

	"github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/perception"
)

func TestBuildMidRoundTextEmpty(t *testing.T) {
	a := &Agent{
		formatter: perception.NewFormatter(perception.FormatProse, perception.VerbosityStandard),
	}
	if text := a.buildMidRoundText(); text != "" {
		t.Errorf("expected empty, got %q", text)
	}
}

func TestBuildMidRoundTextWithEvents(t *testing.T) {
	a := &Agent{
		formatter: perception.NewFormatter(perception.FormatProse, perception.VerbosityStandard),
	}
	a.appendEvent(&pb.GameEvent{
		Type:        pb.EventType_EVENT_CHAT_MESSAGE,
		Description: "alice: hello",
	})

	text := a.buildMidRoundText()
	if !strings.Contains(text, "[Mid-round update]") {
		t.Errorf("missing header, got %q", text)
	}
	if !strings.Contains(text, "New events since last tool call") {
		t.Errorf("missing events section, got %q", text)
	}
}

func TestBuildMidRoundTextWithChat(t *testing.T) {
	a := &Agent{
		formatter: perception.NewFormatter(perception.FormatProse, perception.VerbosityStandard),
	}
	a.appendChat("where are you?", "bob")

	text := a.buildMidRoundText()
	if !strings.Contains(text, "[Mid-round update]") {
		t.Errorf("missing header, got %q", text)
	}
	if !strings.Contains(text, "Player chat:") {
		t.Errorf("missing chat section, got %q", text)
	}
	if !strings.Contains(text, "bob") || !strings.Contains(text, "where are you?") {
		t.Errorf("missing chat content, got %q", text)
	}
}

func TestBuildMidRoundTextWithBoth(t *testing.T) {
	a := &Agent{
		formatter: perception.NewFormatter(perception.FormatProse, perception.VerbosityStandard),
	}
	a.appendEvent(&pb.GameEvent{
		Type:        pb.EventType_EVENT_WEATHER_CHANGE,
		Description: "it started raining",
	})
	a.appendChat("nice weather", "alice")

	text := a.buildMidRoundText()
	if !strings.Contains(text, "New events since last tool call") {
		t.Errorf("missing events section, got %q", text)
	}
	if !strings.Contains(text, "Player chat:") {
		t.Errorf("missing chat section, got %q", text)
	}
}

func TestBuildMidRoundTextDrains(t *testing.T) {
	a := &Agent{
		formatter: perception.NewFormatter(perception.FormatProse, perception.VerbosityStandard),
	}
	a.appendEvent(&pb.GameEvent{
		Type:        pb.EventType_EVENT_CHAT_MESSAGE,
		Description: "test",
	})
	a.appendChat("hello", "alice")

	first := a.buildMidRoundText()
	if first == "" {
		t.Fatal("first call should return content")
	}

	second := a.buildMidRoundText()
	if second != "" {
		t.Errorf("second call should return empty after drain, got %q", second)
	}
}
