package agent

import (
	"fmt"
	"sync"
	"time"
)

// CancelReason identifies why an in-flight tool was interrupted.
type CancelReason int

// CancelReason constants for tool interruptions.
const (
	CancelEmergencyStop CancelReason = iota + 1
	CancelCriticalEvent
)

func (r CancelReason) String() string {
	switch r {
	case CancelEmergencyStop:
		return "emergency stop"
	case CancelCriticalEvent:
		return "critical game event"
	default:
		return "unknown"
	}
}

// toolInterruptError is the cause set on a tool context when it is interrupted.
// It carries a structured reason so the agent can report what happened
// accurately instead of confabulating.
type toolInterruptError struct {
	Reason CancelReason
	Detail string
}

func (e *toolInterruptError) Error() string {
	return fmt.Sprintf("tool interrupted: %s -- %s", e.Reason, e.Detail)
}

// toolFlight tracks whether a tool call is currently blocking the think loop.
// Written by the main goroutine (executeTool); read by the event loop and
// perception loop goroutines.
type toolFlight struct {
	mu        sync.Mutex
	name      string
	startedAt time.Time
	cancelFn  func(error)
}

// enter registers a tool as in-flight. cancel is the CancelCauseFunc from
// context.WithCancelCause -- interrupt passes a *toolInterruptError through it.
func (f *toolFlight) enter(name string, cancel func(error)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.name = name
	f.startedAt = time.Now()
	f.cancelFn = cancel
}

// exit clears the in-flight state. Must be deferred after enter.
func (f *toolFlight) exit() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.name = ""
	f.startedAt = time.Time{}
	f.cancelFn = nil
}

// active reports whether a tool is currently in flight.
func (f *toolFlight) active() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.name != ""
}

// snapshot returns the in-flight tool's name and elapsed time. ok is false
// when no tool is in flight.
func (f *toolFlight) snapshot() (name string, elapsed time.Duration, ok bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.name == "" {
		return "", 0, false
	}
	return f.name, time.Since(f.startedAt), true
}

// interrupt cancels the in-flight tool's context with a structured reason.
// Returns true if a tool was actually interrupted, false if nothing was in
// flight.
func (f *toolFlight) interrupt(reason CancelReason, detail string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cancelFn == nil {
		return false
	}
	f.cancelFn(&toolInterruptError{Reason: reason, Detail: detail})
	return true
}
