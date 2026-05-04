package claude

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/mab-go/golem/internal/logging"
)

// ModelTotals is the cumulative token accounting for a single model ID.
type ModelTotals struct {
	Calls              int64
	InputTokens        int64
	OutputTokens       int64
	CacheCreationInput int64
	CacheReadInput     int64
}

// Metrics tracks per-model token usage across the lifetime of a process. It is
// safe for concurrent Record calls from multiple goroutines.
//
// A nil *Metrics is a valid no-op: Record returns without touching state and
// Summary returns an empty map. This keeps test wiring simple -- production
// code passes a real Metrics via NewClient, tests can pass nil.
type Metrics struct {
	mu     sync.Mutex
	totals map[string]*ModelTotals
}

// NewMetrics returns an empty Metrics ready to record.
func NewMetrics() *Metrics {
	return &Metrics{totals: make(map[string]*ModelTotals)}
}

// Record adds one call's worth of token accounting to the model's totals.
// An empty model string is ignored so partial/aborted streams don't pollute
// the summary.
func (m *Metrics) Record(model string, u Usage) {
	if m == nil || model == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	t := m.totals[model]
	if t == nil {
		t = &ModelTotals{}
		m.totals[model] = t
	}
	t.Calls++
	t.InputTokens += u.InputTokens
	t.OutputTokens += u.OutputTokens
	t.CacheCreationInput += u.CacheCreationInput
	t.CacheReadInput += u.CacheReadInput
}

// Summary returns a snapshot of the current totals keyed by model. The map
// returned is safe to read without holding the Metrics lock.
func (m *Metrics) Summary() map[string]ModelTotals {
	if m == nil {
		return map[string]ModelTotals{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make(map[string]ModelTotals, len(m.totals))
	for k, v := range m.totals {
		out[k] = *v
	}
	return out
}

// StartSummaryLoop emits one "tokens_summary" log line per model at the given
// interval until ctx is cancelled. Models with no activity emit nothing. The
// first summary fires after `interval`, not immediately; an interval <= 0
// disables the loop.
func (m *Metrics) StartSummaryLoop(ctx context.Context, interval time.Duration, log logging.Logger) {
	if m == nil || interval <= 0 || log == nil {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.emitSummary(log)
		}
	}
}

func (m *Metrics) emitSummary(log logging.Logger) {
	totals := m.Summary()
	if len(totals) == 0 {
		return
	}
	models := make([]string, 0, len(totals))
	for model := range totals {
		models = append(models, model)
	}
	sort.Strings(models)

	for _, model := range models {
		t := totals[model]
		log.WithFields(logging.Fields{
			"model":        model,
			"calls":        t.Calls,
			"input":        t.InputTokens,
			"output":       t.OutputTokens,
			"cache_create": t.CacheCreationInput,
			"cache_read":   t.CacheReadInput,
		}).Info(eventTokenSummary)
	}
}
