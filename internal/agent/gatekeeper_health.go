package agent

import (
	"sync"
	"time"

	"github.com/mab-go/golem/internal/logging"
)

type gatekeeperCall struct {
	at     time.Time
	failed bool
}

type gatekeeperHealth struct {
	mu        sync.Mutex
	window    time.Duration
	threshold float64
	calls     []gatekeeperCall
	lastWarn  time.Time
	log       logging.Logger
}

func newGatekeeperHealth(window time.Duration, threshold float64, log logging.Logger) *gatekeeperHealth {
	return &gatekeeperHealth{
		window:    window,
		threshold: threshold,
		log:       log,
	}
}

func (h *gatekeeperHealth) record(failed bool) {
	now := time.Now()

	h.mu.Lock()
	defer h.mu.Unlock()

	h.calls = append(h.calls, gatekeeperCall{at: now, failed: failed})

	cutoff := now.Add(-h.window)
	i := 0
	for i < len(h.calls) && h.calls[i].at.Before(cutoff) {
		i++
	}
	h.calls = h.calls[i:]

	var fails int
	for _, c := range h.calls {
		if c.failed {
			fails++
		}
	}

	total := len(h.calls)
	rate := float64(fails) / float64(total)
	if rate > h.threshold && now.Sub(h.lastWarn) >= h.window {
		h.lastWarn = now
		h.log.WithFields(logging.Fields{
			"fail_rate": rate,
			"fails":     fails,
			"total":     total,
			"window":    h.window.String(),
		}).Warn(eventGatekeeperHighFailRate)
	}
}
