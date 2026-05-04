package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewSeedsFiles(t *testing.T) {
	dir := t.TempDir()
	m, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if m.Dir() != dir {
		t.Errorf("Dir=%s want %s", m.Dir(), dir)
	}
	for name := range defaultSeeds {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected seed file %s: %v", name, err)
		}
	}
}

func TestNewPreservesExistingFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileGoals)
	if err := os.WriteFile(path, []byte("existing goals"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := New(dir); err != nil {
		t.Fatalf("New: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "existing goals" {
		t.Errorf("seeded over existing file: %q", string(data))
	}
}

func TestReadAll(t *testing.T) {
	dir := t.TempDir()
	m, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	st, err := m.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !strings.Contains(st.Journal, "Journal") {
		t.Errorf("journal seed missing: %q", st.Journal)
	}
	if !strings.Contains(st.Goals, "Goals") {
		t.Errorf("goals seed missing: %q", st.Goals)
	}
	if st.InventoryNotes == "" {
		t.Errorf("inventory-notes seed empty")
	}
}

func TestWriteJournalAppends(t *testing.T) {
	dir := t.TempDir()
	m, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := m.WriteJournal("first entry"); err != nil {
		t.Fatalf("WriteJournal 1: %v", err)
	}
	if err := m.WriteJournal("second entry"); err != nil {
		t.Fatalf("WriteJournal 2: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, FileJournal))
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "first entry") || !strings.Contains(s, "second entry") {
		t.Errorf("journal entries missing: %s", s)
	}
	if strings.Count(s, "## ") < 2 {
		t.Errorf("expected at least 2 headers, got %d", strings.Count(s, "## "))
	}
}

func TestUpdateGoalsAtomic(t *testing.T) {
	dir := t.TempDir()
	m, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := m.UpdateGoals("- survive\n- thrive"); err != nil {
		t.Fatalf("UpdateGoals: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, FileGoals))
	if !strings.HasSuffix(string(data), "\n") {
		t.Errorf("missing trailing newline: %q", string(data))
	}
	if !strings.Contains(string(data), "survive") {
		t.Errorf("goals content missing: %q", string(data))
	}
	// No leftover tmp files.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("leftover tmp: %s", e.Name())
		}
	}
}

func TestUpdateInventoryNotesRejectsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	m, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := m.UpdateInventoryNotes(json.RawMessage("not json")); err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
	if err := m.UpdateInventoryNotes(json.RawMessage(`{"food":"bread"}`)); err != nil {
		t.Fatalf("valid JSON should succeed: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, FileInventoryNotes))
	if !strings.Contains(string(data), "bread") {
		t.Errorf("inventory notes missing payload: %q", string(data))
	}
}

func TestUpdateWorldKnowledgeAndSocial(t *testing.T) {
	dir := t.TempDir()
	m, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := m.UpdateWorldKnowledge("notes"); err != nil {
		t.Fatalf("UpdateWorldKnowledge: %v", err)
	}
	if err := m.UpdateSocial("met alice"); err != nil {
		t.Fatalf("UpdateSocial: %v", err)
	}
	wk, _ := os.ReadFile(filepath.Join(dir, FileWorldKnowledge))
	soc, _ := os.ReadFile(filepath.Join(dir, FileSocial))
	if !strings.Contains(string(wk), "notes") {
		t.Errorf("world knowledge missing: %q", wk)
	}
	if !strings.Contains(string(soc), "met alice") {
		t.Errorf("social missing: %q", soc)
	}
}
