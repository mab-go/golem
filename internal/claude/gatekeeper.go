package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/perception"
)

const gatekeeperSystemPrompt = `You are the attention filter for an autonomous Minecraft agent. Every few
seconds you receive a compact snapshot of the game state and recent events.
Your job: decide whether the agent's conscious mind should wake up and think.

Reply with ONLY a JSON object. No prose outside the JSON.

## Entity categories

Nearby entities are grouped by category:
- Players: real humans on the server -- never a threat, never trigger wake on their own
- Hostile: mobs that attack on sight -- wake if within ~8 blocks
- Neutral: mobs that only attack when provoked -- treat as passive unless you see damage
- Passive: harmless animals and villagers -- never a threat

## When to wake (wake: true)

- Player chat addressed to the bot or mentioning it
- Health dropped or taking damage
- Hostile mob within ~8 blocks (entities listed under "Hostile:")
- Food critically low (≤ 6)
- Task completed, failed, or cancelled
- Significant environment change (time of day shift, weather)
- New player joined or left

## When NOT to wake (wake: false)

- Players nearby -- players are never threats
- Routine entity spawns (passive/neutral mobs appearing/despawning)
- Distant sounds or particles
- No events at all and vitals are stable
- Task making normal incremental progress
- Events that are purely informational with no action needed

## Response format

{"wake": true/false, "reason": "short why", "events": [{"index": N, "priority": "low|normal|high|critical", "reason": "short why"}, ...]}

The events array classifies each input event by index. Include it only when
wake is true AND events are present. Omit it when wake is false or no events.`

// GatekeeperSnapshot is the compact input sent to Haiku for wake/no-wake
// decisions. It is much smaller than a full PerceptionSnapshot.
type GatekeeperSnapshot struct {
	Vitals         *pb.GetVitalSignsResponse
	Position       *pb.Vec3
	Biome          string
	TimeOfDay      string
	NearbyEntities []*pb.Entity
	ActiveTask     *perception.TaskStatus
	Events         []*pb.GameEvent
	TimeSinceWake  time.Duration
	GoalsSummary   string
}

// GatekeeperDecision is the parsed output of a gatekeeper call.
type GatekeeperDecision struct {
	Wake             bool
	Reason           string
	ClassifiedEvents []perception.ClassifiedEvent
}

type entityBucket struct {
	name    string
	count   int
	nearest float64
}

// GatekeeperCheck asks ModelWorkhorse (Haiku) whether the agent should wake.
// On any error it fails open (returns wake=true).
func (c *Client) GatekeeperCheck(ctx context.Context, snap GatekeeperSnapshot) (*GatekeeperDecision, error) {
	input := formatGatekeeperInput(snap)
	parts := CacheableSystemPrompt{Stable: gatekeeperSystemPrompt}
	msgs := []Message{{
		Role:    RoleUser,
		Content: []Block{{Type: BlockText, Text: input}},
	}}

	resp, err := c.SendMessageParts(ctx, ModelWorkhorse, parts, msgs, nil, Silent())
	if err != nil {
		return &GatekeeperDecision{Wake: true, Reason: "fail-open: API error"}, err
	}
	return parseGatekeeperResponse(resp.Text, snap.Events), nil
}

// formatGatekeeperInput renders the compact snapshot that Haiku evaluates.
func formatGatekeeperInput(snap GatekeeperSnapshot) string {
	var b strings.Builder
	writeVitals(&b, snap.Vitals)
	writePosition(&b, snap.Position, snap.Biome, snap.TimeOfDay)
	writeEntityBuckets(&b, snap.NearbyEntities)
	writeTask(&b, snap.ActiveTask)
	writeGoals(&b, snap.GoalsSummary)
	writeLastWake(&b, snap.TimeSinceWake)
	writeEventList(&b, snap.Events)
	return b.String()
}

func writeVitals(b *strings.Builder, v *pb.GetVitalSignsResponse) {
	if v != nil {
		fmt.Fprintf(b, "Vitals: HP %.0f/%.0f | food %d/20 | armor %d\n",
			v.Health, v.MaxHealth, v.Food, v.Armor)
	} else {
		b.WriteString("Vitals: unavailable\n")
	}
}

func writePosition(b *strings.Builder, pos *pb.Vec3, biome, timeOfDay string) {
	if pos != nil {
		fmt.Fprintf(b, "Position: %.0f, %.0f, %.0f", pos.X, pos.Y, pos.Z)
	} else {
		b.WriteString("Position: unknown")
	}
	if biome != "" {
		fmt.Fprintf(b, " | %s", biome)
	}
	if timeOfDay != "" {
		fmt.Fprintf(b, " | %s", timeOfDay)
	}
	b.WriteString("\n")
}

func aggregateEntities(entities []*pb.Entity) map[perception.EntityCategory][]*entityBucket {
	byCategory := map[perception.EntityCategory][]*entityBucket{}
	seen := map[perception.EntityCategory]map[string]*entityBucket{}

	for _, e := range entities {
		cat := perception.ClassifyEntity(e.Type)
		name := e.Type
		if e.Name != "" {
			name = e.Name
		}
		if seen[cat] == nil {
			seen[cat] = map[string]*entityBucket{}
		}
		if bk, ok := seen[cat][name]; ok {
			bk.count++
			if e.Distance < bk.nearest {
				bk.nearest = e.Distance
			}
		} else {
			bk := &entityBucket{name: name, count: 1, nearest: e.Distance}
			seen[cat][name] = bk
			byCategory[cat] = append(byCategory[cat], bk)
		}
	}
	return byCategory
}

func writeEntityBuckets(b *strings.Builder, entities []*pb.Entity) {
	if len(entities) == 0 {
		return
	}

	byCategory := aggregateEntities(entities)

	b.WriteString("Nearby entities:\n")
	for _, cat := range []perception.EntityCategory{
		perception.CategoryPlayer,
		perception.CategoryHostile,
		perception.CategoryNeutral,
		perception.CategoryPassive,
	} {
		buckets := byCategory[cat]
		if len(buckets) == 0 {
			continue
		}
		parts := make([]string, 0, len(buckets))
		for _, bk := range buckets {
			parts = append(parts, fmt.Sprintf("%d %s (%.0f blk)", bk.count, bk.name, bk.nearest))
		}
		fmt.Fprintf(b, "  %s: %s\n", cat, strings.Join(parts, ", "))
	}
}

func writeTask(b *strings.Builder, task *perception.TaskStatus) {
	if task == nil {
		return
	}
	fmt.Fprintf(b, "Task: %s %d/%d (%.0fs)\n",
		task.Name, task.Current, task.Total, task.Elapsed.Seconds())
}

func writeGoals(b *strings.Builder, summary string) {
	if summary != "" {
		fmt.Fprintf(b, "Goals: %s\n", summary)
	}
}

func writeLastWake(b *strings.Builder, since time.Duration) {
	fmt.Fprintf(b, "Last wake: %.0fs ago\n", since.Seconds())
}

func writeEventList(b *strings.Builder, events []*pb.GameEvent) {
	if len(events) == 0 {
		return
	}
	fmt.Fprintf(b, "\nEvents (%d):\n", len(events))
	for i, e := range events {
		if e == nil {
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
		fmt.Fprintf(b, "[%d] %s priority=%s%s desc=%q\n",
			i, e.Type.String(), e.Priority.String(), player, desc)
	}
}

type gatekeeperEventRow struct {
	Index    int    `json:"index"`
	Priority string `json:"priority"`
	Reason   string `json:"reason"`
}

// gatekeeperResponse mirrors the JSON the gatekeeper returns.
type gatekeeperResponse struct {
	Wake   bool                 `json:"wake"`
	Reason string               `json:"reason"`
	Events []gatekeeperEventRow `json:"events"`
}

// parseGatekeeperResponse parses the model's JSON response. On any parse
// failure it fails open (wake=true).
func parseGatekeeperResponse(text string, events []*pb.GameEvent) *GatekeeperDecision {
	jsonStr, ok := extractJSONObject(strings.TrimSpace(text))
	if !ok {
		return &GatekeeperDecision{Wake: true, Reason: "fail-open: no JSON object found"}
	}

	var parsed gatekeeperResponse
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return &GatekeeperDecision{Wake: true, Reason: "fail-open: JSON parse error"}
	}

	decision := &GatekeeperDecision{Wake: parsed.Wake, Reason: parsed.Reason}
	if parsed.Wake && len(parsed.Events) > 0 && len(events) > 0 {
		decision.ClassifiedEvents = applyEventPriorities(parsed.Events, events)
	}
	return decision
}

func extractJSONObject(s string) (string, bool) {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end < 0 || end <= start {
		return "", false
	}
	return s[start : end+1], true
}

func applyEventPriorities(rows []gatekeeperEventRow, events []*pb.GameEvent) []perception.ClassifiedEvent {
	classified := fallbackClassified(events)
	for _, row := range rows {
		if row.Index < 0 || row.Index >= len(classified) {
			continue
		}
		prio, ok := priorityFromString(row.Priority)
		if !ok {
			continue
		}
		classified[row.Index].Priority = prio
		classified[row.Index].Reason = row.Reason
	}
	return classified
}
