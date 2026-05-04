package agent

import (
	"testing"

	"github.com/mab-go/golem/internal/perception"
)

func TestPacingPauseReadbackOnly(t *testing.T) {
	p := NewPacingState(perception.VerbosityStandard)
	if p.Paused() {
		t.Error("new state should not be paused")
	}
	p.SetPaused(true)
	if !p.Paused() {
		t.Error("SetPaused(true) not reflected")
	}
	p.SetPaused(false)
	if p.Paused() {
		t.Error("SetPaused(false) not reflected")
	}
}

func TestPacingVerbosity(t *testing.T) {
	p := NewPacingState(perception.VerbosityStandard)
	if p.Verbosity() != perception.VerbosityStandard {
		t.Errorf("initial verbosity wrong: %v", p.Verbosity())
	}
	p.SetVerbosity(perception.VerbosityTerse)
	if p.Verbosity() != perception.VerbosityTerse {
		t.Errorf("SetVerbosity not applied")
	}
}
