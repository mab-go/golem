package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mab-go/golem/internal/claude"
	"github.com/mab-go/golem/internal/game"
	"github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/logging"
	"github.com/mab-go/golem/internal/perception"
	"github.com/mab-go/golem/internal/publisher"
)

// Run starts the event subscription, perception loop, and heartbeat, then
// blocks on wake signals until ctx is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	a.log.Info(eventStart)
	go a.eventLoop(ctx)
	go a.perceptionLoop(ctx)
	go a.heartbeatLoop(ctx)

	for {
		select {
		case <-ctx.Done():
			a.log.Info(eventStop)
			return nil
		case sig := <-a.wakeChan:
			if a.pacing.Paused() {
				continue
			}
			if err := a.think(ctx, sig); err != nil {
				if ctx.Err() != nil {
					a.log.Info(eventStop)
					return nil
				}
				a.log.WithError(err).Error(eventCycleFailed)
				a.sleepFor(ctx, 5*time.Second)
			}
		}
	}
}

// think runs one Perceive -> Think -> Act -> Remember cycle.
func (a *Agent) think(ctx context.Context, sig WakeSignal) error {
	a.cycle++
	cycle := a.cycle
	a.lastWakeAt.Store(time.Now().UnixNano())
	a.pub.PublishAgentCycle(cycle, sig.Reason.String())

	a.log.WithFields(logging.Fields{
		"cycle":  cycle,
		"reason": sig.Reason.String(),
	}).Info(eventWake)

	snap := a.assemblePerception(ctx, sig)

	userMsg, err := a.builder.BuildUserMessage(snap)
	if err != nil {
		return fmt.Errorf("build user message: %w", err)
	}
	a.history.Append(userMsg)

	sysParts := claude.SystemPromptParts(claude.SystemPromptConfig{
		Verbosity:        a.pacing.Verbosity().String(),
		PerceptionFormat: formatName(a.cfg.PerceptionFormat),
		BotUsername:      a.cfg.BotUsername,
		MemoryDir:        a.memory.Dir(),
	})

	for round := 0; round < a.cfg.MaxToolRounds; round++ {
		done, err := a.runToolRound(ctx, cycle, round, sysParts)
		if done || err != nil {
			return err
		}
	}
	return nil
}

// assemblePerception gathers all inputs for a think cycle: drains queued
// chat and events, fetches game state, classifies events when needed, and
// attaches task status and heartbeat notes.
func (a *Agent) assemblePerception(ctx context.Context, sig WakeSignal) claude.PerceptionSnapshot {
	chatMsgs := a.drainChat()

	var snap claude.PerceptionSnapshot
	if sig.GatekeeperSnap != nil {
		gs := sig.GatekeeperSnap
		snap = a.fetchPerception(ctx)
		snap.Events = gs.Events
		snap.ClassifiedEvents = gs.ClassifiedEvents
	} else {
		events := a.drainEvents()
		snap = a.fetchPerception(ctx)
		snap.Events = events

		if sig.Reason == WakeBypassCritical && len(events) > 0 {
			classCtx, classCancel := context.WithTimeout(ctx, 5*time.Second)
			classified, err := a.ai.ClassifyEvents(classCtx, events)
			classCancel()
			if err != nil {
				a.log.WithError(err).Debug(eventClassifyFailed)
			}
			snap.ClassifiedEvents = classified
		}
	}

	snap.ActiveTask = a.taskMgr.Status()
	if len(chatMsgs) > 0 {
		snap.ChatMessages = make([]claude.ChatMessage, len(chatMsgs))
		for i, cm := range chatMsgs {
			snap.ChatMessages[i] = claude.ChatMessage{Sender: cm.Sender, Message: cm.Message}
		}
	}

	if sig.Reason == WakeHeartbeat {
		elapsed := time.Duration(time.Now().UnixNano()-a.lastWakeAt.Load()) * time.Nanosecond
		snap.HeartbeatNote = fmt.Sprintf("[Heartbeat -- no significant events. Time elapsed: %.0fs.]", elapsed.Seconds())
	}

	return snap
}

// runToolRound sends one API request, processes the response, and executes
// any tool calls. Returns done=true when the model's turn is complete (no
// more tool calls), or an error if the API call failed.
func (a *Agent) runToolRound(ctx context.Context, cycle uint64, round int, sysParts claude.CacheableSystemPrompt) (bool, error) {
	msgs := a.history.Snapshot()
	apiStart := time.Now()
	resp, err := a.ai.SendMessageParts(ctx, claude.ModelPlayer, sysParts, msgs, a.tools)
	apiMS := time.Since(apiStart).Milliseconds()
	if err != nil {
		return false, fmt.Errorf("send message (cycle %d round %d): %w", cycle, round, err)
	}

	a.history.Append(assistantMessageFromResponse(resp))

	if resp.Text != "" {
		a.pub.PublishThinkingText(cycle, round, resp.Text)
	}

	if len(resp.ToolUses) == 0 || resp.StopReason != "tool_use" {
		a.logAndPublishTurn(cycle, round, resp, apiMS, 0)
		return true, nil
	}

	resultBlocks, toolMS := a.executeToolUses(ctx, resp.ToolUses)
	a.history.Append(claude.Message{Role: claude.RoleUser, Content: resultBlocks})

	a.logAndPublishTurn(cycle, round, resp, apiMS, toolMS)
	return false, nil
}

func (a *Agent) executeToolUses(ctx context.Context, uses []claude.ToolUse) ([]claude.Block, int64) {
	resultBlocks := make([]claude.Block, 0, len(uses))
	var toolMS int64
	for _, use := range uses {
		a.pub.PublishToolExec(use.Name, use.ID, use.Input)
		toolStart := time.Now()
		result, isError := a.executeTool(ctx, use)
		toolDuration := time.Since(toolStart)
		elapsed := toolDuration.Milliseconds()
		toolMS += elapsed

		a.log.WithFields(logging.Fields{
			"tool":        use.Name,
			"tool_use_id": use.ID,
			"result_ok":   !isError,
			"result":      truncateResult(result.Text, 500),
			"tool_ms":     elapsed,
			"has_image":   len(result.ImagePNG) > 0,
		}).Debug(eventToolResult)
		a.pub.PublishToolResult(use.Name, use.ID, result.Text, isError, toolDuration)

		block := claude.Block{
			Type:      claude.BlockToolResult,
			ToolUseID: use.ID,
			Content:   result.Text,
			IsError:   isError,
		}
		if len(result.ImagePNG) > 0 {
			block.ImageMediaType = "image/png"
			block.ImageData = result.ImagePNG
		}
		resultBlocks = append(resultBlocks, block)
	}
	if midRound := a.buildMidRoundText(); midRound != "" {
		resultBlocks = append(resultBlocks, claude.Block{
			Type: claude.BlockText,
			Text: midRound,
		})
	}
	return resultBlocks, toolMS
}

func (a *Agent) logAndPublishTurn(cycle uint64, round int, resp *claude.Response, apiMS, toolMS int64) {
	a.log.WithFields(logging.Fields{
		"cycle":        cycle,
		"round":        round,
		"stop_reason":  resp.StopReason,
		"tool_uses":    formatToolUsesForLog(resp.ToolUses),
		"tokens_in":    resp.Usage.InputTokens,
		"tokens_out":   resp.Usage.OutputTokens,
		"cache_create": resp.Usage.CacheCreationInput,
		"cache_read":   resp.Usage.CacheReadInput,
		"api_ms":       apiMS,
		"tool_ms":      toolMS,
		"model":        resp.Model,
	}).Info(eventTurnComplete)
	a.pub.PublishTurnComplete(cycle, round, publisher.TurnStats{
		StopReason:  resp.StopReason,
		Model:       resp.Model,
		TokensIn:    resp.Usage.InputTokens,
		TokensOut:   resp.Usage.OutputTokens,
		CacheCreate: resp.Usage.CacheCreationInput,
		CacheRead:   resp.Usage.CacheReadInput,
		APIMillis:   apiMS,
		ToolMillis:  toolMS,
		ToolNames:   extractToolNames(resp.ToolUses),
	})
}

// executeTool dispatches one tool_use, returning the Result (text + optional
// image) and an isError flag.
func (a *Agent) executeTool(ctx context.Context, use claude.ToolUse) (game.Result, bool) {
	log := a.log.WithField("tool", use.Name).WithField("tool_use_id", use.ID)
	log.WithField("input", string(use.Input)).Debug(eventToolExec)

	var timeout time.Duration
	if use.Name == claude.ToolThinkDeeply {
		timeout = 60 * time.Second
	} else {
		timeout = a.dispatcher.Timeout(use.Name, use.Input)
	}

	toolCtx, cancel := context.WithCancelCause(ctx)
	timer := time.AfterFunc(timeout, func() { cancel(context.DeadlineExceeded) })
	defer timer.Stop()
	defer cancel(nil)

	a.flight.enter(use.Name, cancel)
	defer a.flight.exit()

	if use.Name == claude.ToolThinkDeeply {
		result, err := a.handleThinkDeeply(toolCtx, use.Input)
		if err != nil {
			var ie *toolInterruptError
			if errors.As(context.Cause(toolCtx), &ie) {
				log.Info(eventToolInterrupted)
				return interruptedResult(use.Name, ie), false
			}
			log.WithError(err).Warn(eventThinkDeepFailed)
			return game.Result{Text: "escalation error: " + err.Error()}, true
		}
		return result, false
	}

	result, err := a.dispatcher.ExecuteResult(toolCtx, use.Name, use.Input)
	if err != nil {
		var ie *toolInterruptError
		if errors.As(context.Cause(toolCtx), &ie) {
			log.Info(eventToolInterrupted)
			return interruptedResult(use.Name, ie), false
		}
		log.WithError(err).Warn(eventToolFailed)
		return game.Result{Text: "transport error: " + err.Error()}, true
	}
	return result, false
}

func truncateResult(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "...(truncated)"
}

// buildMidRoundText drains any events and chat that arrived during tool
// execution and formats them as a text block for injection between rounds.
func (a *Agent) buildMidRoundText() string {
	events := a.drainEvents()
	chats := a.drainChat()
	if len(events) == 0 && len(chats) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("[Mid-round update]\n\n")

	if len(events) > 0 {
		b.WriteString("New events since last tool call:\n")
		b.WriteString(a.formatter.FormatEvents(events))
		b.WriteString("\n\n")
	}

	if len(chats) > 0 {
		b.WriteString("Player chat:\n")
		for _, cm := range chats {
			fmt.Fprintf(&b, "%s: %q\n", cm.Sender, cm.Message)
		}
	}

	return b.String()
}

func interruptedResult(toolName string, ie *toolInterruptError) game.Result {
	return game.Result{
		Text: fmt.Sprintf("%s was interrupted: %s. %s", toolName, ie.Reason, ie.Detail),
	}
}

// handleThinkDeeply escalates to Opus via the think_deeply tool. It assembles
// a compact context summary from memory and current perception, then calls
// the strategic advisor model.
func (a *Agent) handleThinkDeeply(ctx context.Context, input json.RawMessage) (game.Result, error) {
	var params struct {
		Situation string `json:"situation"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return game.Result{Text: "invalid think_deeply input: " + err.Error()}, nil
	}

	state, _ := a.memory.ReadAll()
	vitals, surr := a.fetchLightPerception(ctx)

	var summary strings.Builder
	summary.WriteString("## Current State\n\n")
	fmt.Fprintf(&summary, "Vitals: %s\n", a.formatter.FormatVitalSigns(vitals))
	if surr != nil {
		fmt.Fprintf(&summary, "Position: (%.0f, %.0f, %.0f) in %s, %s\n",
			surr.Position.GetX(), surr.Position.GetY(), surr.Position.GetZ(),
			surr.GetBiome(), surr.GetTimeOfDay())
	}
	summary.WriteString("\n## Goals\n\n")
	summary.WriteString(state.Goals)
	summary.WriteString("\n\n## Recent Journal\n\n")
	summary.WriteString(tailJournal(state.Journal, 20))
	summary.WriteString("\n\n## World Knowledge\n\n")
	summary.WriteString(state.WorldKnowledge)

	start := time.Now()
	resp, err := a.ai.ThinkDeeply(ctx, params.Situation, summary.String())
	elapsed := time.Since(start)
	if err != nil {
		return game.Result{}, fmt.Errorf("opus escalation: %w", err)
	}

	a.log.WithFields(logging.Fields{
		"model":      resp.Model,
		"tokens_in":  resp.Usage.InputTokens,
		"tokens_out": resp.Usage.OutputTokens,
		"latency_ms": elapsed.Milliseconds(),
	}).Info(eventThinkDeepDone)

	return game.Result{Text: resp.Text}, nil
}

// tailJournal returns the last n lines of a journal string.
func tailJournal(s string, n int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// assistantMessageFromResponse converts the Response's blocks into a history
// Message for round-tripping back to the API.
func assistantMessageFromResponse(resp *claude.Response) claude.Message {
	content := make([]claude.Block, 0, len(resp.Blocks))
	for _, b := range resp.Blocks {
		switch b.Type {
		case claude.BlockText:
			if b.Text == "" {
				continue
			}
			content = append(content, claude.Block{Type: claude.BlockText, Text: b.Text})
		case claude.BlockToolUse:
			content = append(content, claude.Block{
				Type:  claude.BlockToolUse,
				ID:    b.ID,
				Name:  b.Name,
				Input: cloneRaw(b.Input),
			})
		}
	}
	return claude.Message{Role: claude.RoleAssistant, Content: content}
}

func cloneRaw(r json.RawMessage) json.RawMessage {
	if len(r) == 0 {
		return nil
	}
	out := make(json.RawMessage, len(r))
	copy(out, r)
	return out
}

// ---------------------------------------------------------------------------
// Event loop
// ---------------------------------------------------------------------------

// eventLoop subscribes to sidecar events and dispatches them. Reconnects with
// a short backoff on stream errors; exits when ctx is cancelled.
func (a *Agent) eventLoop(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		stream, err := a.client.SubscribeEvents(ctx)
		if err != nil {
			a.log.WithError(err).Warn(eventSubscribeFailed)
			if !a.sleepFor(ctx, backoff) {
				return
			}
			backoff = minDuration(backoff*2, 30*time.Second)
			continue
		}
		backoff = time.Second
		a.readEvents(ctx, stream)
	}
}

func (a *Agent) readEvents(ctx context.Context, stream pb.MinecraftService_SubscribeEventsClient) {
	for {
		evt, err := stream.Recv()
		if err != nil {
			if ctx.Err() != nil || err == io.EOF {
				return
			}
			a.log.WithError(err).Warn(eventStreamBroke)
			return
		}
		a.handleEvent(ctx, evt)
	}
}

func (a *Agent) handleEvent(ctx context.Context, evt *pb.GameEvent) {
	if evt == nil {
		return
	}
	a.pub.PublishGameEvent(evt.Description, int32(evt.Priority), int32(evt.Type))

	if evt.Type == pb.EventType_EVENT_CHAT_MESSAGE {
		a.handleChatEvent(ctx, evt)
		return
	}

	a.appendEvent(evt)

	if evt.Priority == pb.EventPriority_EVENT_PRIORITY_CRITICAL {
		a.sendWake(WakeSignal{Reason: WakeBypassCritical})
		a.flight.interrupt(CancelCriticalEvent, evt.Description)
		return
	}

	if evt.Type == pb.EventType_EVENT_TASK_PROGRESS && isTerminalTaskDesc(evt.Description) {
		a.sendWake(WakeSignal{Reason: WakeBypassTask})
	}
}

func (a *Agent) handleChatEvent(ctx context.Context, evt *pb.GameEvent) {
	a.pub.PublishChat(evt.PlayerName, evt.ChatMessage, evt.PlayerName == a.cfg.BotUsername)
	res := a.chatHandler.Handle(ctx, evt)
	if res.EmergencyStop {
		a.log.WithField("sender", evt.PlayerName).Info(eventEmergencyStop)
		a.flight.interrupt(CancelEmergencyStop,
			fmt.Sprintf("player %s issued /golem stop", evt.PlayerName))
		a.sendWake(WakeSignal{Reason: WakeBypassCritical})
		return
	}
	if res.Handled {
		a.log.WithFields(logging.Fields{
			"sender":  evt.PlayerName,
			"message": truncForLog(evt.ChatMessage, 120),
		}).Debug(eventChatFiltered)
		return
	}
	a.appendChat(res.HumanMessage, res.Sender)
	a.appendEvent(evt)
	a.sendWake(WakeSignal{Reason: WakeBypassChat})
	a.log.WithFields(logging.Fields{
		"sender":  res.Sender,
		"message": truncForLog(res.HumanMessage, 120),
	}).Debug(eventChatSurfaced)
}

// ---------------------------------------------------------------------------
// Perception loop (gatekeeper)
// ---------------------------------------------------------------------------

// perceptionLoop ticks every PerceptionTickInterval, fetches lightweight
// perception, and calls the Haiku gatekeeper to decide whether to wake the
// think loop.
func (a *Agent) perceptionLoop(ctx context.Context) {
	ticker := time.NewTicker(a.cfg.PerceptionTickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.perceptionTick(ctx)
		}
	}
}

func (a *Agent) perceptionTick(ctx context.Context) {
	if a.flight.active() {
		a.log.Debug(eventGatekeeperSkipToolFlight)
		return
	}

	events := a.drainEvents()

	if containsBypass(events) {
		return
	}

	vitals, surr := a.fetchLightPerception(ctx)

	if a.shouldThrottleRoutine(events, vitals) {
		if len(events) > 0 {
			a.log.WithField("routine_events", len(events)).Debug(eventGatekeeperSkipRoutine)
		}
		return
	}

	snap := a.buildGatekeeperSnapshot(vitals, surr, events)

	gkCtx, gkCancel := context.WithTimeout(ctx, a.cfg.GatekeeperTimeout)
	decision, err := a.ai.GatekeeperCheck(gkCtx, snap)
	gkCancel()
	a.gkHealth.record(err != nil)

	if err != nil {
		a.log.WithError(err).Debug(eventGatekeeperFailed)
	}

	a.pub.PublishGatekeeperDecision(decision.Wake, decision.Reason)

	if decision.Wake {
		a.log.WithField("reason", decision.Reason).Debug(eventGatekeeperWake)
		a.sendWake(WakeSignal{
			Reason: WakeGatekeeper,
			GatekeeperSnap: &GatekeeperResult{
				Events:           events,
				ClassifiedEvents: decision.ClassifiedEvents,
				Vitals:           vitals,
				Surroundings:     surr,
				Reason:           decision.Reason,
			},
		})
	} else {
		a.log.WithField("reason", decision.Reason).Debug(eventGatekeeperIdle)
	}
}

func (a *Agent) shouldThrottleRoutine(events []*pb.GameEvent, vitals *pb.GetVitalSignsResponse) bool {
	if !routineEventsOnly(events) || vitals == nil || vitals.Health <= 10 || vitals.Food <= 6 {
		return false
	}
	sinceWake := time.Duration(time.Now().UnixNano()-a.lastWakeAt.Load()) * time.Nanosecond
	return sinceWake < a.cfg.HeartbeatInterval/2
}

func (a *Agent) buildGatekeeperSnapshot(
	vitals *pb.GetVitalSignsResponse,
	surr *pb.GetSurroundingsResponse,
	events []*pb.GameEvent,
) claude.GatekeeperSnapshot {
	var position *pb.Vec3
	var biome, timeOfDay string
	var nearbyEntities []*pb.Entity
	if surr != nil {
		position = surr.Position
		biome = surr.Biome
		timeOfDay = surr.TimeOfDay
		nearbyEntities = surr.NearbyEntities
	}

	sinceWake := time.Duration(time.Now().UnixNano()-a.lastWakeAt.Load()) * time.Nanosecond

	return claude.GatekeeperSnapshot{
		Vitals:         vitals,
		Position:       position,
		Biome:          biome,
		TimeOfDay:      timeOfDay,
		NearbyEntities: nearbyEntities,
		ActiveTask:     a.taskMgr.Status(),
		Events:         events,
		TimeSinceWake:  sinceWake,
		GoalsSummary:   a.readGoalsSummary(),
	}
}

// readGoalsSummary returns the first line of goals.md.
func (a *Agent) readGoalsSummary() string {
	state, err := a.memory.ReadAll()
	if err != nil {
		return ""
	}
	goals := strings.TrimSpace(state.Goals)
	if goals == "" {
		return ""
	}
	if idx := strings.IndexByte(goals, '\n'); idx >= 0 {
		return goals[:idx]
	}
	return goals
}

// ---------------------------------------------------------------------------
// Heartbeat loop
// ---------------------------------------------------------------------------

// heartbeatLoop fires periodically to ensure temporal awareness even during
// quiet periods.
func (a *Agent) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(a.cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sinceWake := time.Duration(time.Now().UnixNano()-a.lastWakeAt.Load()) * time.Nanosecond
			if sinceWake < a.cfg.HeartbeatInterval/2 {
				continue
			}
			a.log.Debug(eventHeartbeat)
			a.sendWake(WakeSignal{Reason: WakeHeartbeat})
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func truncForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func formatName(f perception.Format) string {
	switch f {
	case perception.FormatStructured:
		return "structured"
	default:
		return "prose"
	}
}
