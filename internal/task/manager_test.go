package task

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	sidecar "github.com/mab-go/golem/internal/grpc"
	pb "github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/logging"
)

type eventCollector struct {
	mu     sync.Mutex
	events []*pb.GameEvent
	wakes  int
}

func (ec *eventCollector) sink(evt *pb.GameEvent) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.events = append(ec.events, evt)
}

func (ec *eventCollector) wake() {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.wakes++
}

func (ec *eventCollector) len() int {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return len(ec.events)
}

func (ec *eventCollector) last() *pb.GameEvent {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	if len(ec.events) == 0 {
		return nil
	}
	return ec.events[len(ec.events)-1]
}

func newTestManager(ec *eventCollector) *Manager {
	return NewManager(context.Background(), ec.sink, ec.wake, 10*time.Second, logging.NewDefaultLogger())
}

func feedEvents(ch chan<- sidecar.TaskEvent, events ...sidecar.TaskEvent) {
	for _, e := range events {
		ch <- e
	}
	close(ch)
}

func TestManager_StartAndComplete(t *testing.T) {
	ec := &eventCollector{}
	tm := newTestManager(ec)

	ch := make(chan sidecar.TaskEvent, 8)
	taskCtx, taskCancel := tm.NewTaskContext()
	err := tm.Start(taskCtx, taskCancel, "gather", ch)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !tm.IsActive() {
		t.Fatal("expected active")
	}
	if tm.ActiveName() != "gather" {
		t.Fatalf("ActiveName: %q", tm.ActiveName())
	}

	st := tm.Status()
	if st == nil || st.Name != "gather" {
		t.Fatalf("Status: %+v", st)
	}

	go feedEvents(ch,
		sidecar.TaskEvent{Progress: &pb.TaskProgress{
			TaskId: "t1", Status: pb.TaskStatus_TASK_IN_PROGRESS,
			Current: 3, Total: 10, Message: "mining stone",
		}},
		sidecar.TaskEvent{Progress: &pb.TaskProgress{
			TaskId: "t1", Status: pb.TaskStatus_TASK_COMPLETED,
			Current: 10, Total: 10, Message: "done",
		}},
	)

	// Wait for drainer to finish.
	time.Sleep(100 * time.Millisecond)

	if tm.IsActive() {
		t.Error("expected idle after completion")
	}
	if ec.len() < 2 {
		t.Fatalf("expected >=2 events, got %d", ec.len())
	}
	last := ec.last()
	if last.Type != pb.EventType_EVENT_TASK_PROGRESS {
		t.Errorf("last event type: %v", last.Type)
	}
	if last.Priority != pb.EventPriority_EVENT_PRIORITY_NORMAL {
		t.Errorf("last event priority: %v", last.Priority)
	}
}

func TestManager_StartWhileActive(t *testing.T) {
	ec := &eventCollector{}
	tm := newTestManager(ec)

	ch1 := make(chan sidecar.TaskEvent, 8)
	ctx1, cancel1 := tm.NewTaskContext()
	if err := tm.Start(ctx1, cancel1, "gather", ch1); err != nil {
		t.Fatalf("Start 1: %v", err)
	}

	ch2 := make(chan sidecar.TaskEvent, 8)
	ctx2, cancel2 := tm.NewTaskContext()
	err := tm.Start(ctx2, cancel2, "farm", ch2)
	if err == nil {
		t.Fatal("expected error for second Start")
	}
	cancel2()

	close(ch1)
	time.Sleep(50 * time.Millisecond)
}

func TestManager_Cancel(t *testing.T) {
	ec := &eventCollector{}
	tm := newTestManager(ec)

	ch := make(chan sidecar.TaskEvent, 8)
	taskCtx, taskCancel := tm.NewTaskContext()
	if err := tm.Start(taskCtx, taskCancel, "gather", ch); err != nil {
		t.Fatalf("Start: %v", err)
	}

	result := tm.Cancel()
	if result != "Cancelled gather." {
		t.Errorf("Cancel result: %q", result)
	}
	if tm.IsActive() {
		t.Error("expected idle after cancel")
	}

	last := ec.last()
	if last == nil {
		t.Fatal("expected cancel event")
	}
	if !strings.Contains(last.Description, "cancelled") {
		t.Errorf("expected 'cancelled' in description, got %q", last.Description)
	}
}

func TestManager_CancelWhenIdle(t *testing.T) {
	ec := &eventCollector{}
	tm := newTestManager(ec)

	result := tm.Cancel()
	if result != "No active task to cancel." {
		t.Errorf("Cancel result: %q", result)
	}
}

func TestManager_TransportError(t *testing.T) {
	ec := &eventCollector{}
	tm := newTestManager(ec)

	ch := make(chan sidecar.TaskEvent, 8)
	taskCtx, taskCancel := tm.NewTaskContext()
	if err := tm.Start(taskCtx, taskCancel, "gather", ch); err != nil {
		t.Fatalf("Start: %v", err)
	}

	ch <- sidecar.TaskEvent{Err: context.DeadlineExceeded}
	close(ch)

	time.Sleep(50 * time.Millisecond)

	if tm.IsActive() {
		t.Error("expected idle after transport error")
	}
	if ec.len() < 1 {
		t.Fatal("expected at least one event")
	}
	last := ec.last()
	if last.Priority != pb.EventPriority_EVENT_PRIORITY_NORMAL {
		t.Errorf("expected NORMAL priority, got %v", last.Priority)
	}
}

func TestManager_StatusNilWhenIdle(t *testing.T) {
	ec := &eventCollector{}
	tm := newTestManager(ec)

	if st := tm.Status(); st != nil {
		t.Errorf("expected nil Status, got %+v", st)
	}
}

func TestManager_Timeout(t *testing.T) {
	ec := &eventCollector{}
	tm := NewManager(context.Background(), ec.sink, ec.wake, 50*time.Millisecond, logging.NewDefaultLogger())

	ch := make(chan sidecar.TaskEvent)
	taskCtx, taskCancel := tm.NewTaskContext()
	if err := tm.Start(taskCtx, taskCancel, "gather", ch); err != nil {
		t.Fatalf("Start: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if tm.IsActive() {
		t.Error("expected idle after timeout")
	}
	if ec.len() < 1 {
		t.Fatal("expected at least one event from timeout")
	}
	last := ec.last()
	if !strings.Contains(last.Description, "timed out") {
		t.Errorf("expected 'timed out' in description, got %q", last.Description)
	}
}

func TestManager_SequentialTasks(t *testing.T) {
	ec := &eventCollector{}
	tm := newTestManager(ec)

	ch1 := make(chan sidecar.TaskEvent, 8)
	ctx1, cancel1 := tm.NewTaskContext()
	if err := tm.Start(ctx1, cancel1, "gather", ch1); err != nil {
		t.Fatalf("Start 1: %v", err)
	}
	ch1 <- sidecar.TaskEvent{Progress: &pb.TaskProgress{
		Status: pb.TaskStatus_TASK_COMPLETED, Message: "done",
	}}
	close(ch1)
	time.Sleep(50 * time.Millisecond)

	if tm.IsActive() {
		t.Fatal("expected idle after first task")
	}

	ch2 := make(chan sidecar.TaskEvent, 8)
	ctx2, cancel2 := tm.NewTaskContext()
	if err := tm.Start(ctx2, cancel2, "farm", ch2); err != nil {
		t.Fatalf("Start 2: %v", err)
	}
	if !tm.IsActive() {
		t.Fatal("expected active for second task")
	}
	ch2 <- sidecar.TaskEvent{Progress: &pb.TaskProgress{
		Status: pb.TaskStatus_TASK_COMPLETED, Message: "done",
	}}
	close(ch2)
	time.Sleep(50 * time.Millisecond)

	if tm.IsActive() {
		t.Fatal("expected idle after second task")
	}
}
