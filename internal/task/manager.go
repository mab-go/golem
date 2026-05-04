package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	sidecar "github.com/mab-go/golem/internal/grpc"
	pb "github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/logging"
	"github.com/mab-go/golem/internal/perception"
)

const eventTaskError logging.Event = "agent.task_error"

// Manager owns the single active-task slot. At most one Tier 2
// background task may run at a time (the bot has one body). The manager
// drains the gRPC streaming channel in a goroutine, emits synthetic
// GameEvents into the agent's event buffer, and wakes the think loop on
// terminal status.
type Manager struct {
	mu        sync.Mutex
	active    *activeTask
	parentCtx context.Context
	eventSink func(*pb.GameEvent)
	wakeFn    func()
	timeout   time.Duration
	log       logging.Logger
}

type activeTask struct {
	Name      string
	TaskID    string
	StartedAt time.Time
	Cancel    context.CancelFunc
	Current   int32
	Total     int32
	Message   string
	Done      chan struct{}
}

// NewManager constructs a Manager. parentCtx is the agent's long-lived
// context used to derive per-task contexts that outlive individual tool
// executions. eventSink receives synthetic GameEvents (task
// progress/completion); wakeFn wakes the think loop.
func NewManager(parentCtx context.Context, eventSink func(*pb.GameEvent), wakeFn func(), timeout time.Duration, log logging.Logger) *Manager {
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	return &Manager{
		parentCtx: parentCtx,
		eventSink: eventSink,
		wakeFn:    wakeFn,
		timeout:   timeout,
		log:       log,
	}
}

// IsActive reports whether a background task is running.
func (tm *Manager) IsActive() bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.active != nil
}

// ActiveName returns the name of the running task, or "".
func (tm *Manager) ActiveName() string {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if tm.active == nil {
		return ""
	}
	return tm.active.Name
}

// NewTaskContext returns a context derived from the agent's long-lived
// parent context with the configured task timeout. Tier 2 handlers use
// this for both the gRPC streaming call and Start(), decoupling from the
// short-lived tool-execution context.
func (tm *Manager) NewTaskContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(tm.parentCtx, tm.timeout)
}

// Start launches a background task that drains ch. The caller must
// provide a task-scoped context and its cancel function (from
// NewTaskContext). Returns an error if a task is already active; the
// caller should return the error text as the tool_result for Claude and
// call cancel to release the context.
func (tm *Manager) Start(ctx context.Context, cancel context.CancelFunc, name string, ch <-chan sidecar.TaskEvent) error {
	tm.mu.Lock()
	if tm.active != nil {
		running := tm.active.Name
		tm.mu.Unlock()
		return fmt.Errorf("cannot start %s -- %s is still active. Cancel it first (cancel_task) or wait for completion", name, running)
	}
	at := &activeTask{
		Name:      name,
		StartedAt: time.Now(),
		Cancel:    cancel,
		Done:      make(chan struct{}),
	}
	tm.active = at
	tm.mu.Unlock()

	go tm.drainTask(ctx, at, ch)
	return nil
}

// Cancel aborts the active task and waits for the drainer to finish.
// Returns a human-readable status string.
func (tm *Manager) Cancel() string {
	tm.mu.Lock()
	at := tm.active
	if at == nil {
		tm.mu.Unlock()
		return "No active task to cancel."
	}
	name := at.Name
	at.Cancel()
	tm.mu.Unlock()

	select {
	case <-at.Done:
	case <-time.After(5 * time.Second):
	}
	return fmt.Sprintf("Cancelled %s.", name)
}

// Status returns a snapshot of the active task for perception rendering.
// Returns nil when idle.
func (tm *Manager) Status() *perception.TaskStatus {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if tm.active == nil {
		return nil
	}
	return &perception.TaskStatus{
		Name:    tm.active.Name,
		Current: tm.active.Current,
		Total:   tm.active.Total,
		Elapsed: time.Since(tm.active.StartedAt),
		Message: tm.active.Message,
	}
}

// drainTask reads from the task event channel, updates progress fields,
// emits synthetic GameEvents, and clears the active slot on completion.
func (tm *Manager) drainTask(ctx context.Context, at *activeTask, ch <-chan sidecar.TaskEvent) {
	defer at.Cancel()
	var lastStatus pb.TaskStatus

	for {
		select {
		case ev, ok := <-ch:
			status, done := tm.processTaskEvent(at, ev, ok, lastStatus)
			lastStatus = status
			if done {
				return
			}

		case <-ctx.Done():
			if isTerminal(lastStatus) {
				return
			}
			reason := "cancelled"
			if errors.Is(context.Cause(ctx), context.DeadlineExceeded) {
				reason = "timed out"
			}
			desc := fmt.Sprintf("[%s] %s", at.Name, reason)
			tm.emitEvent(at, pb.EventPriority_EVENT_PRIORITY_NORMAL, desc)
			tm.clearActive(at)
			return
		}
	}
}

func (tm *Manager) processTaskEvent(at *activeTask, ev sidecar.TaskEvent, ok bool, lastStatus pb.TaskStatus) (pb.TaskStatus, bool) {
	if !ok {
		tm.emitTerminal(at, "Task channel closed", lastStatus)
		return lastStatus, true
	}
	if ev.Err != nil {
		tm.log.WithField("task", at.Name).WithError(ev.Err).Warn(eventTaskError)
		tm.emitEvent(at, pb.EventPriority_EVENT_PRIORITY_NORMAL,
			fmt.Sprintf("[%s] transport error: %v", at.Name, ev.Err))
		tm.clearActive(at)
		return lastStatus, true
	}
	p := ev.Progress
	if p == nil {
		return lastStatus, false
	}
	if p.TaskId != "" {
		at.TaskID = p.TaskId
	}

	tm.mu.Lock()
	at.Current = p.Current
	at.Total = p.Total
	if p.Message != "" {
		at.Message = p.Message
	}
	tm.mu.Unlock()

	if isTerminal(p.Status) {
		label := taskStatusLabel(p.Status)
		desc := fmt.Sprintf("[%s] %s: %s", at.Name, label, p.Message)
		tm.emitEvent(at, pb.EventPriority_EVENT_PRIORITY_NORMAL, desc)
		tm.clearActive(at)
		return p.Status, true
	}

	desc := fmt.Sprintf("[%s] %d/%d -- %s", at.Name, p.Current, p.Total, p.Message)
	tm.emitEvent(at, pb.EventPriority_EVENT_PRIORITY_LOW, desc)
	return p.Status, false
}

func (tm *Manager) emitTerminal(at *activeTask, msg string, last pb.TaskStatus) {
	if !isTerminal(last) {
		tm.emitEvent(at, pb.EventPriority_EVENT_PRIORITY_NORMAL,
			fmt.Sprintf("[%s] %s", at.Name, msg))
	}
	tm.clearActive(at)
}

func (tm *Manager) clearActive(at *activeTask) {
	tm.mu.Lock()
	if tm.active == at {
		tm.active = nil
	}
	tm.mu.Unlock()
	close(at.Done)
	tm.wakeFn()
}

func (tm *Manager) emitEvent(_ *activeTask, priority pb.EventPriority, desc string) {
	tm.eventSink(&pb.GameEvent{
		Type:        pb.EventType_EVENT_TASK_PROGRESS,
		Priority:    priority,
		Timestamp:   time.Now().UnixMilli(),
		Description: desc,
	})
}

func isTerminal(s pb.TaskStatus) bool {
	switch s {
	case pb.TaskStatus_TASK_COMPLETED, pb.TaskStatus_TASK_FAILED, pb.TaskStatus_TASK_CANCELLED:
		return true
	}
	return false
}

func taskStatusLabel(s pb.TaskStatus) string {
	switch s {
	case pb.TaskStatus_TASK_COMPLETED:
		return "completed"
	case pb.TaskStatus_TASK_FAILED:
		return "failed"
	case pb.TaskStatus_TASK_CANCELLED:
		return "cancelled"
	default:
		return "unknown"
	}
}
