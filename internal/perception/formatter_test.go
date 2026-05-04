package perception

import (
	"strings"
	"testing"
	"time"

	"github.com/mab-go/golem/internal/grpc/pb"
)

func requireContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected %q in output:\n%s", substr, s)
	}
}

func requireNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Errorf("unexpected %q in output:\n%s", substr, s)
	}
}

func sampleSurroundings() *pb.GetSurroundingsResponse {
	return &pb.GetSurroundingsResponse{
		Position:   &pb.Vec3{X: 142, Y: 68, Z: -231},
		Biome:      "birch_forest",
		TimeOfDay:  "afternoon",
		LightLevel: 12,
		Dimension:  "overworld",
		NearbyEntities: []*pb.Entity{
			{Type: "cow", Distance: 8},
			{Type: "sheep", Distance: 14},
			{Type: "sheep", Distance: 15},
		},
		NotableBlocks: []*pb.Block{
			{Type: "oak_log", Distance: 6},
			{Type: "oak_log", Distance: 8},
		},
	}
}

func TestParseFormat(t *testing.T) {
	cases := map[string]Format{"": FormatProse, "prose": FormatProse, "STRUCTURED": FormatStructured}
	for in, want := range cases {
		got, err := ParseFormat(in)
		if err != nil {
			t.Fatalf("ParseFormat(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("ParseFormat(%q)=%d want %d", in, got, want)
		}
	}
	if _, err := ParseFormat("xml"); err == nil {
		t.Fatalf("expected error for unknown format")
	}
}

func TestParseVerbosity(t *testing.T) {
	cases := map[string]VerbosityMode{"": VerbosityStandard, "verbose": VerbosityVerbose, "TERSE": VerbosityTerse}
	for in, want := range cases {
		got, err := ParseVerbosity(in)
		if err != nil {
			t.Fatalf("ParseVerbosity(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("ParseVerbosity(%q)=%d want %d", in, got, want)
		}
	}
}

func TestFormatSurroundingsProseStandard(t *testing.T) {
	f := NewFormatter(FormatProse, VerbosityStandard)
	out := f.FormatSurroundings(sampleSurroundings())
	for _, want := range []string{"birch forest", "afternoon", "light 12", "142, 68, -231", "cow", "2 sheep", "oak_log"} {
		if !strings.Contains(out, want) {
			t.Errorf("prose/standard missing %q\noutput: %s", want, out)
		}
	}
}

func TestFormatSurroundingsTerse(t *testing.T) {
	f := NewFormatter(FormatProse, VerbosityTerse)
	out := f.FormatSurroundings(sampleSurroundings())
	if !strings.Contains(out, "birch forest") || !strings.Contains(out, "afternoon") {
		t.Errorf("terse missing core fields: %s", out)
	}
}

func TestFormatSurroundingsVerbose(t *testing.T) {
	f := NewFormatter(FormatProse, VerbosityVerbose)
	out := f.FormatSurroundings(sampleSurroundings())
	if !strings.Contains(out, "You stand") {
		t.Errorf("verbose missing narrative opener: %s", out)
	}
}

func TestFormatSurroundingsStructured(t *testing.T) {
	f := NewFormatter(FormatStructured, VerbosityStandard)
	out := f.FormatSurroundings(sampleSurroundings())
	for _, want := range []string{"[Position]", "[Entities]", "[Blocks]", "cow x1", "sheep x2", "oak_log x2"} {
		if !strings.Contains(out, want) {
			t.Errorf("structured missing %q\noutput: %s", want, out)
		}
	}
}

func TestFormatSurroundingsNilSafe(t *testing.T) {
	f := NewFormatter(FormatProse, VerbosityStandard)
	if got := f.FormatSurroundings(nil); got == "" {
		t.Fatal("expected non-empty output for nil surroundings")
	}
}

func TestFormatVitalSigns(t *testing.T) {
	f := NewFormatter(FormatProse, VerbosityStandard)
	out := f.FormatVitalSigns(&pb.GetVitalSignsResponse{
		Health: 18, MaxHealth: 20, Food: 15, Armor: 12, XpLevel: 7, XpProgress: 0.42, GameMode: "survival",
		MainHand: &pb.InventoryItem{Name: "iron_sword"},
		Effects:  []*pb.StatusEffect{{Name: "regeneration"}},
	})
	for _, want := range []string{"HP 18/20", "food 15/20", "armor 12", "xp 7 (42%)", "hand iron_sword", "effects regeneration", "mode survival"} {
		if !strings.Contains(out, want) {
			t.Errorf("vitals missing %q\noutput: %s", want, out)
		}
	}
}

func TestFormatInventory(t *testing.T) {
	f := NewFormatter(FormatProse, VerbosityStandard)
	out := f.FormatInventory(&pb.GetInventoryResponse{
		Items: []*pb.InventoryItem{
			{Name: "oak_log", Count: 5},
			{Name: "oak_log", Count: 3},
			{Name: "iron_pickaxe", Count: 1, MaxDurability: 250, CurrentDurability: 10},
		},
		EmptySlots: 26,
		ArmorItems: []*pb.InventoryItem{{Name: "iron_helmet"}},
		CraftSuggestions: []*pb.CraftSuggestion{
			{ItemName: "stick", MaxCount: 4},
			{ItemName: "crafting_table", MaxCount: 1, RequiresCraftingTable: false},
		},
	})
	for _, want := range []string{"oak_logx8", "iron_pickaxex1", "durability 4%!", "26 empty", "armor: iron_helmet", "stickx4"} {
		if !strings.Contains(out, want) {
			t.Errorf("inventory missing %q\noutput: %s", want, out)
		}
	}
}

func TestFormatEventsEmpty(t *testing.T) {
	f := NewFormatter(FormatProse, VerbosityStandard)
	if got := f.FormatEvents(nil); !strings.Contains(got, "No new") {
		t.Errorf("expected 'No new' marker, got %q", got)
	}
}

func TestFormatClassifiedEventsGroupsByPriority(t *testing.T) {
	f := NewFormatter(FormatProse, VerbosityStandard)
	events := []ClassifiedEvent{
		{
			Event:    &pb.GameEvent{Type: pb.EventType_EVENT_ENTITY_SPAWN, Timestamp: 1000, Description: "pig appeared"},
			Priority: pb.EventPriority_EVENT_PRIORITY_LOW,
		},
		{
			Event:    &pb.GameEvent{Type: pb.EventType_EVENT_BOT_DEATH, Timestamp: 2000, Description: "bot died"},
			Priority: pb.EventPriority_EVENT_PRIORITY_CRITICAL,
			Reason:   "lava",
		},
		{
			Event:    &pb.GameEvent{Type: pb.EventType_EVENT_CHAT_MESSAGE, Timestamp: 3000, Description: "alice: hi golem", PlayerName: "alice"},
			Priority: pb.EventPriority_EVENT_PRIORITY_HIGH,
			Reason:   "addressed to bot",
		},
		{
			Event:    &pb.GameEvent{Type: pb.EventType_EVENT_WEATHER_CHANGE, Timestamp: 4000, Description: "it started to rain"},
			Priority: pb.EventPriority_EVENT_PRIORITY_LOW,
		},
	}

	out := f.FormatClassifiedEvents(events)

	// Critical must precede high; high must precede the LOW collapse line.
	critIdx := strings.Index(out, "bot died")
	highIdx := strings.Index(out, "hi golem")
	lowIdx := strings.Index(out, "[LOW]")
	if critIdx < 0 || highIdx < 0 || lowIdx < 0 {
		t.Fatalf("missing expected lines:\n%s", out)
	}
	if critIdx >= highIdx || highIdx >= lowIdx {
		t.Errorf("priority ordering wrong (crit=%d high=%d low=%d):\n%s", critIdx, highIdx, lowIdx, out)
	}

	// Reason should appear for high/critical events.
	requireContains(t, out, "lava")
	requireContains(t, out, "addressed to bot")

	// LOW events collapsed into one count line, not individual lines.
	requireNotContains(t, out, "pig appeared")
	requireNotContains(t, out, "started to rain")
	requireContains(t, out, "2 routine")
}

func TestFormatClassifiedEventsEmptyIsHuman(t *testing.T) {
	f := NewFormatter(FormatProse, VerbosityStandard)
	if got := f.FormatClassifiedEvents(nil); !strings.Contains(got, "No new events") {
		t.Errorf("expected 'No new events' message, got %q", got)
	}
}

func TestFormatEventsOrderingAndDedupe(t *testing.T) {
	f := NewFormatter(FormatProse, VerbosityStandard)
	events := []*pb.GameEvent{
		{Type: pb.EventType_EVENT_ENTITY_SPAWN, Timestamp: 2000, Description: "zombie spawned", Entity: &pb.Entity{Type: "zombie"}},
		{Type: pb.EventType_EVENT_ENTITY_SPAWN, Timestamp: 1000, Description: "zombie spawned", Entity: &pb.Entity{Type: "zombie"}},
		{Type: pb.EventType_EVENT_ENTITY_SPAWN, Timestamp: 3000, Description: "zombie spawned", Entity: &pb.Entity{Type: "zombie"}},
		{Type: pb.EventType_EVENT_CHAT_MESSAGE, Timestamp: 4000, Description: "player: hi", PlayerName: "alice", ChatMessage: "hi"},
	}
	out := f.FormatEvents(events)
	if !strings.Contains(out, "x3") {
		t.Errorf("expected deduped x3 count, got:\n%s", out)
	}
	if !strings.Contains(out, "player: hi") {
		t.Errorf("chat event missing:\n%s", out)
	}
	zIdx := strings.Index(out, "zombie")
	cIdx := strings.Index(out, "player:")
	if zIdx == -1 || cIdx == -1 || zIdx > cIdx {
		t.Errorf("events not chronological:\n%s", out)
	}
}

func TestFormatActiveTaskNil(t *testing.T) {
	f := NewFormatter(FormatProse, VerbosityStandard)
	if got := f.FormatActiveTask(nil); got != "" {
		t.Errorf("expected empty for nil, got %q", got)
	}
}

func TestFormatActiveTaskWithProgress(t *testing.T) {
	f := NewFormatter(FormatProse, VerbosityStandard)
	out := f.FormatActiveTask(&TaskStatus{
		Name: "gather", Current: 10, Total: 16, Elapsed: 22 * time.Second, Message: "mining stone",
	})
	for _, want := range []string{"[Active Task]", "gather", "10/16", "22s", "mining stone", "cancel_task"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output: %s", want, out)
		}
	}
}

func TestFormatActiveTaskWithoutTotal(t *testing.T) {
	f := NewFormatter(FormatProse, VerbosityStandard)
	out := f.FormatActiveTask(&TaskStatus{
		Name: "organize_inventory", Total: 0, Elapsed: 5 * time.Second,
	})
	if strings.Contains(out, "/") {
		t.Errorf("should omit fraction when Total==0: %s", out)
	}
	if !strings.Contains(out, "organize_inventory") || !strings.Contains(out, "5s") {
		t.Errorf("missing expected content: %s", out)
	}
}
