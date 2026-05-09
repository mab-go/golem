package claude

import (
	"strings"
	"testing"
)

func TestSystemPromptParts(t *testing.T) {
	cfg := SystemPromptConfig{
		Verbosity:        "verbose",
		PerceptionFormat: "structured",
		BotUsername:      "testbot",
		MemoryDir:        "/tmp/mem",
	}
	parts := SystemPromptParts(cfg)

	if !strings.Contains(parts.Stable, "You are Claude") {
		t.Error("Stable should contain persona identity")
	}
	if !strings.Contains(parts.Stable, "How you perceive the world") {
		t.Error("Stable should contain perception legend")
	}
	if !strings.Contains(parts.Stable, "What you can do") {
		t.Error("Stable should contain tool reference")
	}

	for _, want := range []string{
		"verbosity: verbose",
		"perception format: structured",
		"in-game name: testbot",
		"memory directory: /tmp/mem",
	} {
		if !strings.Contains(parts.Dynamic, want) {
			t.Errorf("Dynamic missing %q:\n%s", want, parts.Dynamic)
		}
	}
}

func TestSystemPromptPartsDefaults(t *testing.T) {
	parts := SystemPromptParts(SystemPromptConfig{})

	for _, want := range []string{
		"verbosity: standard",
		"perception format: prose",
		"in-game name: claude",
		"memory directory: ./memory",
	} {
		if !strings.Contains(parts.Dynamic, want) {
			t.Errorf("Dynamic missing default %q:\n%s", want, parts.Dynamic)
		}
	}
}

func TestSystemPrompt(t *testing.T) {
	cfg := SystemPromptConfig{BotUsername: "bot"}
	parts := SystemPromptParts(cfg)
	joined := SystemPrompt(cfg)

	want := parts.Stable + "\n\n" + parts.Dynamic
	if joined != want {
		t.Error("SystemPrompt should equal Stable + newlines + Dynamic")
	}
}

func TestFallback(t *testing.T) {
	if got := fallback("custom", "default"); got != "custom" {
		t.Errorf("fallback non-empty = %q, want custom", got)
	}
	if got := fallback("", "default"); got != "default" {
		t.Errorf("fallback empty = %q, want default", got)
	}
	if got := fallback("   ", "default"); got != "default" {
		t.Errorf("fallback whitespace = %q, want default", got)
	}
}
