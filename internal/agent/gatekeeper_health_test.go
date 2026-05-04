package agent

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mab-go/golem/internal/logging"
)

func testLogger(buf *bytes.Buffer) logging.Logger {
	return logging.NewLogger(logging.Config{Level: logging.DebugLevel, Output: buf})
}

func countWarnings(buf *bytes.Buffer) int {
	return strings.Count(buf.String(), "level=warning")
}

func TestGatekeeperHealthBelowThreshold(t *testing.T) {
	var buf bytes.Buffer
	h := newGatekeeperHealth(10*time.Minute, 0.05, testLogger(&buf))

	for i := 0; i < 96; i++ {
		h.record(false)
	}
	for i := 0; i < 4; i++ {
		h.record(true)
	}

	if countWarnings(&buf) != 0 {
		t.Error("should not warn at 4% fail rate (threshold 5%)")
	}
}

func TestGatekeeperHealthAboveThreshold(t *testing.T) {
	var buf bytes.Buffer
	h := newGatekeeperHealth(5*time.Minute, 0.10, testLogger(&buf))

	for i := 0; i < 8; i++ {
		h.record(false)
	}
	for i := 0; i < 2; i++ {
		h.record(true)
	}

	if countWarnings(&buf) != 1 {
		t.Error("should warn at 20% fail rate (threshold 10%)")
	}
}

func TestGatekeeperHealthThrottlesWarning(t *testing.T) {
	var buf bytes.Buffer
	h := newGatekeeperHealth(5*time.Minute, 0.05, testLogger(&buf))

	for i := 0; i < 8; i++ {
		h.record(false)
	}
	h.record(true)
	h.record(true)

	if n := countWarnings(&buf); n != 1 {
		t.Fatalf("expected 1 warning, got %d", n)
	}

	for i := 0; i < 5; i++ {
		h.record(true)
	}

	if n := countWarnings(&buf); n != 1 {
		t.Errorf("expected warning to be throttled (still 1), got %d", n)
	}
}
