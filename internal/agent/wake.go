package agent

import (
	"github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/perception"
)

// WakeReason identifies why the think loop was woken.
type WakeReason int

// WakeReason constants identify the trigger that woke the think loop.
const (
	WakeGatekeeper     WakeReason = iota // Haiku gatekeeper said YES
	WakeBypassChat                       // Player chat (skip gatekeeper)
	WakeBypassCritical                   // Critical event (skip gatekeeper)
	WakeBypassTask                       // Task completion/failure
	WakeHeartbeat                        // Timer-based temporal awareness
)

func (r WakeReason) String() string {
	switch r {
	case WakeGatekeeper:
		return "gatekeeper"
	case WakeBypassChat:
		return "bypass_chat"
	case WakeBypassCritical:
		return "bypass_critical"
	case WakeBypassTask:
		return "bypass_task"
	case WakeHeartbeat:
		return "heartbeat"
	default:
		return "unknown"
	}
}

// WakeSignal carries the reason and optional pre-fetched data for a wake.
type WakeSignal struct {
	Reason         WakeReason
	GatekeeperSnap *GatekeeperResult
}

// GatekeeperResult holds the lightweight perception and classified events
// produced by the gatekeeper when it decides to wake the think loop.
type GatekeeperResult struct {
	Events           []*pb.GameEvent
	ClassifiedEvents []perception.ClassifiedEvent
	Vitals           *pb.GetVitalSignsResponse
	Surroundings     *pb.GetSurroundingsResponse
	Reason           string
}
