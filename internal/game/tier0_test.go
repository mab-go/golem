package game

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	pb "github.com/mab-go/golem/internal/grpc/pb"
)

func TestHandleMoveTo(t *testing.T) {
	t.Run("decode_failure", func(t *testing.T) {
		mc := &mockClient{}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "move_to", json.RawMessage(`not json`))
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if !strings.Contains(res, "invalid tool input") {
			t.Errorf("expected decode error text, got %q", res)
		}
	})

	t.Run("range_default", func(t *testing.T) {
		mc := &mockClient{
			MoveToFunc: func(_ context.Context, _ *pb.Vec3, rng float32, _ bool) (*pb.MoveToResponse, error) {
				if rng != 1 {
					t.Errorf("expected default range 1, got %v", rng)
				}
				return &pb.MoveToResponse{Result: okResult("Moved.")}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		_, err := d.Execute(context.Background(), "move_to", json.RawMessage(`{"target":{"x":1,"y":2,"z":3}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("success_with_position", func(t *testing.T) {
		mc := &mockClient{
			MoveToFunc: func(_ context.Context, _ *pb.Vec3, _ float32, _ bool) (*pb.MoveToResponse, error) {
				return &pb.MoveToResponse{
					Result:        okResult("Arrived."),
					FinalPosition: &pb.Vec3{X: 10, Y: 64, Z: -5},
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "move_to", json.RawMessage(`{"target":{"x":10,"y":64,"z":-5},"range":2}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "10, 64, -5") {
			t.Errorf("expected position in result, got %q", res)
		}
	})

	t.Run("failure_with_distance", func(t *testing.T) {
		mc := &mockClient{
			MoveToFunc: func(_ context.Context, _ *pb.Vec3, _ float32, _ bool) (*pb.MoveToResponse, error) {
				return &pb.MoveToResponse{
					Result:            failResult("path blocked"),
					DistanceRemaining: 12.5,
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "move_to", json.RawMessage(`{"target":{"x":1,"y":2,"z":3}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "12.5 blocks remaining") {
			t.Errorf("expected distance in failure, got %q", res)
		}
	})

	t.Run("transport_error", func(t *testing.T) {
		mc := &mockClient{
			MoveToFunc: func(_ context.Context, _ *pb.Vec3, _ float32, _ bool) (*pb.MoveToResponse, error) {
				return nil, errors.New("connection lost")
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		_, err := d.Execute(context.Background(), "move_to", json.RawMessage(`{"target":{"x":1,"y":2,"z":3}}`))
		if err == nil {
			t.Fatal("expected transport error")
		}
	})
}

func TestHandleLookAt(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mc := &mockClient{
			LookAtFunc: func(_ context.Context, _ *pb.Vec3) (*pb.LookAtResponse, error) {
				return &pb.LookAtResponse{Result: okResult("Looking at target.")}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "look_at", json.RawMessage(`{"target":{"x":1,"y":2,"z":3}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != "Looking at target." {
			t.Errorf("got %q", res)
		}
	})

	t.Run("failure", func(t *testing.T) {
		mc := &mockClient{
			LookAtFunc: func(_ context.Context, _ *pb.Vec3) (*pb.LookAtResponse, error) {
				return &pb.LookAtResponse{Result: failResult("out of range")}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "look_at", json.RawMessage(`{"target":{"x":1,"y":2,"z":3}}`))
		if !strings.Contains(res, "look_at failed") {
			t.Errorf("expected failure text, got %q", res)
		}
	})
}

func TestHandlePlaceBlock(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mc := &mockClient{
			PlaceBlockFunc: func(_ context.Context, _ *pb.Vec3, face, blockName string) (*pb.PlaceBlockResponse, error) {
				if face != "top" || blockName != "cobblestone" {
					t.Errorf("unexpected args: face=%q block=%q", face, blockName)
				}
				return &pb.PlaceBlockResponse{Result: okResult("Placed cobblestone.")}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "place_block", json.RawMessage(`{"position":{"x":0,"y":64,"z":0},"face":"top","block_name":"cobblestone"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "Placed") {
			t.Errorf("got %q", res)
		}
	})

	t.Run("failure", func(t *testing.T) {
		mc := &mockClient{
			PlaceBlockFunc: func(_ context.Context, _ *pb.Vec3, _, _ string) (*pb.PlaceBlockResponse, error) {
				return &pb.PlaceBlockResponse{Result: failResult("no block in inventory")}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "place_block", json.RawMessage(`{"position":{"x":0,"y":64,"z":0}}`))
		if !strings.Contains(res, "place_block failed") {
			t.Errorf("expected failure, got %q", res)
		}
	})
}

func TestHandleDigBlock(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mc := &mockClient{
			DigBlockFunc: func(_ context.Context, pos *pb.Vec3) (*pb.DigBlockResponse, error) {
				if pos.X != 5 || pos.Y != 60 || pos.Z != -3 {
					t.Errorf("unexpected position: %v", pos)
				}
				return &pb.DigBlockResponse{Result: okResult("Dug block.")}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "dig_block", json.RawMessage(`{"position":{"x":5,"y":60,"z":-3}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != "Dug block." {
			t.Errorf("got %q", res)
		}
	})
}

func TestHandleEquipItem(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mc := &mockClient{
			EquipItemFunc: func(_ context.Context, itemName, dest string) (*pb.EquipItemResponse, error) {
				if itemName != "diamond_sword" || dest != "hand" {
					t.Errorf("unexpected args: %q %q", itemName, dest)
				}
				return &pb.EquipItemResponse{Result: okResult("Equipped diamond_sword.")}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "equip_item", json.RawMessage(`{"item_name":"diamond_sword","destination":"hand"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "Equipped") {
			t.Errorf("got %q", res)
		}
	})
}

func TestHandleUseItem(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mc := &mockClient{
			UseItemFunc: func(_ context.Context) (*pb.UseItemResponse, error) {
				return &pb.UseItemResponse{Result: okResult("Used item.")}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "use_item", json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != "Used item." {
			t.Errorf("got %q", res)
		}
	})

	t.Run("failure", func(t *testing.T) {
		mc := &mockClient{
			UseItemFunc: func(_ context.Context) (*pb.UseItemResponse, error) {
				return &pb.UseItemResponse{Result: failResult("nothing in hand")}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "use_item", json.RawMessage(`{}`))
		if !strings.Contains(res, "use_item failed") {
			t.Errorf("expected failure, got %q", res)
		}
	})
}

func TestHandleAttackEntity(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mc := &mockClient{
			AttackEntityFunc: func(_ context.Context, entityID int32) (*pb.AttackEntityResponse, error) {
				if entityID != 42 {
					t.Errorf("expected entity_id 42, got %d", entityID)
				}
				return &pb.AttackEntityResponse{Result: okResult("Attacked entity.")}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "attack_entity", json.RawMessage(`{"entity_id":42}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "Attacked") {
			t.Errorf("got %q", res)
		}
	})
}

func TestHandleJump(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mc := &mockClient{
			JumpFunc: func(_ context.Context) (*pb.JumpResponse, error) {
				return &pb.JumpResponse{Result: okResult("Jumped.")}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "jump", json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != "Jumped." {
			t.Errorf("got %q", res)
		}
	})

	t.Run("transport_error", func(t *testing.T) {
		mc := &mockClient{
			JumpFunc: func(_ context.Context) (*pb.JumpResponse, error) {
				return nil, errors.New("rpc failed")
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		_, err := d.Execute(context.Background(), "jump", json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleSetSneak(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mc := &mockClient{
			SetSneakFunc: func(_ context.Context, enabled bool) (*pb.SetSneakResponse, error) {
				if !enabled {
					t.Error("expected enabled=true")
				}
				return &pb.SetSneakResponse{Result: okResult("Now sneaking.")}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "set_sneak", json.RawMessage(`{"enabled":true}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "sneaking") {
			t.Errorf("got %q", res)
		}
	})
}

func TestHandleSendChat(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mc := &mockClient{
			SendChatFunc: func(_ context.Context, message string) error {
				if message != "hello world" {
					t.Errorf("expected %q, got %q", "hello world", message)
				}
				return nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "send_chat", json.RawMessage(`{"message":"hello world"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "hello world") {
			t.Errorf("expected message in result, got %q", res)
		}
	})

	t.Run("empty_message", func(t *testing.T) {
		mc := &mockClient{}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "send_chat", json.RawMessage(`{"message":""}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "skipped") {
			t.Errorf("expected skip, got %q", res)
		}
	})

	t.Run("transport_error", func(t *testing.T) {
		mc := &mockClient{
			SendChatFunc: func(_ context.Context, _ string) error {
				return errors.New("connection lost")
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		_, err := d.Execute(context.Background(), "send_chat", json.RawMessage(`{"message":"hi"}`))
		if err == nil {
			t.Fatal("expected transport error")
		}
	})
}
