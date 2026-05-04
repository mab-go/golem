package claude

import (
	"strings"
	"testing"
	"time"

	"github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/perception"
)

func TestFormatGatekeeperInput_Full(t *testing.T) {
	snap := GatekeeperSnapshot{
		Vitals: &pb.GetVitalSignsResponse{
			Health: 18, MaxHealth: 20, Food: 14, Armor: 10,
		},
		Position:  &pb.Vec3{X: 104, Y: 63, Z: -221},
		Biome:     "plains",
		TimeOfDay: "day",
		NearbyEntities: []*pb.Entity{
			{Type: "player", Name: "loosid", Distance: 3},
			{Type: "cow", Distance: 15},
			{Type: "cow", Distance: 20},
			{Type: "zombie", Distance: 12},
		},
		ActiveTask: &perception.TaskStatus{
			Name: "gather stone", Current: 10, Total: 16,
			Elapsed: 22 * time.Second,
		},
		Events: []*pb.GameEvent{
			{
				Type:        pb.EventType_EVENT_ENTITY_SPAWN,
				Priority:    pb.EventPriority_EVENT_PRIORITY_LOW,
				Description: "cow spawned 15 blocks away",
			},
		},
		TimeSinceWake: 8 * time.Second,
		GoalsSummary:  "Build a shelter near the river",
	}

	out := formatGatekeeperInput(snap)

	for _, want := range []string{
		"HP 18/20",
		"food 14/20",
		"armor 10",
		"104, 63, -221",
		"plains",
		"day",
		"Players: 1 loosid",
		"Hostile: 1 zombie",
		"Passive: 2 cow",
		"gather stone",
		"Last wake: 8s ago",
		"Events (1):",
		"Build a shelter",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n\nfull output:\n%s", want, out)
		}
	}
}

func TestFormatGatekeeperInput_Minimal(t *testing.T) {
	snap := GatekeeperSnapshot{TimeSinceWake: 3 * time.Second}
	out := formatGatekeeperInput(snap)

	if !strings.Contains(out, "Vitals: unavailable") {
		t.Errorf("missing unavailable vitals: %s", out)
	}
	if !strings.Contains(out, "Position: unknown") {
		t.Errorf("missing unknown position: %s", out)
	}
	if strings.Contains(out, "Events") {
		t.Errorf("should not contain Events section with no events: %s", out)
	}
}

func TestParseGatekeeperResponse_Wake(t *testing.T) {
	events := []*pb.GameEvent{
		{Type: pb.EventType_EVENT_HEALTH_CHANGE, Priority: pb.EventPriority_EVENT_PRIORITY_NORMAL, Description: "took 2 damage"},
		{Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_LOW, Description: "cow spawned"},
	}

	resp := `{"wake": true, "reason": "taking damage", "events": [{"index": 0, "priority": "high", "reason": "damage"}, {"index": 1, "priority": "low", "reason": "routine"}]}`
	decision := parseGatekeeperResponse(resp, events)

	if !decision.Wake {
		t.Fatal("expected wake=true")
	}
	if decision.Reason != "taking damage" {
		t.Errorf("reason=%q want 'taking damage'", decision.Reason)
	}
	if len(decision.ClassifiedEvents) != 2 {
		t.Fatalf("classified events len=%d want 2", len(decision.ClassifiedEvents))
	}
	if decision.ClassifiedEvents[0].Priority != pb.EventPriority_EVENT_PRIORITY_HIGH {
		t.Errorf("event[0] priority=%v want HIGH", decision.ClassifiedEvents[0].Priority)
	}
	if decision.ClassifiedEvents[1].Priority != pb.EventPriority_EVENT_PRIORITY_LOW {
		t.Errorf("event[1] priority=%v want LOW", decision.ClassifiedEvents[1].Priority)
	}
}

func TestParseGatekeeperResponse_NoWake(t *testing.T) {
	events := []*pb.GameEvent{
		{Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_LOW},
	}

	resp := `{"wake": false, "reason": "nothing interesting"}`
	decision := parseGatekeeperResponse(resp, events)

	if decision.Wake {
		t.Fatal("expected wake=false")
	}
	if decision.ClassifiedEvents != nil {
		t.Errorf("classified events should be nil for no-wake, got %d", len(decision.ClassifiedEvents))
	}
}

func TestParseGatekeeperResponse_MalformedJSON_FailOpen(t *testing.T) {
	decision := parseGatekeeperResponse("this is not JSON", nil)
	if !decision.Wake {
		t.Fatal("malformed JSON should fail open (wake=true)")
	}
	if !strings.Contains(decision.Reason, "fail-open") {
		t.Errorf("reason should mention fail-open: %q", decision.Reason)
	}
}

func TestParseGatekeeperResponse_WrappedInProse(t *testing.T) {
	resp := `Here is my analysis:
{"wake": true, "reason": "player chatting"}
That's my decision.`

	decision := parseGatekeeperResponse(resp, nil)
	if !decision.Wake {
		t.Fatal("should extract JSON from prose wrapper")
	}
	if decision.Reason != "player chatting" {
		t.Errorf("reason=%q", decision.Reason)
	}
}

func TestParseGatekeeperResponse_EmptyResponse_FailOpen(t *testing.T) {
	decision := parseGatekeeperResponse("", nil)
	if !decision.Wake {
		t.Fatal("empty response should fail open")
	}
}

func TestParseGatekeeperResponse_OutOfRangeIndex(t *testing.T) {
	events := []*pb.GameEvent{
		{Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_LOW},
	}
	resp := `{"wake": true, "reason": "test", "events": [{"index": 5, "priority": "high", "reason": "oob"}]}`
	decision := parseGatekeeperResponse(resp, events)

	if !decision.Wake {
		t.Fatal("expected wake=true")
	}
	if len(decision.ClassifiedEvents) != 1 {
		t.Fatalf("classified len=%d want 1", len(decision.ClassifiedEvents))
	}
	if decision.ClassifiedEvents[0].Priority != pb.EventPriority_EVENT_PRIORITY_LOW {
		t.Errorf("oob index should not change priority, got %v", decision.ClassifiedEvents[0].Priority)
	}
}

func TestFormatGatekeeperInput_PlayerNotHostile(t *testing.T) {
	snap := GatekeeperSnapshot{
		TimeSinceWake: 5 * time.Second,
		NearbyEntities: []*pb.Entity{
			{Type: "player", Name: "loosid", Distance: 2},
			{Type: "zombie", Distance: 15},
			{Type: "cow", Distance: 20},
		},
	}
	out := formatGatekeeperInput(snap)

	if !strings.Contains(out, "Players: 1 loosid (2 blk)") {
		t.Errorf("player should appear under Players category\n%s", out)
	}
	if !strings.Contains(out, "Hostile: 1 zombie (15 blk)") {
		t.Errorf("zombie should appear under Hostile category\n%s", out)
	}
	if !strings.Contains(out, "Passive: 1 cow (20 blk)") {
		t.Errorf("cow should appear under Passive category\n%s", out)
	}

	// "loosid" must not appear on a Hostile line
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Hostile") && strings.Contains(line, "loosid") {
			t.Errorf("player 'loosid' must not appear in Hostile section: %s", line)
		}
	}
}

func TestFormatGatekeeperInput_EmptyCategoriesOmitted(t *testing.T) {
	snap := GatekeeperSnapshot{
		TimeSinceWake: 5 * time.Second,
		NearbyEntities: []*pb.Entity{
			{Type: "player", Name: "alice", Distance: 5},
		},
	}
	out := formatGatekeeperInput(snap)

	if !strings.Contains(out, "Players:") {
		t.Errorf("should contain Players section\n%s", out)
	}
	if strings.Contains(out, "Hostile:") {
		t.Errorf("should not contain Hostile section when no hostiles\n%s", out)
	}
	if strings.Contains(out, "Passive:") {
		t.Errorf("should not contain Passive section when no passives\n%s", out)
	}
}

func TestGatekeeperPromptNoStaleness(t *testing.T) {
	if strings.Contains(gatekeeperSystemPrompt, "stale") {
		t.Error("gatekeeper prompt should not mention staleness")
	}
	if strings.Contains(gatekeeperSystemPrompt, "30 seconds") {
		t.Error("gatekeeper prompt should not contain 30-second threshold")
	}
}

func TestFormatGatekeeperInput_NeutralCategory(t *testing.T) {
	snap := GatekeeperSnapshot{
		TimeSinceWake: 5 * time.Second,
		NearbyEntities: []*pb.Entity{
			{Type: "wolf", Distance: 6},
			{Type: "wolf", Distance: 10},
			{Type: "bee", Distance: 8},
		},
	}
	out := formatGatekeeperInput(snap)

	if !strings.Contains(out, "Neutral:") {
		t.Errorf("should contain Neutral section\n%s", out)
	}
	if !strings.Contains(out, "2 wolf (6 blk)") {
		t.Errorf("should aggregate wolves with nearest distance\n%s", out)
	}
	if !strings.Contains(out, "1 bee (8 blk)") {
		t.Errorf("should show bee\n%s", out)
	}
}
