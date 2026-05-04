// Package publisher defines the EventPublisher interface used to bridge the
// agent loop to external consumers (e.g. a TUI). All methods are
// fire-and-forget: no return values, no blocking.
package publisher

import (
	"encoding/json"
	"time"
)

// Status describes the health of a monitored component.
type Status int

// Status values for component health reporting.
const (
	StatusOK       Status = iota
	StatusDegraded        // partial failures, elevated error rate
	StatusDown            // unreachable or crashed
)

// EventPublisher receives lifecycle events from the agent loop. Every method
// must be safe for concurrent calls from multiple goroutines and must never
// block the caller.
type EventPublisher interface {
	PublishAgentCycle(cycle uint64, reason string)
	PublishTextDelta(delta string)
	PublishThinkingText(cycle uint64, round int, fullText string)
	PublishToolExec(name, id string, input json.RawMessage)
	PublishToolResult(name, id string, result string, isError bool, elapsed time.Duration)
	PublishTurnComplete(cycle uint64, round int, stats TurnStats)
	PublishComponentStatus(component string, status Status, detail string)
	PublishMemoryUpdate(file string, preview string)
	PublishGameEvent(description string, priority int32, eventType int32)
	PublishChat(sender, message string, isBot bool)
	PublishGatekeeperDecision(wake bool, reason string)
	PublishLog(level, event, msg string, fields map[string]any)
}

// TurnStats bundles the structured fields emitted at the end of each API
// round for display in the TUI's log and events panes.
type TurnStats struct {
	StopReason  string
	Model       string
	TokensIn    int64
	TokensOut   int64
	CacheCreate int64
	CacheRead   int64
	APIMillis   int64
	ToolMillis  int64
	ToolNames   []string
}
