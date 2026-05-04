package agent

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestToolFlightInitiallyInactive(t *testing.T) {
	f := &toolFlight{}
	if f.active() {
		t.Fatal("expected inactive on init")
	}
	if _, _, ok := f.snapshot(); ok {
		t.Fatal("expected no snapshot on init")
	}
}

func TestToolFlightEnterExit(t *testing.T) {
	f := &toolFlight{}

	f.enter("move_to", func(error) {})
	if !f.active() {
		t.Fatal("expected active after enter")
	}

	name, elapsed, ok := f.snapshot()
	if !ok {
		t.Fatal("expected snapshot after enter")
	}
	if name != "move_to" {
		t.Fatalf("expected name move_to, got %s", name)
	}
	if elapsed < 0 {
		t.Fatalf("expected non-negative elapsed, got %v", elapsed)
	}

	f.exit()
	if f.active() {
		t.Fatal("expected inactive after exit")
	}
}

func TestToolFlightInterruptWhileActive(t *testing.T) {
	f := &toolFlight{}
	var called atomic.Bool
	f.enter("navigate_to", func(error) { called.Store(true) })

	if !f.interrupt(CancelEmergencyStop, "test") {
		t.Fatal("expected interrupt to return true when active")
	}
	if !called.Load() {
		t.Fatal("expected cancel func to be called")
	}
}

func TestToolFlightInterruptCarriesReason(t *testing.T) {
	f := &toolFlight{}
	var gotErr error
	f.enter("move_to", func(err error) { gotErr = err })

	f.interrupt(CancelCriticalEvent, "bot death")

	var ie *toolInterruptError
	if !errors.As(gotErr, &ie) {
		t.Fatalf("expected toolInterruptError, got %v", gotErr)
	}
	if ie.Reason != CancelCriticalEvent {
		t.Errorf("reason = %v, want CancelCriticalEvent", ie.Reason)
	}
	if ie.Detail != "bot death" {
		t.Errorf("detail = %q, want %q", ie.Detail, "bot death")
	}
}

func TestToolFlightInterruptWhileInactive(t *testing.T) {
	f := &toolFlight{}
	if f.interrupt(CancelEmergencyStop, "test") {
		t.Fatal("expected interrupt to return false when inactive")
	}
}

func TestToolFlightExitPreventsInterrupt(t *testing.T) {
	f := &toolFlight{}
	var called atomic.Bool
	f.enter("move_to", func(error) { called.Store(true) })
	f.exit()

	if f.interrupt(CancelEmergencyStop, "test") {
		t.Fatal("expected interrupt to return false after exit")
	}
	if called.Load() {
		t.Fatal("cancel func should not be called after exit")
	}
}

func TestToolFlightConcurrentAccess(t *testing.T) {
	f := &toolFlight{}
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			f.enter("tool", func(error) {})
			time.Sleep(time.Microsecond)
			f.exit()
		}()
		go func() {
			defer wg.Done()
			f.active()
		}()
		go func() {
			defer wg.Done()
			f.interrupt(CancelEmergencyStop, "test")
		}()
	}

	wg.Wait()

	if f.active() {
		t.Fatal("expected inactive after all goroutines complete")
	}
}

func TestCancelReasonString(t *testing.T) {
	tests := []struct {
		reason CancelReason
		want   string
	}{
		{CancelEmergencyStop, "emergency stop"},
		{CancelCriticalEvent, "critical game event"},
		{CancelReason(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.reason.String(); got != tt.want {
			t.Errorf("CancelReason(%d).String() = %q, want %q", tt.reason, got, tt.want)
		}
	}
}
