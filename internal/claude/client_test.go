package claude

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	sdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/mab-go/golem/internal/logging"
)

func TestSystemPromptBlocks(t *testing.T) {
	if got := systemPromptBlocks(CacheableSystemPrompt{}); len(got) != 0 {
		t.Errorf("empty parts should return nil slice, got %d blocks", len(got))
	}
	if got := systemPromptBlocks(CacheableSystemPrompt{Stable: "   ", Dynamic: "\t"}); len(got) != 0 {
		t.Errorf("whitespace-only parts should return nil slice")
	}

	onlyStable := systemPromptBlocks(CacheableSystemPrompt{Stable: "be helpful"})
	if len(onlyStable) != 1 || onlyStable[0].Text != "be helpful" {
		t.Errorf("unexpected stable-only blocks: %#v", onlyStable)
	}
	if onlyStable[0].CacheControl.Type == "" {
		t.Errorf("stable-only block should carry cache control, got %#v", onlyStable[0].CacheControl)
	}

	both := systemPromptBlocks(CacheableSystemPrompt{Stable: "persona", Dynamic: "runtime"})
	if len(both) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(both))
	}
	if both[0].CacheControl.Type == "" {
		t.Errorf("stable block missing cache control: %#v", both[0].CacheControl)
	}
	if both[1].CacheControl.Type != "" {
		t.Errorf("dynamic block should NOT carry cache control, got %#v", both[1].CacheControl)
	}
}

func TestSystemPromptBlocksMarshalsTwoCacheMarkers(t *testing.T) {
	blocks := systemPromptBlocks(CacheableSystemPrompt{Stable: "persona", Dynamic: "runtime"})
	data, err := json.Marshal(blocks)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got := strings.Count(string(data), `"cache_control":{"type":"ephemeral"}`); got != 1 {
		t.Errorf("system prompt blocks: expected exactly one cache_control marker, got %d\n%s", got, data)
	}
}

func TestToSDKToolsCachesLastToolOnly(t *testing.T) {
	in := []Tool{
		{Name: "a", InputSchema: map[string]any{"properties": map[string]any{}}},
		{Name: "b", InputSchema: map[string]any{"properties": map[string]any{}}},
		{Name: "c", InputSchema: map[string]any{"properties": map[string]any{}}},
	}
	tools := toSDKTools(in)
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}
	data, err := json.Marshal(tools)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got := strings.Count(string(data), `"cache_control":{"type":"ephemeral"}`); got != 1 {
		t.Errorf("expected exactly one cache_control marker on the last tool, got %d\n%s", got, data)
	}
	if !strings.Contains(string(data), `"name":"c","cache_control"`) &&
		!strings.Contains(string(data), `"cache_control":{"type":"ephemeral"},"type":"custom"`) {
		// Field ordering in marshaled JSON isn't guaranteed. We assert the
		// last tool 'c' carries the marker by checking that tools before it
		// do not.
		for _, name := range []string{"a", "b"} {
			segment := findToolSegment(string(data), name)
			if strings.Contains(segment, "cache_control") {
				t.Errorf("tool %q should not carry cache_control, got: %s", name, segment)
			}
		}
	}
}

// findToolSegment returns the substring of a ToolUnionParam JSON array that
// corresponds to the tool with the given name. Useful for per-tool assertions
// without depending on field order.
func findToolSegment(s, name string) string {
	needle := `"name":"` + name + `"`
	idx := strings.Index(s, needle)
	if idx < 0 {
		return ""
	}
	// Walk forward to the next "}," or final "}]".
	end := idx
	for end < len(s) && s[end] != '}' {
		end++
	}
	return s[idx:end]
}

func TestToSDKMessages(t *testing.T) {
	in := []Message{
		{Role: RoleUser, Content: []Block{{Type: BlockText, Text: "hi"}}},
		{Role: RoleAssistant, Content: []Block{
			{Type: BlockText, Text: "sure"},
			{Type: BlockToolUse, ID: "tu_1", Name: "look_around", Input: json.RawMessage(`{"radius":16}`)},
		}},
		{Role: RoleUser, Content: []Block{
			{Type: BlockToolResult, ToolUseID: "tu_1", Content: "clear"},
		}},
	}
	out := toSDKMessages(in)
	if len(out) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(out))
	}
	// Round-trip to JSON to verify structure (content param marshaling is union-based).
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	for _, want := range []string{
		`"role":"user"`, `"role":"assistant"`,
		`"type":"tool_use"`, `"type":"tool_result"`,
		`"name":"look_around"`, `"tool_use_id":"tu_1"`,
		`"radius":16`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("marshaled messages missing %q\n%s", want, s)
		}
	}
}

func TestToSDKMessagesCachesLastBlockOnly(t *testing.T) {
	// Moving cache breakpoint: the final block of the final message carries
	// an ephemeral cache_control marker; no earlier block does. On the next
	// turn the breakpoint advances, giving cumulative cache hits on the
	// growing conversation prefix.
	in := []Message{
		{Role: RoleUser, Content: []Block{{Type: BlockText, Text: "turn 1 perception"}}},
		{Role: RoleAssistant, Content: []Block{
			{Type: BlockText, Text: "thinking"},
			{Type: BlockToolUse, ID: "tu_1", Name: "look", Input: json.RawMessage(`{}`)},
		}},
		{Role: RoleUser, Content: []Block{
			{Type: BlockToolResult, ToolUseID: "tu_1", Content: "ok"},
		}},
	}
	out := toSDKMessages(in)
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)
	if got := strings.Count(s, `"cache_control":{"type":"ephemeral"}`); got != 1 {
		t.Fatalf("expected exactly one cache_control marker on the last block, got %d\n%s", got, s)
	}
	// The single marker must fall on (or after) the tool_result block --
	// i.e. the last block of the last message. Earlier blocks
	// (turn 1 perception text, the tool_use) must not carry it.
	trIdx := strings.Index(s, `"tool_use_id":"tu_1"`)
	if trIdx < 0 {
		t.Fatalf("tool_result for tu_1 missing from marshaled output: %s", s)
	}
	ccIdx := strings.Index(s, `"cache_control"`)
	if ccIdx < trIdx {
		t.Errorf("cache_control appears before the final tool_result; expected it on the last block\n%s", s)
	}
}

func TestToSDKMessagesEmptyInputIsSafe(t *testing.T) {
	if got := toSDKMessages(nil); len(got) != 0 {
		t.Errorf("nil input should yield empty slice, got %d", len(got))
	}
}

func TestResponseFromSDKMessage(t *testing.T) {
	raw := []byte(`{
		"id": "msg_123",
		"type": "message",
		"role": "assistant",
		"model": "claude-sonnet-4-6",
		"stop_reason": "tool_use",
		"content": [
			{"type": "text", "text": "I'll look around."},
			{"type": "tool_use", "id": "tu_42", "name": "look_around", "input": {"radius": 16}}
		],
		"usage": {"input_tokens": 150, "output_tokens": 75}
	}`)
	var msg sdk.Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	resp := responseFromSDKMessage(&msg)
	if resp.Text != "I'll look around." {
		t.Errorf("text=%q", resp.Text)
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("stop_reason=%q", resp.StopReason)
	}
	if len(resp.ToolUses) != 1 {
		t.Fatalf("expected 1 tool use, got %d", len(resp.ToolUses))
	}
	tu := resp.ToolUses[0]
	if tu.ID != "tu_42" || tu.Name != "look_around" {
		t.Errorf("tool use id/name: %+v", tu)
	}
	if !bytes.Contains(tu.Input, []byte(`"radius"`)) {
		t.Errorf("tool use input missing radius: %s", tu.Input)
	}
	if resp.Usage.InputTokens != 150 || resp.Usage.OutputTokens != 75 {
		t.Errorf("usage: %+v", resp.Usage)
	}
}

// fakeStream drives accumulate with a canned event sequence.
type fakeStream struct {
	events []sdk.MessageStreamEventUnion
	i      int
	err    error
}

func (f *fakeStream) Next() bool {
	if f.i >= len(f.events) {
		return false
	}
	f.i++
	return true
}

func (f *fakeStream) Current() sdk.MessageStreamEventUnion {
	return f.events[f.i-1]
}

func (f *fakeStream) Err() error { return f.err }

// streamEvent creates a MessageStreamEventUnion by unmarshaling JSON; this is
// the only way to populate the union's unexported JSON.raw field.
func streamEvent(t *testing.T, jsonStr string) sdk.MessageStreamEventUnion {
	t.Helper()
	var ev sdk.MessageStreamEventUnion
	if err := json.Unmarshal([]byte(jsonStr), &ev); err != nil {
		t.Fatalf("unmarshal stream event: %v\n%s", err, jsonStr)
	}
	return ev
}

func TestAccumulateTextAndToolUse(t *testing.T) {
	events := []sdk.MessageStreamEventUnion{
		streamEvent(t, `{"type":"message_start","message":{"id":"m_1","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[],"stop_reason":null,"usage":{"input_tokens":10,"output_tokens":0}}}`),
		streamEvent(t, `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
		streamEvent(t, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello "}}`),
		streamEvent(t, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"world"}}`),
		streamEvent(t, `{"type":"content_block_stop","index":0}`),
		streamEvent(t, `{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"tu_1","name":"look_around","input":{}}}`),
		streamEvent(t, `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"radius\":16}"}}`),
		streamEvent(t, `{"type":"content_block_stop","index":1}`),
		streamEvent(t, `{"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":42}}`),
		streamEvent(t, `{"type":"message_stop"}`),
	}
	var deltas []string
	deltaFn := func(s string) { deltas = append(deltas, s) }

	c := &Client{log: logging.NewDefaultLogger()}
	resp, err := c.accumulate(&fakeStream{events: events}, deltaFn)
	if err != nil {
		t.Fatalf("accumulate: %v", err)
	}
	if resp.Text != "hello world" {
		t.Errorf("text=%q", resp.Text)
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("stop_reason=%q", resp.StopReason)
	}
	if len(resp.ToolUses) != 1 {
		t.Fatalf("tool_uses=%d", len(resp.ToolUses))
	}
	if tu := resp.ToolUses[0]; tu.Name != "look_around" || tu.ID != "tu_1" {
		t.Errorf("tool use: %+v", tu)
	}
	if !strings.Contains(string(resp.ToolUses[0].Input), `"radius":16`) {
		t.Errorf("input missing radius: %s", resp.ToolUses[0].Input)
	}
	if wantDeltas := []string{"hello ", "world"}; !slices.Equal(deltas, wantDeltas) {
		t.Errorf("deltas=%v, want %v", deltas, wantDeltas)
	}
}

func TestAccumulateSilent(t *testing.T) {
	events := []sdk.MessageStreamEventUnion{
		streamEvent(t, `{"type":"message_start","message":{"id":"m_1","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[],"stop_reason":null,"usage":{"input_tokens":10,"output_tokens":0}}}`),
		streamEvent(t, `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
		streamEvent(t, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`),
		streamEvent(t, `{"type":"content_block_stop","index":0}`),
		streamEvent(t, `{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":5}}`),
		streamEvent(t, `{"type":"message_stop"}`),
	}
	called := false
	c := &Client{
		log:           logging.NewDefaultLogger(),
		TextDeltaFunc: func(string) { called = true },
	}
	resp, err := c.accumulate(&fakeStream{events: events}, func(string) {})
	if err != nil {
		t.Fatalf("accumulate: %v", err)
	}
	if resp.Text != "hello" {
		t.Errorf("text=%q", resp.Text)
	}
	if called {
		t.Error("client-level TextDeltaFunc should not be called when per-call func is provided")
	}
}

func TestAccumulateNilDeltaFunc(t *testing.T) {
	events := []sdk.MessageStreamEventUnion{
		streamEvent(t, `{"type":"message_start","message":{"id":"m_1","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[],"stop_reason":null,"usage":{"input_tokens":10,"output_tokens":0}}}`),
		streamEvent(t, `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
		streamEvent(t, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`),
		streamEvent(t, `{"type":"content_block_stop","index":0}`),
		streamEvent(t, `{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":5}}`),
		streamEvent(t, `{"type":"message_stop"}`),
	}
	c := &Client{log: logging.NewDefaultLogger()}
	resp, err := c.accumulate(&fakeStream{events: events}, nil)
	if err != nil {
		t.Fatalf("accumulate: %v", err)
	}
	if resp.Text != "hello" {
		t.Errorf("text=%q", resp.Text)
	}
}
