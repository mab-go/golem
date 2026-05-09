package game

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	pb "github.com/mab-go/golem/internal/grpc/pb"
)

func TestHandleLookAround(t *testing.T) {
	t.Run("radius_default", func(t *testing.T) {
		mc := &mockClient{
			GetSurroundingsFunc: func(_ context.Context, radius int32, _ bool) (*pb.GetSurroundingsResponse, error) {
				if radius != 16 {
					t.Errorf("expected default radius 16, got %d", radius)
				}
				return &pb.GetSurroundingsResponse{
					Biome:      "plains",
					TimeOfDay:  "noon",
					LightLevel: 15,
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "look_around", json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "plains") || !strings.Contains(res, "noon") {
			t.Errorf("expected biome and time, got %q", res)
		}
	})

	t.Run("empty_biome", func(t *testing.T) {
		mc := &mockClient{
			GetSurroundingsFunc: func(_ context.Context, _ int32, _ bool) (*pb.GetSurroundingsResponse, error) {
				return &pb.GetSurroundingsResponse{TimeOfDay: "dusk"}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "look_around", json.RawMessage(`{"radius":32}`))
		if !strings.Contains(res, "unknown biome") {
			t.Errorf("expected unknown biome, got %q", res)
		}
	})

	t.Run("with_entities_and_blocks", func(t *testing.T) {
		mc := &mockClient{
			GetSurroundingsFunc: func(_ context.Context, _ int32, _ bool) (*pb.GetSurroundingsResponse, error) {
				return &pb.GetSurroundingsResponse{
					Biome:          "forest",
					TimeOfDay:      "dawn",
					LightLevel:     12,
					NearbyEntities: []*pb.Entity{{}, {}, {}},
					NotableBlocks:  []*pb.Block{{}, {}},
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "look_around", json.RawMessage(`{}`))
		if !strings.Contains(res, "3 entities") || !strings.Contains(res, "2 notable blocks") {
			t.Errorf("expected counts, got %q", res)
		}
	})

	t.Run("underscore_biome", func(t *testing.T) {
		mc := &mockClient{
			GetSurroundingsFunc: func(_ context.Context, _ int32, _ bool) (*pb.GetSurroundingsResponse, error) {
				return &pb.GetSurroundingsResponse{
					Biome:     "dark_forest",
					TimeOfDay: "night",
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "look_around", json.RawMessage(`{}`))
		if !strings.Contains(res, "dark forest") {
			t.Errorf("expected underscore replaced with space, got %q", res)
		}
	})
}

func TestHandleCheckInventory(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		mc := &mockClient{
			GetInventoryFunc: func(_ context.Context, _ bool) (*pb.GetInventoryResponse, error) {
				return &pb.GetInventoryResponse{EmptySlots: 36}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "check_inventory", json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "empty") || !strings.Contains(res, "36 slots free") {
			t.Errorf("expected empty inventory, got %q", res)
		}
	})

	t.Run("with_items", func(t *testing.T) {
		mc := &mockClient{
			GetInventoryFunc: func(_ context.Context, craft bool) (*pb.GetInventoryResponse, error) {
				if !craft {
					t.Error("expected includeCraftSuggestions=true")
				}
				return &pb.GetInventoryResponse{
					Items: []*pb.InventoryItem{
						{Name: "diamond", Count: 3},
						{Name: "iron_ingot", Count: 12},
					},
					EmptySlots: 34,
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "check_inventory", json.RawMessage(`{}`))
		if !strings.Contains(res, "diamondx3") || !strings.Contains(res, "34 slots free") {
			t.Errorf("expected inventory list, got %q", res)
		}
	})

	t.Run("transport_error", func(t *testing.T) {
		mc := &mockClient{
			GetInventoryFunc: func(_ context.Context, _ bool) (*pb.GetInventoryResponse, error) {
				return nil, errors.New("rpc failed")
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		_, err := d.Execute(context.Background(), "check_inventory", json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("expected transport error")
		}
	})
}

func TestHandleTakeScreenshotLowRes(t *testing.T) {
	mc := &mockClient{
		TakeScreenshotFunc: func(_ context.Context, res pb.ScreenshotResolution, lookAt *pb.Vec3) (*pb.TakeScreenshotResponse, error) {
			if res != pb.ScreenshotResolution_SCREENSHOT_RESOLUTION_LOW {
				t.Errorf("expected low resolution, got %v", res)
			}
			if lookAt != nil {
				t.Error("expected nil lookAt")
			}
			return &pb.TakeScreenshotResponse{
				Width:          320,
				Height:         240,
				CameraPosition: &pb.Vec3{X: 10, Y: 64, Z: 20},
				CameraYaw:      90,
				CameraPitch:    -15,
				ImageData:      []byte{0x89, 0x50, 0x4E, 0x47},
			}, nil
		},
	}
	d := newTestDispatcherWithClient(t, mc)
	result, err := d.ExecuteResult(context.Background(), "take_screenshot", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Text, "320x240") {
		t.Errorf("expected dimensions in metadata, got %q", result.Text)
	}
	if len(result.ImagePNG) != 4 {
		t.Errorf("expected 4-byte image, got %d bytes", len(result.ImagePNG))
	}
}

func TestHandleTakeScreenshotHighRes(t *testing.T) {
	mc := &mockClient{
		TakeScreenshotFunc: func(_ context.Context, res pb.ScreenshotResolution, lookAt *pb.Vec3) (*pb.TakeScreenshotResponse, error) {
			if res != pb.ScreenshotResolution_SCREENSHOT_RESOLUTION_HIGH {
				t.Errorf("expected high resolution")
			}
			if lookAt == nil || lookAt.X != 5 {
				t.Errorf("expected lookAt at (5,...), got %v", lookAt)
			}
			return &pb.TakeScreenshotResponse{
				Width:          1280,
				Height:         720,
				CameraPosition: &pb.Vec3{X: 0, Y: 64, Z: 0},
				ImageData:      []byte{0xFF},
			}, nil
		},
	}
	d := newTestDispatcherWithClient(t, mc)
	result, _ := d.ExecuteResult(context.Background(), "take_screenshot", json.RawMessage(`{"resolution":"high","look_at":{"x":5,"y":10,"z":15}}`))
	if !strings.Contains(result.Text, "1280x720") {
		t.Errorf("expected dimensions, got %q", result.Text)
	}
}

func TestHandleTakeScreenshotError(t *testing.T) {
	mc := &mockClient{
		TakeScreenshotFunc: func(_ context.Context, _ pb.ScreenshotResolution, _ *pb.Vec3) (*pb.TakeScreenshotResponse, error) {
			return nil, errors.New("viewer not available")
		},
	}
	d := newTestDispatcherWithClient(t, mc)
	result, err := d.ExecuteResult(context.Background(), "take_screenshot", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("screenshot errors should not propagate: %v", err)
	}
	if !strings.Contains(result.Text, "screenshot unavailable") {
		t.Errorf("expected unavailable message, got %q", result.Text)
	}
}
