package game

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mab-go/golem/internal/claude"
	sidecar "github.com/mab-go/golem/internal/grpc"
	pb "github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/logging"
	"github.com/mab-go/golem/internal/memory"
	"github.com/mab-go/golem/internal/perception"
	"github.com/mab-go/golem/internal/task"
)

type fakeState struct {
	verbosity perception.VerbosityMode
}

func (f *fakeState) SetVerbosity(v perception.VerbosityMode) { f.verbosity = v }

// newActiveTaskManager returns a *task.Manager with a task named taskName
// already running. The returned cancel must be called in t.Cleanup to stop
// the background drainer goroutine.
func newActiveTaskManager(t *testing.T, taskName string) *task.Manager {
	t.Helper()
	tm := task.NewManager(context.Background(), func(*pb.GameEvent) {}, func() {}, 10*time.Minute, logging.NewDefaultLogger())
	ch := make(chan sidecar.TaskEvent)
	taskCtx, taskCancel := tm.NewTaskContext()
	if err := tm.Start(taskCtx, taskCancel, taskName, ch); err != nil {
		t.Fatalf("task.Start: %v", err)
	}
	t.Cleanup(func() { taskCancel() })
	return tm
}

func newTestDispatcher(t *testing.T) (*Dispatcher, *fakeState, *memory.Manager) {
	t.Helper()
	dir := t.TempDir()
	mem, err := memory.New(dir)
	if err != nil {
		t.Fatalf("memory.New: %v", err)
	}
	st := &fakeState{verbosity: perception.VerbosityStandard}
	// gRPC client is nil -- tests only exercise handlers that don't touch it.
	d := NewDispatcher("testbot", nil, mem, st, nil, nil, logging.NewDefaultLogger())
	return d, st, mem
}

func TestExecuteUnknownTool(t *testing.T) {
	d, _, _ := newTestDispatcher(t)
	_, err := d.Execute(context.Background(), "doesnt_exist", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestSetVerbosity(t *testing.T) {
	d, st, _ := newTestDispatcher(t)
	res, err := d.Execute(context.Background(), claude.ToolSetVerbosity, json.RawMessage(`{"mode":"terse"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(res, "terse") {
		t.Errorf("unexpected result: %q", res)
	}
	if st.verbosity != perception.VerbosityTerse {
		t.Errorf("verbosity not updated: %v", st.verbosity)
	}
}

func TestSetVerbosityInvalid(t *testing.T) {
	d, _, _ := newTestDispatcher(t)
	res, err := d.Execute(context.Background(), claude.ToolSetVerbosity, json.RawMessage(`{"mode":"neon"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(res, "unknown verbosity") {
		t.Errorf("expected parse error surfaced to Claude, got %q", res)
	}
}

func TestWriteJournal(t *testing.T) {
	d, _, mem := newTestDispatcher(t)
	res, err := d.Execute(context.Background(), claude.ToolWriteJournal, json.RawMessage(`{"entry":"found diamonds"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(res, "appended") {
		t.Errorf("result: %q", res)
	}
	data, err := os.ReadFile(filepath.Join(mem.Dir(), memory.FileJournal))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "found diamonds") {
		t.Errorf("journal missing entry: %s", data)
	}
}

func TestWriteJournalEmpty(t *testing.T) {
	d, _, _ := newTestDispatcher(t)
	res, _ := d.Execute(context.Background(), claude.ToolWriteJournal, json.RawMessage(`{"entry":""}`))
	if !strings.Contains(res, "skipped") {
		t.Errorf("expected skip result, got %q", res)
	}
}

func TestUpdateGoals(t *testing.T) {
	d, _, mem := newTestDispatcher(t)
	res, err := d.Execute(context.Background(), claude.ToolUpdateGoals, json.RawMessage(`{"content":"- survive"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(res, "updated") {
		t.Errorf("result: %q", res)
	}
	data, _ := os.ReadFile(filepath.Join(mem.Dir(), memory.FileGoals))
	if !strings.Contains(string(data), "- survive") {
		t.Errorf("goals missing content: %s", data)
	}
}

func TestUpdateWorldKnowledge(t *testing.T) {
	d, _, mem := newTestDispatcher(t)
	res, err := d.Execute(context.Background(), claude.ToolUpdateWorldKnowledge, json.RawMessage(`{"content":"# World\n\n- iron at (-13, 47, 7)"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(res, "updated") {
		t.Errorf("result: %q", res)
	}
	data, _ := os.ReadFile(filepath.Join(mem.Dir(), memory.FileWorldKnowledge))
	if !strings.Contains(string(data), "iron at (-13, 47, 7)") {
		t.Errorf("world-knowledge missing content: %s", data)
	}
}

func TestUpdateWorldKnowledgeEmpty(t *testing.T) {
	d, _, _ := newTestDispatcher(t)
	res, _ := d.Execute(context.Background(), claude.ToolUpdateWorldKnowledge, json.RawMessage(`{"content":""}`))
	if !strings.Contains(res, "skipped") {
		t.Errorf("expected skip result, got %q", res)
	}
}

func TestUpdateInventoryNotes(t *testing.T) {
	d, _, mem := newTestDispatcher(t)
	res, err := d.Execute(context.Background(), claude.ToolUpdateInventoryNotes, json.RawMessage(`{"notes":{"stashes":[{"pos":"-10,65,3","contents":"cobblex15"}]}}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(res, "updated") {
		t.Errorf("result: %q", res)
	}
	data, _ := os.ReadFile(filepath.Join(mem.Dir(), memory.FileInventoryNotes))
	if !strings.Contains(string(data), "cobblex15") {
		t.Errorf("inventory-notes missing content: %s", data)
	}
}

func TestUpdateInventoryNotesEmpty(t *testing.T) {
	d, _, _ := newTestDispatcher(t)
	res, _ := d.Execute(context.Background(), claude.ToolUpdateInventoryNotes, json.RawMessage(`{}`))
	if !strings.Contains(res, "skipped") {
		t.Errorf("expected skip result, got %q", res)
	}
}

func TestUpdateInventoryNotesInvalidJSON(t *testing.T) {
	d, _, _ := newTestDispatcher(t)
	// The outer tool input is valid JSON but `notes` is a JSON scalar that the manager
	// accepts (any valid JSON is fine). To force manager-level rejection we'd need a
	// RawMessage containing invalid JSON bytes, which the outer decode would also reject.
	// Instead, verify that a raw invalid-JSON tool input surfaces as result text, not error.
	res, err := d.Execute(context.Background(), claude.ToolUpdateInventoryNotes, json.RawMessage(`{"notes": not-json}`))
	if err != nil {
		t.Fatalf("expected parse error as text, got transport error: %v", err)
	}
	if !strings.Contains(res, "invalid tool input") {
		t.Errorf("expected parse-error text, got %q", res)
	}
}

func TestUpdateSocial(t *testing.T) {
	d, _, mem := newTestDispatcher(t)
	res, err := d.Execute(context.Background(), claude.ToolUpdateSocial, json.RawMessage(`{"content":"# Social\n\n- loosid: harness dev"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(res, "updated") {
		t.Errorf("result: %q", res)
	}
	data, _ := os.ReadFile(filepath.Join(mem.Dir(), memory.FileSocial))
	if !strings.Contains(string(data), "loosid: harness dev") {
		t.Errorf("social missing content: %s", data)
	}
}

func TestUpdateSocialEmpty(t *testing.T) {
	d, _, _ := newTestDispatcher(t)
	res, _ := d.Execute(context.Background(), claude.ToolUpdateSocial, json.RawMessage(`{"content":""}`))
	if !strings.Contains(res, "skipped") {
		t.Errorf("expected skip result, got %q", res)
	}
}

func TestExecuteEmptyInputDefaults(t *testing.T) {
	d, _, _ := newTestDispatcher(t)
	// Meta handlers ignore input but should not panic on nil.
	if _, err := d.Execute(context.Background(), claude.ToolWriteJournal, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func newTestDispatcherWithTask(t *testing.T, tm *task.Manager) *Dispatcher {
	t.Helper()
	dir := t.TempDir()
	mem, err := memory.New(dir)
	if err != nil {
		t.Fatalf("memory.New: %v", err)
	}
	st := &fakeState{verbosity: perception.VerbosityStandard}
	return NewDispatcher("testbot", nil, mem, st, tm, nil, logging.NewDefaultLogger())
}

func TestConflictRejection(t *testing.T) {
	tm := newActiveTaskManager(t, "gather")
	d := newTestDispatcherWithTask(t, tm)

	for _, tool := range []string{
		claude.ToolMoveTo, claude.ToolNavigateTo, claude.ToolDigBlock,
		claude.ToolPlaceBlock, claude.ToolAttackEntity,
		claude.ToolHarvestBlock, claude.ToolGather,
	} {
		res, err := d.Execute(context.Background(), tool, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("Execute(%s): %v", tool, err)
		}
		if !strings.Contains(res, "Cannot execute") {
			t.Errorf("%s: expected conflict rejection, got %q", tool, res)
		}
	}
}

func TestNonConflictDuringTask(t *testing.T) {
	tm := newActiveTaskManager(t, "gather")
	d := newTestDispatcherWithTask(t, tm)

	// set_verbosity is non-conflicting -- it should work during an active task.
	res, err := d.Execute(context.Background(), claude.ToolSetVerbosity, json.RawMessage(`{"mode":"terse"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if strings.Contains(res, "Cannot execute") {
		t.Errorf("non-conflicting tool was rejected: %q", res)
	}
}

func TestCancelTaskHandler(t *testing.T) {
	tm := newActiveTaskManager(t, "gather")
	d := newTestDispatcherWithTask(t, tm)

	res, err := d.Execute(context.Background(), claude.ToolCancelTask, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res != "Cancelled gather." {
		t.Errorf("result: %q", res)
	}
}

func TestCancelTaskNoTaskManager(t *testing.T) {
	d, _, _ := newTestDispatcher(t)
	res, err := d.Execute(context.Background(), claude.ToolCancelTask, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(res, "No active task") {
		t.Errorf("result: %q", res)
	}
}

func TestTimeout(t *testing.T) {
	d, _, _ := newTestDispatcher(t)

	tests := []struct {
		name  string
		tool  string
		input string
		want  time.Duration
	}{
		{"default for unknown tool", "move_to", `{}`, DefaultToolTimeout},
		{"harvest_block returns default (now streaming)", claude.ToolHarvestBlock, `{"block_type":"iron_ore","count":5}`, DefaultToolTimeout},
		{"smelt count=1 no scaling", claude.ToolSmeltItem, `{"item_name":"raw_iron","count":1}`, DefaultToolTimeout},
		{"smelt count=8", claude.ToolSmeltItem, `{"item_name":"raw_iron","count":8}`, smeltBaseTimeout + 8*smeltPerItem},
		{"smelt count=200 capped", claude.ToolSmeltItem, `{"item_name":"raw_iron","count":200}`, maxToolTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var input json.RawMessage
			if tt.input != "" {
				input = json.RawMessage(tt.input)
			}
			got := d.Timeout(tt.tool, input)
			if got != tt.want {
				t.Errorf("Timeout(%q, %s) = %v, want %v", tt.tool, tt.input, got, tt.want)
			}
		})
	}
}
