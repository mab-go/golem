package agent

import (
	"strings"

	"github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/perception"
)

// containsBypass checks if any events should bypass the gatekeeper entirely.
// Returns true if a bypass wake was already sent by handleEvent for these events.
func containsBypass(events []*pb.GameEvent) bool {
	for _, e := range events {
		if e == nil {
			continue
		}
		if e.Type == pb.EventType_EVENT_CHAT_MESSAGE {
			return true
		}
		if e.Priority == pb.EventPriority_EVENT_PRIORITY_CRITICAL {
			return true
		}
	}
	return false
}

// routineEventsOnly returns true when every event in the slice is low-impact
// noise that never warrants waking the agent.
func routineEventsOnly(events []*pb.GameEvent) bool {
	for _, e := range events {
		if e != nil && !isRoutineEvent(e) {
			return false
		}
	}
	return true
}

func isRoutineEvent(e *pb.GameEvent) bool {
	if e.Priority > pb.EventPriority_EVENT_PRIORITY_LOW {
		return false
	}
	switch e.Type {
	case pb.EventType_EVENT_ENTITY_DESPAWN,
		pb.EventType_EVENT_BLOCK_UPDATE,
		pb.EventType_EVENT_DAY_NIGHT,
		pb.EventType_EVENT_WEATHER_CHANGE:
		return true
	case pb.EventType_EVENT_ENTITY_SPAWN:
		ent := e.GetEntity()
		if ent == nil {
			return false
		}
		cat := perception.ClassifyEntity(ent.Type)
		return cat == perception.CategoryPassive || cat == perception.CategoryNeutral
	default:
		return false
	}
}

// isTerminalTaskDesc checks if a task progress event description indicates
// a terminal state (completed/failed/cancelled).
func isTerminalTaskDesc(desc string) bool {
	lower := strings.ToLower(desc)
	return strings.Contains(lower, "completed") ||
		strings.Contains(lower, "failed") ||
		strings.Contains(lower, "cancelled")
}
