package logging

import (
	"fmt"
	"sync"
	"testing"
)

func TestTUILogger_BasicLevels(t *testing.T) {
	var entries []LogEntry
	log := NewTUILogger(func(e LogEntry) {
		entries = append(entries, e)
	})

	log.Debug("debug msg")
	log.Info("info msg")
	log.Warn("warn msg")
	log.Error("error msg")

	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	expected := []struct {
		level Level
		msg   string
	}{
		{DebugLevel, "debug msg"},
		{InfoLevel, "info msg"},
		{WarnLevel, "warn msg"},
		{ErrorLevel, "error msg"},
	}

	for i, e := range expected {
		if entries[i].Level != e.level {
			t.Errorf("entry %d: level = %d, want %d", i, entries[i].Level, e.level)
		}
		if entries[i].Message != e.msg {
			t.Errorf("entry %d: message = %q, want %q", i, entries[i].Message, e.msg)
		}
	}
}

func TestTUILogger_EventExtraction(t *testing.T) {
	var entry LogEntry
	log := NewTUILogger(func(e LogEntry) {
		entry = e
	})

	log.Info(Event("agent.wake"), "cycle started")

	if entry.Event != "agent.wake" {
		t.Errorf("event = %q, want %q", entry.Event, "agent.wake")
	}
	if entry.Message != "cycle started" {
		t.Errorf("message = %q, want %q", entry.Message, "cycle started")
	}
}

func TestTUILogger_EventOnly(t *testing.T) {
	var entry LogEntry
	log := NewTUILogger(func(e LogEntry) {
		entry = e
	})

	log.Info(Event("agent.wake"))

	if entry.Event != "agent.wake" {
		t.Errorf("event = %q, want %q", entry.Event, "agent.wake")
	}
	if entry.Message != "agent.wake" {
		t.Errorf("message = %q, want %q", entry.Message, "agent.wake")
	}
}

func TestTUILogger_FormatArgs(t *testing.T) {
	var entry LogEntry
	log := NewTUILogger(func(e LogEntry) {
		entry = e
	})

	log.Info("cycle %d started with %s", 5, "reason")

	if entry.Message != "cycle 5 started with reason" {
		t.Errorf("message = %q, want %q", entry.Message, "cycle 5 started with reason")
	}
}

func TestTUILogger_WithField(t *testing.T) {
	var entry LogEntry
	log := NewTUILogger(func(e LogEntry) {
		entry = e
	})

	log.WithField("cycle", 3).Info("test")

	if v, ok := entry.Fields["cycle"]; !ok || v != 3 {
		t.Errorf("fields[cycle] = %v, want 3", v)
	}
}

func TestTUILogger_WithFields(t *testing.T) {
	var entry LogEntry
	log := NewTUILogger(func(e LogEntry) {
		entry = e
	})

	log.WithFields(Fields{"a": 1, "b": "two"}).Info("test")

	if v := entry.Fields["a"]; v != 1 {
		t.Errorf("fields[a] = %v, want 1", v)
	}
	if v := entry.Fields["b"]; v != "two" {
		t.Errorf("fields[b] = %v, want %q", v, "two")
	}
}

func TestTUILogger_WithError(t *testing.T) {
	var entry LogEntry
	log := NewTUILogger(func(e LogEntry) {
		entry = e
	})

	log.WithError(fmt.Errorf("something broke")).Error("failed")

	if v := entry.Fields["error"]; v != "something broke" {
		t.Errorf("fields[error] = %v, want %q", v, "something broke")
	}
}

func TestTUILogger_ImmutableFields(t *testing.T) {
	var entries []LogEntry
	log := NewTUILogger(func(e LogEntry) {
		entries = append(entries, e)
	})

	logA := log.WithField("key", "a")
	logB := log.WithField("key", "b")
	logA.Info("from A")
	logB.Info("from B")

	if entries[0].Fields["key"] != "a" {
		t.Errorf("entry 0 key = %v, want %q", entries[0].Fields["key"], "a")
	}
	if entries[1].Fields["key"] != "b" {
		t.Errorf("entry 1 key = %v, want %q", entries[1].Fields["key"], "b")
	}
}

func TestTUILogger_Copy(t *testing.T) {
	var entries []LogEntry
	log := NewTUILogger(func(e LogEntry) {
		entries = append(entries, e)
	})

	original := log.WithField("source", "original")
	copied := original.Copy()
	copied.WithField("extra", "yes").Info("from copy")
	original.Info("from original")

	if v := entries[0].Fields["extra"]; v != "yes" {
		t.Errorf("copy should have extra field")
	}
	if _, ok := entries[1].Fields["extra"]; ok {
		t.Error("original should not have extra field")
	}
}

func TestTUILogger_ConcurrentSafety(t *testing.T) {
	var mu sync.Mutex
	var count int
	log := NewTUILogger(func(_ LogEntry) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			log.WithField("goroutine", true).Info("concurrent")
		})
	}
	wg.Wait()

	if count != 100 {
		t.Errorf("expected 100 entries, got %d", count)
	}
}

func TestTUILogger_Panic(t *testing.T) {
	log := NewTUILogger(func(_ LogEntry) {})

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		}
	}()
	log.Panic("boom")
}
