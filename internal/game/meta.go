package game

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mab-go/golem/internal/memory"
	"github.com/mab-go/golem/internal/perception"
)

func truncatePreview(s string) string {
	const maxLen = 80
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (d *Dispatcher) handleCancelTask(_ context.Context, _ json.RawMessage) (string, error) {
	if d.taskMgr == nil {
		return "No active task to cancel.", nil
	}
	return d.taskMgr.Cancel(), nil
}

func (d *Dispatcher) handleSetVerbosity(_ context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Mode string `json:"mode"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	v, err := perception.ParseVerbosity(in.Mode)
	if err != nil {
		return err.Error(), nil
	}
	d.state.SetVerbosity(v)
	return fmt.Sprintf("verbosity set to %s", v), nil
}

func (d *Dispatcher) handleWriteJournal(_ context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Entry string `json:"entry"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.Entry == "" {
		return "write_journal skipped: entry is empty", nil
	}
	if err := d.memory.WriteJournal(in.Entry); err != nil {
		return "", err
	}
	d.pub.PublishMemoryUpdate(memory.FileJournal, truncatePreview(in.Entry))
	return "journal entry appended", nil
}

func (d *Dispatcher) handleUpdateGoals(_ context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Content string `json:"content"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.Content == "" {
		return "update_goals skipped: content is empty", nil
	}
	if err := d.memory.UpdateGoals(in.Content); err != nil {
		return "", err
	}
	d.pub.PublishMemoryUpdate(memory.FileGoals, truncatePreview(in.Content))
	return "goals.md updated", nil
}

func (d *Dispatcher) handleUpdateWorldKnowledge(_ context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Content string `json:"content"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.Content == "" {
		return "update_world_knowledge skipped: content is empty", nil
	}
	if err := d.memory.UpdateWorldKnowledge(in.Content); err != nil {
		return "", err
	}
	d.pub.PublishMemoryUpdate(memory.FileWorldKnowledge, truncatePreview(in.Content))
	return "world-knowledge.md updated", nil
}

func (d *Dispatcher) handleUpdateInventoryNotes(_ context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Notes json.RawMessage `json:"notes"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if len(in.Notes) == 0 {
		return "update_inventory_notes skipped: notes is empty", nil
	}
	if err := d.memory.UpdateInventoryNotes(in.Notes); err != nil {
		return fmt.Sprintf("update_inventory_notes failed: %v", err), nil
	}
	d.pub.PublishMemoryUpdate(memory.FileInventoryNotes, truncatePreview(string(in.Notes)))
	return "inventory-notes.json updated", nil
}

func (d *Dispatcher) handleUpdateSocial(_ context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Content string `json:"content"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.Content == "" {
		return "update_social skipped: content is empty", nil
	}
	if err := d.memory.UpdateSocial(in.Content); err != nil {
		return "", err
	}
	d.pub.PublishMemoryUpdate(memory.FileSocial, truncatePreview(in.Content))
	return "social.md updated", nil
}
