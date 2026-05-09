package claude

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mab-go/golem/internal/logging"
)

func TestMetricsRecordAggregates(t *testing.T) {
	m := NewMetrics()
	m.Record("model-a", Usage{InputTokens: 100, OutputTokens: 50, CacheCreationInput: 10, CacheReadInput: 5})
	m.Record("model-a", Usage{InputTokens: 200, OutputTokens: 75, CacheReadInput: 20})
	m.Record("model-b", Usage{InputTokens: 30, OutputTokens: 10})

	got := m.Summary()

	a, ok := got["model-a"]
	if !ok {
		t.Fatalf("missing model-a in summary: %v", got)
	}
	wantA := ModelTotals{Calls: 2, InputTokens: 300, OutputTokens: 125, CacheCreationInput: 10, CacheReadInput: 25}
	if a != wantA {
		t.Errorf("model-a totals = %+v, want %+v", a, wantA)
	}

	b, ok := got["model-b"]
	if !ok {
		t.Fatalf("missing model-b in summary: %v", got)
	}
	wantB := ModelTotals{Calls: 1, InputTokens: 30, OutputTokens: 10}
	if b != wantB {
		t.Errorf("model-b totals = %+v, want %+v", b, wantB)
	}
}

func TestMetricsRecordIgnoresEmptyModel(t *testing.T) {
	m := NewMetrics()
	m.Record("", Usage{InputTokens: 999})
	if len(m.Summary()) != 0 {
		t.Errorf("expected empty summary, got %v", m.Summary())
	}
}

func TestMetricsNilReceiverIsNoOp(t *testing.T) {
	var m *Metrics
	m.Record("anything", Usage{InputTokens: 1})
	if got := m.Summary(); len(got) != 0 {
		t.Errorf("nil Metrics Summary() = %v; want empty", got)
	}
}

func TestMetricsSummaryIsIsolated(t *testing.T) {
	m := NewMetrics()
	m.Record("model-a", Usage{InputTokens: 100})

	snap := m.Summary()
	snap["model-a"] = ModelTotals{InputTokens: 999}

	again := m.Summary()
	if again["model-a"].InputTokens != 100 {
		t.Errorf("Summary() leaked internal state: %+v", again["model-a"])
	}
}

func TestMetricsConcurrentRecordIsRaceFree(t *testing.T) {
	m := NewMetrics()
	const workers = 16
	const perWorker = 200

	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for range perWorker {
				m.Record("model-a", Usage{InputTokens: 1, OutputTokens: 1})
			}
		}()
	}
	wg.Wait()

	a := m.Summary()["model-a"]
	wantCalls := int64(workers * perWorker)
	if a.Calls != wantCalls {
		t.Errorf("Calls = %d, want %d", a.Calls, wantCalls)
	}
	if a.InputTokens != wantCalls || a.OutputTokens != wantCalls {
		t.Errorf("tokens not summed correctly: %+v (want %d each)", a, wantCalls)
	}
}

func TestStartSummaryLoopExitsOnContextCancel(t *testing.T) {
	m := NewMetrics()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		m.StartSummaryLoop(ctx, 10*time.Millisecond, logging.NewDefaultLogger())
		close(done)
	}()

	time.Sleep(5 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("StartSummaryLoop did not exit after ctx cancel")
	}
}

func TestEmitSummary(_ *testing.T) {
	m := NewMetrics()
	m.Record("model-a", Usage{InputTokens: 100, OutputTokens: 50})
	m.Record("model-b", Usage{InputTokens: 200, OutputTokens: 75})
	m.emitSummary(logging.NewDefaultLogger())
}

func TestEmitSummaryEmpty(_ *testing.T) {
	m := NewMetrics()
	m.emitSummary(logging.NewDefaultLogger())
}

func TestStartSummaryLoopNoOpOnZeroInterval(t *testing.T) {
	m := NewMetrics()

	done := make(chan struct{})
	go func() {
		m.StartSummaryLoop(t.Context(), 0, logging.NewDefaultLogger())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("StartSummaryLoop with interval <= 0 should return immediately")
	}
}
