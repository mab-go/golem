package agent

import (
	"testing"
)

func TestWakeReasonString(t *testing.T) {
	tests := []struct {
		reason WakeReason
		want   string
	}{
		{WakeGatekeeper, "gatekeeper"},
		{WakeBypassChat, "bypass_chat"},
		{WakeBypassCritical, "bypass_critical"},
		{WakeBypassTask, "bypass_task"},
		{WakeHeartbeat, "heartbeat"},
		{WakeReason(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.reason.String(); got != tt.want {
			t.Errorf("WakeReason(%d).String() = %q, want %q", tt.reason, got, tt.want)
		}
	}
}

func TestSendWakeCoalesces(t *testing.T) {
	ch := make(chan WakeSignal, 1)

	send := func(sig WakeSignal) {
		select {
		case ch <- sig:
		default:
		}
	}

	send(WakeSignal{Reason: WakeBypassChat})
	send(WakeSignal{Reason: WakeBypassCritical})
	send(WakeSignal{Reason: WakeHeartbeat})

	sig := <-ch
	if sig.Reason != WakeBypassChat {
		t.Errorf("first signal should be chat, got %s", sig.Reason)
	}

	select {
	case extra := <-ch:
		t.Errorf("expected channel to be empty after drain, got %s", extra.Reason)
	default:
	}
}
