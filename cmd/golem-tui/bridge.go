package main

import (
	"encoding/json"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mab-go/golem/internal/publisher"
)

var _ publisher.EventPublisher = (*tuiPublisher)(nil)

type tuiPublisher struct {
	send func(tea.Msg)
}

func newTUIPublisher(send func(tea.Msg)) *tuiPublisher {
	return &tuiPublisher{send: send}
}

func (t *tuiPublisher) PublishAgentCycle(cycle uint64, reason string) {
	t.send(AgentCycleMsg{Cycle: cycle, Reason: reason})
}

func (t *tuiPublisher) PublishTextDelta(delta string) {
	t.send(TextDeltaMsg{Delta: delta})
}

func (t *tuiPublisher) PublishThinkingText(cycle uint64, round int, fullText string) {
	t.send(ThinkingTextMsg{Cycle: cycle, Round: round, FullText: fullText})
}

func (t *tuiPublisher) PublishToolExec(name, id string, input json.RawMessage) {
	t.send(ToolExecMsg{Name: name, ID: id, Input: input})
}

func (t *tuiPublisher) PublishToolResult(name, id string, result string, isError bool, elapsed time.Duration) {
	t.send(ToolResultMsg{Name: name, ID: id, Result: result, IsError: isError, Elapsed: elapsed})
}

func (t *tuiPublisher) PublishTurnComplete(cycle uint64, round int, stats publisher.TurnStats) {
	t.send(TurnCompleteMsg{Cycle: cycle, Round: round, Stats: stats})
}

func (t *tuiPublisher) PublishComponentStatus(component string, status publisher.Status, detail string) {
	t.send(ComponentStatusMsg{Component: component, Status: status, Detail: detail})
}

func (t *tuiPublisher) PublishMemoryUpdate(file string, preview string) {
	t.send(MemoryUpdateMsg{File: file, Preview: preview})
}

func (t *tuiPublisher) PublishGameEvent(description string, priority int32, eventType int32) {
	t.send(GameEventMsg{Description: description, Priority: priority, EventType: eventType})
}

func (t *tuiPublisher) PublishChat(sender, message string, isBot bool) {
	t.send(ChatMsg{Sender: sender, Message: message, IsBot: isBot})
}

func (t *tuiPublisher) PublishGatekeeperDecision(wake bool, reason string) {
	t.send(GatekeeperDecisionMsg{Wake: wake, Reason: reason})
}

func (t *tuiPublisher) PublishLog(_, _, _ string, _ map[string]any) {}
