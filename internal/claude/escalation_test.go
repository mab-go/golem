package claude

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestThinkDeeplyUsesModelDeep(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set(ViperKeyModelDeep, "test-opus-model")
	InitModels()

	if ModelDeep != "test-opus-model" {
		t.Fatalf("ModelDeep = %q, want test-opus-model", ModelDeep)
	}
}

func TestEscalationSystemPromptIsNonEmpty(t *testing.T) {
	if strings.TrimSpace(escalationSystemPrompt) == "" {
		t.Error("escalation system prompt should not be empty")
	}
	if !strings.Contains(escalationSystemPrompt, "strategic") {
		t.Error("escalation prompt should mention 'strategic'")
	}
}

func TestToolCatalogIncludesThinkDeeply(t *testing.T) {
	tools := Tools()
	found := false
	for _, tool := range tools {
		if tool.Name == ToolThinkDeeply {
			found = true
			if tool.Description == "" {
				t.Error("think_deeply tool should have a description")
			}
			props, ok := tool.InputSchema["properties"].(map[string]any)
			if !ok {
				t.Fatal("think_deeply should have properties in its schema")
			}
			if _, hasSituation := props["situation"]; !hasSituation {
				t.Error("think_deeply schema should require a 'situation' property")
			}
			req, ok := tool.InputSchema["required"].([]string)
			if !ok || len(req) == 0 {
				t.Error("think_deeply should have 'situation' as required")
			}
			break
		}
	}
	if !found {
		t.Error("Tools() should include think_deeply")
	}
}

func TestToolsByNameIncludesThinkDeeply(t *testing.T) {
	m := ToolsByName()
	if _, ok := m[ToolThinkDeeply]; !ok {
		t.Error("ToolsByName() should include think_deeply")
	}
}
