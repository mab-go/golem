package agent

import (
	"sync"

	"github.com/mab-go/golem/internal/perception"
)

// PacingState holds the paused and verbosity flags shared between the agent
// loop and its collaborators (ChatInterceptor, Formatter). The mode machine
// (Active/Idle/Interrupt) was replaced by perception-driven wake logic in
// Phase B.
type PacingState struct {
	mu        sync.Mutex
	paused    bool
	verbosity perception.VerbosityMode
}

// NewPacingState constructs a PacingState with the given verbosity.
func NewPacingState(v perception.VerbosityMode) *PacingState {
	return &PacingState{verbosity: v}
}

// Verbosity returns the current verbosity mode.
func (p *PacingState) Verbosity() perception.VerbosityMode {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.verbosity
}

// SetVerbosity updates the verbosity mode.
func (p *PacingState) SetVerbosity(v perception.VerbosityMode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.verbosity = v
}

// Paused reports whether the loop is paused.
func (p *PacingState) Paused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.paused
}

// SetPaused toggles the paused flag.
func (p *PacingState) SetPaused(v bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = v
}
