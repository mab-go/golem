package logging

import (
	"bytes"
	"strings"
	"testing"
)

func newTestLogger(buf *bytes.Buffer) Logger {
	return NewLogger(Config{
		Level:  DebugLevel,
		Output: buf,
	})
}

func TestEventSetsField(t *testing.T) {
	var buf bytes.Buffer
	log := newTestLogger(&buf)

	log.Info(Event("agent.wake"))

	out := buf.String()
	if !strings.Contains(out, "event=agent.wake") {
		t.Errorf("expected event field in output, got: %s", out)
	}
}

func TestEventAsMessage(t *testing.T) {
	var buf bytes.Buffer
	log := newTestLogger(&buf)

	log.Info(Event("agent.wake"))

	out := buf.String()
	if !strings.Contains(out, "agent.wake") {
		t.Errorf("expected event name as message, got: %s", out)
	}
}

func TestPlainStringNoEventField(t *testing.T) {
	var buf bytes.Buffer
	log := newTestLogger(&buf)

	log.Info("plain message")

	out := buf.String()
	if !strings.Contains(out, "plain message") {
		t.Errorf("expected plain message, got: %s", out)
	}
	if strings.Contains(out, "event=") {
		t.Errorf("unexpected event field in plain log, got: %s", out)
	}
}

func TestEventWithFields(t *testing.T) {
	var buf bytes.Buffer
	log := newTestLogger(&buf)

	log.WithField("cycle", 5).Info(Event("agent.wake"))

	out := buf.String()
	if !strings.Contains(out, "event=agent.wake") {
		t.Errorf("expected event field, got: %s", out)
	}
	if !strings.Contains(out, "cycle=5") {
		t.Errorf("expected cycle field, got: %s", out)
	}
}

func TestEventWithMultipleFields(t *testing.T) {
	var buf bytes.Buffer
	log := newTestLogger(&buf)

	log.WithFields(Fields{"cycle": 1, "round": 2}).Debug(Event("agent.turn_complete"))

	out := buf.String()
	if !strings.Contains(out, "event=agent.turn_complete") {
		t.Errorf("expected event field, got: %s", out)
	}
}

func TestEventWithError(t *testing.T) {
	var buf bytes.Buffer
	log := newTestLogger(&buf)

	log.WithError(errForTest).Error(Event("agent.cycle_failed"))

	out := buf.String()
	if !strings.Contains(out, "event=agent.cycle_failed") {
		t.Errorf("expected event field, got: %s", out)
	}
	if !strings.Contains(out, "agent.cycle_failed") {
		t.Errorf("expected event as message, got: %s", out)
	}
}

func TestEmptyArgsNoEvent(t *testing.T) {
	var buf bytes.Buffer
	log := newTestLogger(&buf)

	log.Info()

	out := buf.String()
	if strings.Contains(out, "event=") {
		t.Errorf("unexpected event field when no event set, got: %s", out)
	}
}

func TestEventAllLevels(t *testing.T) {
	levels := []struct {
		name string
		call func(Logger)
	}{
		{"debug", func(l Logger) { l.Debug(Event("test.debug")) }},
		{"info", func(l Logger) { l.Info(Event("test.info")) }},
		{"warn", func(l Logger) { l.Warn(Event("test.warn")) }},
		{"error", func(l Logger) { l.Error(Event("test.error")) }},
	}

	for _, tt := range levels {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := newTestLogger(&buf)

			tt.call(log)

			out := buf.String()
			if !strings.Contains(out, "event=test."+tt.name) {
				t.Errorf("expected event field for %s level, got: %s", tt.name, out)
			}
		})
	}
}

type testError struct{}

func (testError) Error() string { return "test error" }

var errForTest error = testError{}
