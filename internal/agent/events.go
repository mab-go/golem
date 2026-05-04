package agent

import "github.com/mab-go/golem/internal/logging"

// Lifecycle
const (
	eventStart logging.Event = "agent.start"
	eventStop  logging.Event = "agent.stop"
)

// Think cycle
const (
	eventWake         logging.Event = "agent.wake"
	eventCycleFailed  logging.Event = "agent.cycle_failed"
	eventTurnComplete logging.Event = "agent.turn_complete"
	eventHeartbeat    logging.Event = "agent.heartbeat"
)

// Tools
const (
	eventToolExec   logging.Event = "agent.tool_exec"
	eventToolFailed logging.Event = "agent.tool_failed"
	eventToolResult logging.Event = "agent.tool_result"
)

// Strategic advisor
const (
	eventThinkDeepDone   logging.Event = "agent.think_deep_done"
	eventThinkDeepFailed logging.Event = "agent.think_deep_failed"
)

// Perception
const (
	eventPerceptionFailed   logging.Event = "agent.perception_failed"
	eventVitalsFailed       logging.Event = "agent.vitals_failed"
	eventSurroundingsFailed logging.Event = "agent.surroundings_failed"
)

// Gatekeeper
const (
	eventGatekeeperWake         logging.Event = "agent.gatekeeper_wake"
	eventGatekeeperIdle         logging.Event = "agent.gatekeeper_idle"
	eventGatekeeperFailed       logging.Event = "agent.gatekeeper_failed"
	eventGatekeeperHighFailRate logging.Event = "agent.gatekeeper_high_fail_rate"
	eventGatekeeperSkipRoutine  logging.Event = "agent.gatekeeper_skip_routine"
	eventClassifyFailed         logging.Event = "agent.classify_failed"
)

// Events / streaming
const (
	eventSubscribeFailed logging.Event = "agent.subscribe_failed"
	eventStreamBroke     logging.Event = "agent.stream_broke"
)

// Chat
const (
	eventChatFiltered   logging.Event = "agent.chat_filtered"
	eventChatSurfaced   logging.Event = "agent.chat_surfaced"
	eventChatSendFailed logging.Event = "agent.chat_send_failed"
)

// Tool flight
const (
	eventGatekeeperSkipToolFlight logging.Event = "agent.gatekeeper_skip_tool_flight"
	eventToolInterrupted          logging.Event = "agent.tool_interrupted"
	eventEmergencyStop            logging.Event = "agent.emergency_stop"
)
