package main

import (
	"encoding/json"
	"time"

	sidecar "github.com/mab-go/golem/internal/grpc"
	"github.com/mab-go/golem/internal/logging"
	"github.com/mab-go/golem/internal/publisher"
)

// LogMsg carries a structured log entry from the TUILogger into the TUI.
type LogMsg struct {
	Time    time.Time
	Level   logging.Level
	Event   string
	Message string
	Fields  logging.Fields
}

// SidecarLogMsg carries a line from the sidecar subprocess stdout/stderr.
type SidecarLogMsg struct {
	Time   time.Time
	Line   string
	Stream string // "stdout" or "stderr"
}

// AgentCycleMsg signals a new think cycle has started.
type AgentCycleMsg struct {
	Cycle  uint64
	Reason string
}

// TextDeltaMsg carries a chunk of streaming text from Claude's response.
type TextDeltaMsg struct {
	Delta string
}

// ThinkingTextMsg carries the full text of Claude's response for a round.
type ThinkingTextMsg struct {
	Cycle    uint64
	Round    int
	FullText string
}

// ToolExecMsg signals a tool call is starting.
type ToolExecMsg struct {
	Name  string
	ID    string
	Input json.RawMessage
}

// ToolResultMsg signals a tool call has completed.
type ToolResultMsg struct {
	Name    string
	ID      string
	Result  string
	IsError bool
	Elapsed time.Duration
}

// TurnCompleteMsg signals a turn (API call + tool execution round) is done.
type TurnCompleteMsg struct {
	Cycle uint64
	Round int
	Stats publisher.TurnStats
}

// ComponentStatusMsg carries health status for a system component.
type ComponentStatusMsg struct {
	Component string
	Status    publisher.Status
	Detail    string
}

// MemoryUpdateMsg signals a memory file was written.
type MemoryUpdateMsg struct {
	File    string
	Preview string
}

// GameEventMsg carries a game event from the sidecar.
type GameEventMsg struct {
	Description string
	Priority    int32
	EventType   int32
}

// GatekeeperDecisionMsg carries the gatekeeper's wake/idle decision.
type GatekeeperDecisionMsg struct {
	Wake   bool
	Reason string
}

// ChatMsg carries a Minecraft chat message for display.
type ChatMsg struct {
	Sender  string
	Message string
	IsBot   bool
}

// ChatPaneActivateMsg signals that the chat pane should be shown.
type ChatPaneActivateMsg struct{}

// ServerLogMsg carries a line from the Docker Minecraft server log stream.
type ServerLogMsg struct {
	Time   time.Time
	Line   string
	Stream string // "stdout" or "stderr"
}

// ServerCmdResultMsg carries the result of a server command executed via rcon-cli.
type ServerCmdResultMsg struct {
	Command string
	Output  string
	Err     error
}

// ServerReadyMsg provides the command executor once the Docker server is running.
type ServerReadyMsg struct {
	ExecCmd func(string) (string, error)
}

// SidecarReadyMsg delivers the gRPC client once the sidecar is connected.
type SidecarReadyMsg struct {
	Client *sidecar.Client
}

// RemoteResultMsg carries the result of a remote command executed via gRPC.
type RemoteResultMsg struct {
	Command string
	Output  string
	Err     error
}
