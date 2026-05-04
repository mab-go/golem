package publisher

import (
	"encoding/json"
	"time"
)

type nopPublisher struct{}

// Nop returns an EventPublisher whose methods are all no-ops.
// Used by the headless binary for zero-overhead publishing.
func Nop() EventPublisher { return nopPublisher{} }

func (nopPublisher) PublishAgentCycle(uint64, string)                              {}
func (nopPublisher) PublishTextDelta(string)                                       {}
func (nopPublisher) PublishThinkingText(uint64, int, string)                       {}
func (nopPublisher) PublishToolExec(string, string, json.RawMessage)               {}
func (nopPublisher) PublishToolResult(string, string, string, bool, time.Duration) {}
func (nopPublisher) PublishTurnComplete(uint64, int, TurnStats)                    {}
func (nopPublisher) PublishComponentStatus(string, Status, string)                 {}
func (nopPublisher) PublishMemoryUpdate(string, string)                            {}
func (nopPublisher) PublishGameEvent(string, int32, int32)                         {}
func (nopPublisher) PublishChat(string, string, bool)                              {}
func (nopPublisher) PublishGatekeeperDecision(bool, string)                        {}
func (nopPublisher) PublishLog(string, string, string, map[string]any)             {}
