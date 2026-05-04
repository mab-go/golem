package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Filenames of the persistent memory files. Names are constants so callers
// referencing specific files stay centralized.
const (
	FileJournal        = "journal.md"
	FileGoals          = "goals.md"
	FileWorldKnowledge = "world-knowledge.md"
	FileInventoryNotes = "inventory-notes.json"
	FileSocial         = "social.md"
)

var defaultSeeds = map[string]string{
	FileJournal:        "# Journal\n\nThis file holds timestamped entries written by the agent across sessions.\n",
	FileGoals:          "# Goals\n\n(No goals set yet.)\n",
	FileWorldKnowledge: "# World Knowledge\n\n(No observations yet.)\n",
	FileInventoryNotes: "{}\n",
	FileSocial:         "# Social Notes\n\n(No player interactions recorded yet.)\n",
}

// State bundles the contents of all memory files -- what a single ReadAll returns.
type State struct {
	Journal        string
	Goals          string
	WorldKnowledge string
	InventoryNotes string
	Social         string
}

// Manager serializes access to the on-disk memory directory.
type Manager struct {
	dir string
	mu  sync.Mutex
}

// New constructs a Manager for the given directory. The directory is created
// if missing, and any seed files listed in defaultSeeds are written if absent.
func New(dir string) (*Manager, error) {
	if dir == "" {
		return nil, fmt.Errorf("memory dir is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create memory dir %s: %w", dir, err)
	}
	m := &Manager{dir: dir}
	if err := m.seedMissingFiles(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) seedMissingFiles() error {
	for name, seed := range defaultSeeds {
		path := filepath.Join(m.dir, name)
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
			return fmt.Errorf("seed %s: %w", path, err)
		}
	}
	return nil
}

// Dir returns the memory directory.
func (m *Manager) Dir() string { return m.dir }

func (m *Manager) readFile(name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(m.dir, name))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read %s: %w", name, err)
	}
	return string(data), nil
}

// ReadAll loads every memory file into a State struct. Missing files are read
// as empty strings so the caller can always render something into the context.
func (m *Manager) ReadAll() (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var s State
	var err error
	if s.Journal, err = m.readFile(FileJournal); err != nil {
		return s, err
	}
	if s.Goals, err = m.readFile(FileGoals); err != nil {
		return s, err
	}
	if s.WorldKnowledge, err = m.readFile(FileWorldKnowledge); err != nil {
		return s, err
	}
	if s.InventoryNotes, err = m.readFile(FileInventoryNotes); err != nil {
		return s, err
	}
	if s.Social, err = m.readFile(FileSocial); err != nil {
		return s, err
	}
	return s, nil
}

// WriteJournal appends a timestamped entry to the journal file. The timestamp
// is an RFC3339 UTC header so entries remain grep-friendly across sessions.
func (m *Manager) WriteJournal(entry string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := filepath.Join(m.dir, FileJournal)
	ts := time.Now().UTC().Format(time.RFC3339)
	body := strings.TrimSpace(entry)
	record := fmt.Sprintf("\n## %s\n\n%s\n", ts, body)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open journal: %w", err)
	}
	if _, err := f.WriteString(record); err != nil {
		_ = f.Close()
		return fmt.Errorf("append journal: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close journal: %w", err)
	}
	return nil
}

// UpdateGoals atomically replaces goals.md with the provided content.
func (m *Manager) UpdateGoals(content string) error {
	return m.atomicWrite(FileGoals, []byte(ensureTrailingNewline(content)))
}

// UpdateWorldKnowledge atomically replaces world-knowledge.md.
func (m *Manager) UpdateWorldKnowledge(content string) error {
	return m.atomicWrite(FileWorldKnowledge, []byte(ensureTrailingNewline(content)))
}

// UpdateInventoryNotes atomically replaces inventory-notes.json. Payload must
// be valid JSON; this function re-marshals through encoding/json to reject
// malformed input early.
func (m *Manager) UpdateInventoryNotes(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("inventory notes payload is empty")
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return fmt.Errorf("validate inventory notes JSON: %w", err)
	}
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode inventory notes: %w", err)
	}
	return m.atomicWrite(FileInventoryNotes, append(pretty, '\n'))
}

// UpdateSocial atomically replaces social.md.
func (m *Manager) UpdateSocial(content string) error {
	return m.atomicWrite(FileSocial, []byte(ensureTrailingNewline(content)))
}

func (m *Manager) atomicWrite(name string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := filepath.Join(m.dir, name)
	tmp, err := os.CreateTemp(m.dir, name+".tmp-*")
	if err != nil {
		return fmt.Errorf("create tmp for %s: %w", name, err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write tmp %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close tmp %s: %w", name, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename %s: %w", name, err)
	}
	return nil
}

func ensureTrailingNewline(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}
