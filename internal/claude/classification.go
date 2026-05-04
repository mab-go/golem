package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/perception"
)

const eventClassificationSystemPrompt = `You triage a batch of Minecraft events that happened to an autonomous
agent named golem. For each event, assign one of four priorities:

- low: routine noise (weather changes, distant entity spawns, chatter not
  addressed to the bot)
- normal: worth noting but not urgent (a crafting completion, a non-hostile
  entity spawn, a chat between two other players)
- high: calls for the agent's attention (damage, low health, hostile mob
  nearby, chat addressed to the bot)
- critical: immediate survival concern (death, severe damage, player in
  danger near the bot)

Respond with ONLY a JSON array aligned with the input order. Each element
must have three fields:

  {"index": N, "priority": "low|normal|high|critical", "reason": "short why"}

Do not omit entries. Do not include any prose outside the JSON array.`

// ClassifyEvents asks ModelWorkhorse (Haiku) to re-rank a batch of events so
// downstream formatting can emphasize what matters. The returned slice is
// aligned with the input order.
//
// On empty input the function returns (nil, nil) without touching the SDK.
// On any model error -- transport failure, malformed response, out-of-range
// index -- it falls back to the events' original priorities so the loop can
// still render a user message.
func (c *Client) ClassifyEvents(
	ctx context.Context,
	events []*pb.GameEvent,
) ([]perception.ClassifiedEvent, error) {
	if len(events) == 0 {
		return nil, nil
	}

	parts, msgs := classifyEventsRequest(events)
	resp, err := c.SendMessageParts(ctx, ModelWorkhorse, parts, msgs, nil, Silent())
	if err != nil {
		return fallbackClassified(events), fmt.Errorf("classify events: %w", err)
	}
	return mergeClassifications(events, resp.Text), nil
}

// fallbackClassified wraps events with their existing priorities. Used when
// classification is skipped or fails so callers always receive a
// same-length result.
func fallbackClassified(events []*pb.GameEvent) []perception.ClassifiedEvent {
	out := make([]perception.ClassifiedEvent, len(events))
	for i, e := range events {
		priority := pb.EventPriority_EVENT_PRIORITY_NORMAL
		if e != nil {
			priority = e.Priority
		}
		out[i] = perception.ClassifiedEvent{Event: e, Priority: priority}
	}
	return out
}

// classifyEventsRequest builds the (system prompt, user messages) payload for
// a ClassifyEvents call. Split out for unit testing without a network round
// trip.
func classifyEventsRequest(events []*pb.GameEvent) (CacheableSystemPrompt, []Message) {
	parts := CacheableSystemPrompt{Stable: eventClassificationSystemPrompt}
	var body strings.Builder
	body.WriteString("Classify these events in order. Respond with a JSON array aligned by index.\n\n")
	for i, e := range events {
		if e == nil {
			fmt.Fprintf(&body, "[%d] (empty)\n", i)
			continue
		}
		desc := e.Description
		if desc == "" {
			desc = e.Type.String()
		}
		player := ""
		if e.PlayerName != "" {
			player = " player=" + e.PlayerName
		}
		fmt.Fprintf(&body, "[%d] type=%s priority=%s%s desc=%q\n",
			i, e.Type.String(), e.Priority.String(), player, desc)
	}
	msgs := []Message{{
		Role:    RoleUser,
		Content: []Block{{Type: BlockText, Text: body.String()}},
	}}
	return parts, msgs
}

// classificationRow mirrors a single element of the model's JSON response.
type classificationRow struct {
	Index    int    `json:"index"`
	Priority string `json:"priority"`
	Reason   string `json:"reason"`
}

// mergeClassifications parses the model's response text and returns a
// ClassifiedEvent per input event. Rows in the response that are missing or
// malformed fall back to the event's original priority.
func mergeClassifications(events []*pb.GameEvent, respText string) []perception.ClassifiedEvent {
	out := fallbackClassified(events)
	data, ok := extractJSONArray(respText)
	if !ok {
		return out
	}
	var rows []classificationRow
	if err := json.Unmarshal([]byte(data), &rows); err != nil {
		return out
	}
	for _, r := range rows {
		if r.Index < 0 || r.Index >= len(out) {
			continue
		}
		prio, ok := priorityFromString(r.Priority)
		if !ok {
			continue
		}
		out[r.Index].Priority = prio
		out[r.Index].Reason = r.Reason
	}
	return out
}

// extractJSONArray pulls the first bracketed JSON array out of a string. Haiku
// is instructed to return only JSON but occasionally wraps it in prose; this
// tolerates that without loosening the contract.
func extractJSONArray(s string) (string, bool) {
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start < 0 || end < 0 || end <= start {
		return "", false
	}
	return s[start : end+1], true
}

func priorityFromString(s string) (pb.EventPriority, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "low":
		return pb.EventPriority_EVENT_PRIORITY_LOW, true
	case "normal":
		return pb.EventPriority_EVENT_PRIORITY_NORMAL, true
	case "high":
		return pb.EventPriority_EVENT_PRIORITY_HIGH, true
	case "critical":
		return pb.EventPriority_EVENT_PRIORITY_CRITICAL, true
	default:
		return 0, false
	}
}
