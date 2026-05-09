package game

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	sidecar "github.com/mab-go/golem/internal/grpc"
	pb "github.com/mab-go/golem/internal/grpc/pb"
)

func closedTaskChan() <-chan sidecar.TaskEvent {
	ch := make(chan sidecar.TaskEvent)
	close(ch)
	return ch
}

func TestHandleHarvestBlock(t *testing.T) {
	t.Run("no_block_type", func(t *testing.T) {
		mc := &mockClient{}
		d := newTestDispatcherWithClientAndTasks(t, mc)
		res, err := d.Execute(context.Background(), "harvest_block", json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "requires block_type") {
			t.Errorf("expected validation error, got %q", res)
		}
	})

	t.Run("defaults_and_task_started", func(t *testing.T) {
		mc := &mockClient{
			HarvestBlockFunc: func(_ context.Context, req *pb.HarvestBlockRequest) (<-chan sidecar.TaskEvent, error) {
				if req.Count != 1 {
					t.Errorf("expected default count 1, got %d", req.Count)
				}
				if req.MaxDistance != 16 {
					t.Errorf("expected default max_distance 16, got %d", req.MaxDistance)
				}
				return closedTaskChan(), nil
			},
		}
		d := newTestDispatcherWithClientAndTasks(t, mc)
		res, err := d.Execute(context.Background(), "harvest_block", json.RawMessage(`{"block_type":"iron_ore"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "Task started") || !strings.Contains(res, "iron_ore") {
			t.Errorf("expected task started message, got %q", res)
		}
	})
}

func TestHandleGather(t *testing.T) {
	t.Run("no_resource", func(t *testing.T) {
		mc := &mockClient{}
		d := newTestDispatcherWithClientAndTasks(t, mc)
		res, _ := d.Execute(context.Background(), "gather", json.RawMessage(`{}`))
		if !strings.Contains(res, "requires") {
			t.Errorf("expected validation error, got %q", res)
		}
	})

	t.Run("defaults_and_task_started", func(t *testing.T) {
		mc := &mockClient{
			GatherFunc: func(_ context.Context, req *pb.GatherRequest) (<-chan sidecar.TaskEvent, error) {
				if req.Quantity != 1 {
					t.Errorf("expected default quantity 1, got %d", req.Quantity)
				}
				return closedTaskChan(), nil
			},
		}
		d := newTestDispatcherWithClientAndTasks(t, mc)
		res, err := d.Execute(context.Background(), "gather", json.RawMessage(`{"resource":"wood"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "Task started") || !strings.Contains(res, "wood") {
			t.Errorf("expected task started message, got %q", res)
		}
	})
}

func TestHandleBuildStructure(t *testing.T) {
	t.Run("no_material", func(t *testing.T) {
		mc := &mockClient{}
		d := newTestDispatcherWithClientAndTasks(t, mc)
		res, _ := d.Execute(context.Background(), "build_structure", json.RawMessage(`{}`))
		if !strings.Contains(res, "requires") {
			t.Errorf("expected validation error, got %q", res)
		}
	})

	t.Run("defaults_and_task_started", func(t *testing.T) {
		mc := &mockClient{
			BuildStructureFunc: func(_ context.Context, req *pb.BuildStructureRequest) (<-chan sidecar.TaskEvent, error) {
				if req.Type != "shelter" {
					t.Errorf("expected default type shelter, got %q", req.Type)
				}
				return closedTaskChan(), nil
			},
		}
		d := newTestDispatcherWithClientAndTasks(t, mc)
		res, err := d.Execute(context.Background(), "build_structure", json.RawMessage(`{"material":"cobblestone","location":{"x":0,"y":64,"z":0},"dimensions":{"x":5,"y":4,"z":5}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "Task started") || !strings.Contains(res, "cobblestone") {
			t.Errorf("expected task started message, got %q", res)
		}
	})
}

func TestHandleProcessAll(t *testing.T) {
	t.Run("no_item", func(t *testing.T) {
		mc := &mockClient{}
		d := newTestDispatcherWithClientAndTasks(t, mc)
		res, _ := d.Execute(context.Background(), "process_all", json.RawMessage(`{}`))
		if !strings.Contains(res, "requires") {
			t.Errorf("expected validation error, got %q", res)
		}
	})

	t.Run("task_started", func(t *testing.T) {
		mc := &mockClient{
			ProcessAllFunc: func(_ context.Context, req *pb.ProcessAllRequest) (<-chan sidecar.TaskEvent, error) {
				if req.Item != "raw_iron" {
					t.Errorf("expected raw_iron, got %q", req.Item)
				}
				return closedTaskChan(), nil
			},
		}
		d := newTestDispatcherWithClientAndTasks(t, mc)
		res, err := d.Execute(context.Background(), "process_all", json.RawMessage(`{"item":"raw_iron"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "Task started") {
			t.Errorf("expected task started, got %q", res)
		}
	})
}

func TestHandleOrganizeInventory(t *testing.T) {
	t.Run("task_started", func(t *testing.T) {
		mc := &mockClient{
			OrganizeInventoryFunc: func(_ context.Context, req *pb.OrganizeInventoryRequest) (<-chan sidecar.TaskEvent, error) {
				if !req.StashJunk {
					t.Error("expected stash_junk=true")
				}
				return closedTaskChan(), nil
			},
		}
		d := newTestDispatcherWithClientAndTasks(t, mc)
		res, err := d.Execute(context.Background(), "organize_inventory", json.RawMessage(`{"stash_junk":true}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "Task started") {
			t.Errorf("expected task started, got %q", res)
		}
	})
}

func TestHandleClearArea(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		mc := &mockClient{
			ClearAreaFunc: func(_ context.Context, req *pb.ClearAreaRequest) (<-chan sidecar.TaskEvent, error) {
				if req.Radius != 4 {
					t.Errorf("expected default radius 4, got %d", req.Radius)
				}
				return closedTaskChan(), nil
			},
		}
		d := newTestDispatcherWithClientAndTasks(t, mc)
		res, err := d.Execute(context.Background(), "clear_area", json.RawMessage(`{"center":{"x":0,"y":64,"z":0}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "Task started") || !strings.Contains(res, "radius 4") {
			t.Errorf("expected task started with radius, got %q", res)
		}
	})
}

func TestHandleFarm(t *testing.T) {
	t.Run("no_crop", func(t *testing.T) {
		mc := &mockClient{}
		d := newTestDispatcherWithClientAndTasks(t, mc)
		res, _ := d.Execute(context.Background(), "farm", json.RawMessage(`{}`))
		if !strings.Contains(res, "requires") {
			t.Errorf("expected validation error, got %q", res)
		}
	})

	t.Run("task_started", func(t *testing.T) {
		mc := &mockClient{
			FarmFunc: func(_ context.Context, req *pb.FarmRequest) (<-chan sidecar.TaskEvent, error) {
				if req.Crop != "wheat" {
					t.Errorf("expected wheat, got %q", req.Crop)
				}
				return closedTaskChan(), nil
			},
		}
		d := newTestDispatcherWithClientAndTasks(t, mc)
		res, err := d.Execute(context.Background(), "farm", json.RawMessage(`{"crop":"wheat","location":{"x":10,"y":64,"z":10}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "Task started") || !strings.Contains(res, "wheat") {
			t.Errorf("expected task started, got %q", res)
		}
	})
}
