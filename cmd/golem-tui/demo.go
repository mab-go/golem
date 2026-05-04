package main

import (
	"encoding/json"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mab-go/golem/internal/logging"
	"github.com/mab-go/golem/internal/publisher"
)

func runDemoDriver(p *tea.Program) {
	time.Sleep(500 * time.Millisecond)

	demoStartup(p)
	demoCycle1(p)
	demoWorldEvents(p)
	demoChat(p)
	demoCycle2(p)
}

func demoStartup(p *tea.Program) {
	send := func(msg tea.Msg, delay time.Duration) {
		time.Sleep(delay)
		p.Send(msg)
	}

	send(ComponentStatusMsg{"server", publisher.StatusDegraded, "Checking container..."}, 300*time.Millisecond)
	send(ServerLogMsg{Time: time.Now(), Line: "[server] Starting Minecraft server on 0.0.0.0:25565", Stream: "stdout"}, 200*time.Millisecond)
	send(ComponentStatusMsg{"server", publisher.StatusDegraded, "Starting container..."}, 600*time.Millisecond)
	send(ServerLogMsg{Time: time.Now(), Line: "[server] Preparing level \"world\"", Stream: "stdout"}, 400*time.Millisecond)
	send(ServerLogMsg{Time: time.Now(), Line: "[server] Done (12.4s)! For help, type \"help\"", Stream: "stdout"}, 800*time.Millisecond)
	send(ComponentStatusMsg{"server", publisher.StatusOK, "Running"}, 200*time.Millisecond)
	send(ServerReadyMsg{ExecCmd: demoExecCmd}, 0)

	send(ComponentStatusMsg{"controller", publisher.StatusDegraded, "Starting sidecar..."}, 200*time.Millisecond)

	send(SidecarLogMsg{Time: time.Now(), Line: "golem-sidecar v0.1.0", Stream: "stdout"}, 400*time.Millisecond)
	send(SidecarLogMsg{Time: time.Now(), Line: "loading mineflayer plugins...", Stream: "stdout"}, 200*time.Millisecond)
	send(SidecarLogMsg{Time: time.Now(), Line: "  mineflayer-pathfinder ok", Stream: "stdout"}, 100*time.Millisecond)
	send(SidecarLogMsg{Time: time.Now(), Line: "  mineflayer-pvp ok", Stream: "stdout"}, 50*time.Millisecond)
	send(SidecarLogMsg{Time: time.Now(), Line: "  mineflayer-auto-eat ok", Stream: "stdout"}, 50*time.Millisecond)
	send(SidecarLogMsg{Time: time.Now(), Line: "  mineflayer-collectblock ok", Stream: "stdout"}, 50*time.Millisecond)
	send(SidecarLogMsg{Time: time.Now(), Line: "gRPC server listening on :50051", Stream: "stdout"}, 300*time.Millisecond)
	send(SidecarLogMsg{Time: time.Now(), Line: "bot connected to localhost:25565 as claude", Stream: "stdout"}, 600*time.Millisecond)

	send(ComponentStatusMsg{"controller", publisher.StatusDegraded, "Connecting to gRPC..."}, 200*time.Millisecond)
	send(ComponentStatusMsg{"controller", publisher.StatusOK, "Connected"}, 500*time.Millisecond)

	send(LogMsg{Time: time.Now(), Level: logging.InfoLevel, Event: "sidecar.connect", Message: "connected to sidecar", Fields: logging.Fields{"address": "localhost:50051"}}, 100*time.Millisecond)
	send(LogMsg{Time: time.Now(), Level: logging.InfoLevel, Event: "memory.load", Message: "memory manager ready", Fields: logging.Fields{"dir": "./memory"}}, 100*time.Millisecond)
	send(LogMsg{Time: time.Now(), Level: logging.InfoLevel, Event: "agent.start", Message: "agent loop started"}, 100*time.Millisecond)

	send(ComponentStatusMsg{"player", publisher.StatusOK, "Running"}, 200*time.Millisecond)
}

func demoCycle1(p *tea.Program) {
	send := func(msg tea.Msg, delay time.Duration) {
		time.Sleep(delay)
		p.Send(msg)
	}

	send(AgentCycleMsg{Cycle: 1, Reason: "gatekeeper"}, 500*time.Millisecond)
	send(LogMsg{Time: time.Now(), Level: logging.InfoLevel, Event: "agent.wake", Message: "think cycle started", Fields: logging.Fields{"cycle": 1, "reason": "gatekeeper"}}, 100*time.Millisecond)
	send(LogMsg{Time: time.Now(), Level: logging.DebugLevel, Event: "perception.fetch", Message: "fetching vitals and surroundings"}, 200*time.Millisecond)

	thought1 := "I've just spawned into the world. I can see a plains biome stretching out around me with some oak trees to the north. " +
		"The sun is high overhead so I have plenty of daylight left. My health and hunger are full. " +
		"Let me look around more carefully to understand my surroundings before deciding on a plan."
	demoStreamText(p, thought1)

	send(ThinkingTextMsg{Cycle: 1, Round: 1, FullText: thought1}, 200*time.Millisecond)

	send(ToolExecMsg{Name: "look_around", ID: "tool_01", Input: json.RawMessage(`{"radius": 16}`)}, 300*time.Millisecond)
	send(LogMsg{Time: time.Now(), Level: logging.InfoLevel, Event: "tool.exec", Message: "executing tool", Fields: logging.Fields{"tool": "look_around"}}, 50*time.Millisecond)
	send(ToolResultMsg{
		Name: "look_around", ID: "tool_01",
		Result:  "You are standing in a plains biome at (42, 65, -128). Nearby: 3 oak trees (N, 8-12 blocks), a small pond (E, 15 blocks), 2 cows grazing (SW, 6 blocks), tall grass all around. No hostile mobs visible. Elevation is flat.",
		Elapsed: 340 * time.Millisecond,
	}, 500*time.Millisecond)

	thought2 := "Good, I can see the area clearly now. There are some oak trees to the north that I should harvest for wood -- " +
		"that's the first priority in any survival situation. The cows nearby could be useful for food later. " +
		"I'll navigate to the nearest oak tree and start gathering wood."
	demoStreamText(p, thought2)

	send(ThinkingTextMsg{Cycle: 1, Round: 2, FullText: thought2}, 200*time.Millisecond)

	send(ToolExecMsg{Name: "navigate_to", ID: "tool_02", Input: json.RawMessage(`{"x": 45, "y": 65, "z": -136}`)}, 300*time.Millisecond)
	send(LogMsg{Time: time.Now(), Level: logging.InfoLevel, Event: "tool.exec", Message: "executing tool", Fields: logging.Fields{"tool": "navigate_to"}}, 50*time.Millisecond)
	send(ToolResultMsg{
		Name: "navigate_to", ID: "tool_02",
		Result:  "Arrived at (45, 65, -136). Now standing next to an oak tree.",
		Elapsed: 2100 * time.Millisecond,
	}, 1*time.Second)

	send(TurnCompleteMsg{Cycle: 1, Round: 2, Stats: publisher.TurnStats{
		StopReason:  "end_turn",
		Model:       "claude-sonnet-4-20250514",
		TokensIn:    3842,
		TokensOut:   287,
		CacheCreate: 3200,
		CacheRead:   0,
		APIMillis:   1850,
		ToolMillis:  2440,
		ToolNames:   []string{"look_around", "navigate_to"},
	}}, 200*time.Millisecond)

	send(LogMsg{Time: time.Now(), Level: logging.InfoLevel, Event: "agent.turn", Message: "turn complete", Fields: logging.Fields{"cycle": 1, "rounds": 2, "tokens_in": 3842, "tokens_out": 287}}, 100*time.Millisecond)
}

func demoWorldEvents(p *tea.Program) {
	send := func(msg tea.Msg, delay time.Duration) {
		time.Sleep(delay)
		p.Send(msg)
	}

	send(GameEventMsg{Description: "Cow mooing nearby", Priority: 1, EventType: 1}, 400*time.Millisecond)
	send(GameEventMsg{Description: "Zombie spotted 18 blocks east", Priority: 2, EventType: 1}, 600*time.Millisecond)
	send(GameEventMsg{Description: "Night is falling -- hostile mobs will spawn soon", Priority: 3, EventType: 2}, 800*time.Millisecond)

	send(MemoryUpdateMsg{File: "journal.md", Preview: "Day 1: Spawned in plains biome near oak forest. Navigated to nearest tree to begin gathering wood."}, 500*time.Millisecond)
	send(MemoryUpdateMsg{File: "world-knowledge.md", Preview: "Oak forest at ~(45, 65, -136). Plains biome. Small pond to the east. Cows grazing to the southwest."}, 300*time.Millisecond)

	send(SidecarLogMsg{Time: time.Now(), Line: "entity.spawn: zombie at (60, 63, -125)", Stream: "stdout"}, 200*time.Millisecond)
	send(SidecarLogMsg{Time: time.Now(), Line: "time.update: dusk (tick 12000)", Stream: "stdout"}, 300*time.Millisecond)

	send(LogMsg{Time: time.Now(), Level: logging.DebugLevel, Event: "gatekeeper.eval", Message: "evaluating wake conditions", Fields: logging.Fields{"events": 3, "critical": 1}}, 200*time.Millisecond)
	send(GatekeeperDecisionMsg{Wake: false, Reason: "routine entity spawns, vitals stable"}, 300*time.Millisecond)
}

func demoChat(p *tea.Program) {
	send := func(msg tea.Msg, delay time.Duration) {
		time.Sleep(delay)
		p.Send(msg)
	}

	send(ChatMsg{Sender: "Steve", Message: "Hey Claude! Welcome to the server!", IsBot: false}, 800*time.Millisecond)
	send(ChatMsg{Sender: "claude", Message: "Thanks Steve! Just spawned in and gathering some wood. Want to team up?", IsBot: true}, 1200*time.Millisecond)
	send(ChatMsg{Sender: "Steve", Message: "Sure! I found a cave nearby with iron. Follow me when you're ready", IsBot: false}, 1500*time.Millisecond)
}

func demoCycle2(p *tea.Program) {
	send := func(msg tea.Msg, delay time.Duration) {
		time.Sleep(delay)
		p.Send(msg)
	}

	send(GatekeeperDecisionMsg{Wake: true, Reason: "player chat addressed to bot"}, 800*time.Millisecond)
	send(AgentCycleMsg{Cycle: 2, Reason: "bypass_chat"}, 300*time.Millisecond)
	send(LogMsg{Time: time.Now(), Level: logging.InfoLevel, Event: "agent.wake", Message: "think cycle started", Fields: logging.Fields{"cycle": 2, "reason": "bypass_chat"}}, 100*time.Millisecond)

	thought3 := "Steve wants to team up and explore a cave with iron! That's a great opportunity. " +
		"But first I should gather some wood to craft basic tools -- I'll need at least a wooden pickaxe before mining iron. " +
		"Let me harvest some oak logs from this tree."
	demoStreamText(p, thought3)

	send(ThinkingTextMsg{Cycle: 2, Round: 1, FullText: thought3}, 200*time.Millisecond)

	send(ToolExecMsg{Name: "harvest_block", ID: "tool_03", Input: json.RawMessage(`{"block_type": "oak_log", "count": 5}`)}, 300*time.Millisecond)
	send(LogMsg{Time: time.Now(), Level: logging.InfoLevel, Event: "tool.exec", Message: "executing tool", Fields: logging.Fields{"tool": "harvest_block"}}, 50*time.Millisecond)
	send(ToolResultMsg{
		Name: "harvest_block", ID: "tool_03",
		Result:  "pathfinder error: no reachable path to target block after 10s timeout",
		IsError: true,
		Elapsed: 10 * time.Second,
	}, 2*time.Second)
	send(LogMsg{Time: time.Now(), Level: logging.WarnLevel, Event: "tool.error", Message: "tool returned error", Fields: logging.Fields{"tool": "harvest_block", "elapsed": "10s"}}, 100*time.Millisecond)

	thought4 := "The pathfinder couldn't reach the oak log -- maybe I'm not positioned right. " +
		"Let me try a different approach and dig the block directly since I'm standing right next to the tree."
	demoStreamText(p, thought4)

	send(ThinkingTextMsg{Cycle: 2, Round: 2, FullText: thought4}, 200*time.Millisecond)

	send(ToolExecMsg{Name: "dig_block", ID: "tool_04", Input: json.RawMessage(`{"x": 45, "y": 66, "z": -136}`)}, 300*time.Millisecond)
	send(ToolResultMsg{
		Name: "dig_block", ID: "tool_04",
		Result:  "Dug oak_log at (45, 66, -136). Collected 1 oak_log.",
		Elapsed: 1200 * time.Millisecond,
	}, 800*time.Millisecond)

	send(TurnCompleteMsg{Cycle: 2, Round: 2, Stats: publisher.TurnStats{
		StopReason:  "end_turn",
		Model:       "claude-sonnet-4-20250514",
		TokensIn:    5190,
		TokensOut:   342,
		CacheCreate: 1200,
		CacheRead:   3200,
		APIMillis:   2100,
		ToolMillis:  11200,
		ToolNames:   []string{"harvest_block", "dig_block"},
	}}, 200*time.Millisecond)

	send(ComponentStatusMsg{"player", publisher.StatusOK, "Idle"}, 300*time.Millisecond)
	send(LogMsg{Time: time.Now(), Level: logging.InfoLevel, Event: "agent.turn", Message: "turn complete", Fields: logging.Fields{"cycle": 2, "rounds": 2, "tokens_in": 5190, "tokens_out": 342}}, 100*time.Millisecond)
}

func demoExecCmd(cmd string) (string, error) {
	time.Sleep(200 * time.Millisecond)
	switch cmd {
	case "list":
		return "There are 2 of a max of 20 players online: claude, Steve", nil
	case "time query daytime":
		return "The time is 6000", nil
	default:
		return "Unknown or incomplete command", nil
	}
}

func demoStreamText(p *tea.Program, text string) {
	const (
		chunkSize = 10
		interval  = 40 * time.Millisecond
	)
	for i := 0; i < len(text); i += chunkSize {
		end := min(i+chunkSize, len(text))
		p.Send(TextDeltaMsg{Delta: text[i:end]})
		time.Sleep(interval)
	}
}
