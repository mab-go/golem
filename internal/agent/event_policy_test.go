package agent

import (
	"testing"

	"github.com/mab-go/golem/internal/grpc/pb"
)

func TestContainsBypass(t *testing.T) {
	tests := []struct {
		name   string
		events []*pb.GameEvent
		want   bool
	}{
		{
			name:   "nil events",
			events: nil,
			want:   false,
		},
		{
			name: "low priority only",
			events: []*pb.GameEvent{
				{Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_LOW},
			},
			want: false,
		},
		{
			name: "chat message",
			events: []*pb.GameEvent{
				{Type: pb.EventType_EVENT_CHAT_MESSAGE, Priority: pb.EventPriority_EVENT_PRIORITY_NORMAL},
			},
			want: true,
		},
		{
			name: "critical event",
			events: []*pb.GameEvent{
				{Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_CRITICAL},
			},
			want: true,
		},
		{
			name:   "nil event in slice",
			events: []*pb.GameEvent{nil},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsBypass(tt.events); got != tt.want {
				t.Errorf("containsBypass() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTerminalTaskDesc(t *testing.T) {
	tests := []struct {
		desc string
		want bool
	}{
		{"[gather] completed: gathered 16 stone", true},
		{"[gather] failed: could not find stone", true},
		{"[gather] cancelled (timeout or explicit)", true},
		{"[gather] 10/16 -- mining stone", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isTerminalTaskDesc(tt.desc); got != tt.want {
			t.Errorf("isTerminalTaskDesc(%q) = %v, want %v", tt.desc, got, tt.want)
		}
	}
}

func TestIsRoutineEvent(t *testing.T) {
	tests := []struct {
		name string
		evt  *pb.GameEvent
		want bool
	}{
		{"passive spawn", &pb.GameEvent{
			Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_LOW,
			Entity: &pb.Entity{Type: "bat"},
		}, true},
		{"neutral spawn", &pb.GameEvent{
			Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_LOW,
			Entity: &pb.Entity{Type: "wolf"},
		}, true},
		{"hostile spawn", &pb.GameEvent{
			Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_LOW,
			Entity: &pb.Entity{Type: "zombie"},
		}, false},
		{"player spawn", &pb.GameEvent{
			Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_LOW,
			Entity: &pb.Entity{Type: "player"},
		}, false},
		{"spawn nil entity", &pb.GameEvent{
			Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_LOW,
		}, false},
		{"spawn non-low priority", &pb.GameEvent{
			Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_NORMAL,
			Entity: &pb.Entity{Type: "cow"},
		}, false},
		{"despawn", &pb.GameEvent{
			Type: pb.EventType_EVENT_ENTITY_DESPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_LOW,
		}, true},
		{"block update", &pb.GameEvent{
			Type: pb.EventType_EVENT_BLOCK_UPDATE, Priority: pb.EventPriority_EVENT_PRIORITY_LOW,
		}, true},
		{"day night", &pb.GameEvent{
			Type: pb.EventType_EVENT_DAY_NIGHT, Priority: pb.EventPriority_EVENT_PRIORITY_LOW,
		}, true},
		{"weather", &pb.GameEvent{
			Type: pb.EventType_EVENT_WEATHER_CHANGE, Priority: pb.EventPriority_EVENT_PRIORITY_LOW,
		}, true},
		{"chat", &pb.GameEvent{
			Type: pb.EventType_EVENT_CHAT_MESSAGE, Priority: pb.EventPriority_EVENT_PRIORITY_HIGH,
		}, false},
		{"health change", &pb.GameEvent{
			Type: pb.EventType_EVENT_HEALTH_CHANGE, Priority: pb.EventPriority_EVENT_PRIORITY_HIGH,
		}, false},
		{"player joined", &pb.GameEvent{
			Type: pb.EventType_EVENT_PLAYER_JOINED, Priority: pb.EventPriority_EVENT_PRIORITY_NORMAL,
		}, false},
		{"task progress", &pb.GameEvent{
			Type: pb.EventType_EVENT_TASK_PROGRESS, Priority: pb.EventPriority_EVENT_PRIORITY_LOW,
		}, false},
		{"despawn non-low priority", &pb.GameEvent{
			Type: pb.EventType_EVENT_ENTITY_DESPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_NORMAL,
		}, false},
		{"unknown entity spawn defaults to routine", &pb.GameEvent{
			Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_LOW,
			Entity: &pb.Entity{Type: "modded_creature"},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRoutineEvent(tt.evt); got != tt.want {
				t.Errorf("isRoutineEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRoutineEventsOnly(t *testing.T) {
	tests := []struct {
		name   string
		events []*pb.GameEvent
		want   bool
	}{
		{"empty", nil, true},
		{"all routine", []*pb.GameEvent{
			{Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_LOW, Entity: &pb.Entity{Type: "bat"}},
			{Type: pb.EventType_EVENT_ENTITY_DESPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_LOW},
		}, true},
		{"one hostile spawn", []*pb.GameEvent{
			{Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_LOW, Entity: &pb.Entity{Type: "bat"}},
			{Type: pb.EventType_EVENT_ENTITY_SPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_LOW, Entity: &pb.Entity{Type: "zombie"}},
		}, false},
		{"nil events skipped", []*pb.GameEvent{
			nil,
			{Type: pb.EventType_EVENT_ENTITY_DESPAWN, Priority: pb.EventPriority_EVENT_PRIORITY_LOW},
		}, true},
		{"chat mixed in", []*pb.GameEvent{
			{Type: pb.EventType_EVENT_BLOCK_UPDATE, Priority: pb.EventPriority_EVENT_PRIORITY_LOW},
			{Type: pb.EventType_EVENT_CHAT_MESSAGE, Priority: pb.EventPriority_EVENT_PRIORITY_HIGH},
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := routineEventsOnly(tt.events); got != tt.want {
				t.Errorf("routineEventsOnly() = %v, want %v", got, tt.want)
			}
		})
	}
}
