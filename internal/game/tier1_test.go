package game

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	pb "github.com/mab-go/golem/internal/grpc/pb"
)

func TestHandleNavigateToValidation(t *testing.T) {
	mc := &mockClient{}
	d := newTestDispatcherWithClient(t, mc)
	res, err := d.Execute(context.Background(), "navigate_to", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res, "requires one of") {
		t.Errorf("expected validation error, got %q", res)
	}
}

func TestHandleNavigateToTargets(t *testing.T) {
	t.Run("by_position", func(t *testing.T) {
		mc := &mockClient{
			NavigateToFunc: func(_ context.Context, req *pb.NavigateToRequest) (*pb.NavigateToResponse, error) {
				if req.GetPosition() == nil {
					t.Error("expected position target")
				}
				return &pb.NavigateToResponse{
					Result:           okResult("Arrived."),
					DistanceTraveled: 25.3,
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "navigate_to", json.RawMessage(`{"position":{"x":10,"y":64,"z":20}}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "25.3 blocks") {
			t.Errorf("expected distance, got %q", res)
		}
	})

	t.Run("by_entity_name", func(t *testing.T) {
		mc := &mockClient{
			NavigateToFunc: func(_ context.Context, req *pb.NavigateToRequest) (*pb.NavigateToResponse, error) {
				if req.GetEntityName() != "cow" {
					t.Errorf("expected entity_name=cow, got %q", req.GetEntityName())
				}
				return &pb.NavigateToResponse{Result: okResult("Arrived."), DistanceTraveled: 5}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		_, err := d.Execute(context.Background(), "navigate_to", json.RawMessage(`{"entity_name":"cow"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("by_block_type", func(t *testing.T) {
		mc := &mockClient{
			NavigateToFunc: func(_ context.Context, req *pb.NavigateToRequest) (*pb.NavigateToResponse, error) {
				if req.GetBlockType() != "crafting_table" {
					t.Errorf("expected block_type=crafting_table, got %q", req.GetBlockType())
				}
				return &pb.NavigateToResponse{Result: okResult("Arrived."), DistanceTraveled: 3}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		_, err := d.Execute(context.Background(), "navigate_to", json.RawMessage(`{"block_type":"crafting_table"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestHandleNavigateToErrors(t *testing.T) {
	t.Run("failure", func(t *testing.T) {
		mc := &mockClient{
			NavigateToFunc: func(_ context.Context, _ *pb.NavigateToRequest) (*pb.NavigateToResponse, error) {
				return &pb.NavigateToResponse{Result: failResult("path blocked")}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "navigate_to", json.RawMessage(`{"entity_name":"zombie"}`))
		if !strings.Contains(res, "navigate_to failed") {
			t.Errorf("expected failure, got %q", res)
		}
	})

	t.Run("transport_error", func(t *testing.T) {
		mc := &mockClient{
			NavigateToFunc: func(_ context.Context, _ *pb.NavigateToRequest) (*pb.NavigateToResponse, error) {
				return nil, errors.New("rpc failed")
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		_, err := d.Execute(context.Background(), "navigate_to", json.RawMessage(`{"position":{"x":0,"y":0,"z":0}}`))
		if err == nil {
			t.Fatal("expected transport error")
		}
	})
}

func TestHandleInteractWithEntityValidation(t *testing.T) {
	t.Run("unknown_action", func(t *testing.T) {
		mc := &mockClient{}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "interact_with_entity", json.RawMessage(`{"entity_name":"cow","action":"explode"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "unknown interaction action") {
			t.Errorf("expected validation error, got %q", res)
		}
	})

	t.Run("no_entity_identifier", func(t *testing.T) {
		mc := &mockClient{}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "interact_with_entity", json.RawMessage(`{"action":"harvest"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "requires entity_name or entity_id") {
			t.Errorf("expected validation error, got %q", res)
		}
	})
}

func TestHandleInteractWithEntityActions(t *testing.T) {
	for _, action := range []string{"harvest", "attack", "feed", "trade", "mount", "shear"} {
		t.Run(action, func(t *testing.T) {
			mc := &mockClient{
				InteractWithEntityFunc: func(_ context.Context, req *pb.InteractWithEntityRequest) (*pb.InteractWithEntityResponse, error) {
					if req.EntityName != "cow" {
						t.Errorf("expected cow, got %q", req.EntityName)
					}
					return &pb.InteractWithEntityResponse{Result: okResult("Done.")}, nil
				},
			}
			d := newTestDispatcherWithClient(t, mc)
			_, err := d.Execute(context.Background(), "interact_with_entity", json.RawMessage(`{"entity_name":"cow","action":"`+action+`"}`))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestHandleInteractWithEntityResponse(t *testing.T) {
	t.Run("with_drops", func(t *testing.T) {
		mc := &mockClient{
			InteractWithEntityFunc: func(_ context.Context, _ *pb.InteractWithEntityRequest) (*pb.InteractWithEntityResponse, error) {
				return &pb.InteractWithEntityResponse{
					Result: okResult("Harvested cow."),
					Drops:  []*pb.InventoryItem{{Name: "leather", Count: 2}, {Name: "raw_beef", Count: 3}},
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "interact_with_entity", json.RawMessage(`{"entity_name":"cow","action":"harvest"}`))
		if !strings.Contains(res, "leatherx2") || !strings.Contains(res, "raw_beefx3") {
			t.Errorf("expected drops in result, got %q", res)
		}
	})

	t.Run("with_description", func(t *testing.T) {
		mc := &mockClient{
			InteractWithEntityFunc: func(_ context.Context, _ *pb.InteractWithEntityRequest) (*pb.InteractWithEntityResponse, error) {
				return &pb.InteractWithEntityResponse{
					Result:      okResult("Done."),
					Description: "Traded 3 emeralds for a map.",
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "interact_with_entity", json.RawMessage(`{"entity_name":"villager","action":"trade"}`))
		if !strings.Contains(res, "Traded 3 emeralds") {
			t.Errorf("expected description, got %q", res)
		}
	})
}

func TestHandleOpenContainer(t *testing.T) {
	t.Run("no_target", func(t *testing.T) {
		mc := &mockClient{}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "open_container", json.RawMessage(`{}`))
		if !strings.Contains(res, "requires position or block_type") {
			t.Errorf("expected validation error, got %q", res)
		}
	})

	t.Run("by_position_with_contents", func(t *testing.T) {
		mc := &mockClient{
			OpenContainerFunc: func(_ context.Context, req *pb.OpenContainerRequest) (*pb.OpenContainerResponse, error) {
				if req.GetPosition() == nil {
					t.Error("expected position target")
				}
				return &pb.OpenContainerResponse{
					Result:        okResult("Opened."),
					ContainerType: "chest",
					Contents:      []*pb.InventoryItem{{Name: "diamond", Count: 5}},
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "open_container", json.RawMessage(`{"position":{"x":0,"y":64,"z":0}}`))
		if !strings.Contains(res, "chest") || !strings.Contains(res, "diamondx5") {
			t.Errorf("expected contents, got %q", res)
		}
	})

	t.Run("empty_container", func(t *testing.T) {
		mc := &mockClient{
			OpenContainerFunc: func(_ context.Context, _ *pb.OpenContainerRequest) (*pb.OpenContainerResponse, error) {
				return &pb.OpenContainerResponse{
					Result:        okResult("Opened."),
					ContainerType: "barrel",
					Contents:      nil,
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "open_container", json.RawMessage(`{"block_type":"barrel"}`))
		if !strings.Contains(res, "empty") {
			t.Errorf("expected empty notice, got %q", res)
		}
	})
}

func TestHandleWithdrawFromContainer(t *testing.T) {
	t.Run("no_item_name", func(t *testing.T) {
		mc := &mockClient{}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "withdraw_from_container", json.RawMessage(`{"position":{"x":0,"y":0,"z":0}}`))
		if !strings.Contains(res, "requires item_name") {
			t.Errorf("expected validation error, got %q", res)
		}
	})

	t.Run("no_target", func(t *testing.T) {
		mc := &mockClient{}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "withdraw_from_container", json.RawMessage(`{"item_name":"diamond"}`))
		if !strings.Contains(res, "requires position or block_type") {
			t.Errorf("expected validation error, got %q", res)
		}
	})

	t.Run("success_with_remaining", func(t *testing.T) {
		mc := &mockClient{
			WithdrawFromContainerFunc: func(_ context.Context, _ *pb.WithdrawFromContainerRequest) (*pb.WithdrawFromContainerResponse, error) {
				return &pb.WithdrawFromContainerResponse{
					Result:             okResult("Withdrew items."),
					TransferredCount:   3,
					ContainerRemaining: []*pb.InventoryItem{{Name: "iron_ingot", Count: 10}},
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "withdraw_from_container", json.RawMessage(`{"position":{"x":0,"y":0,"z":0},"item_name":"diamond","count":3}`))
		if !strings.Contains(res, "took 3") || !strings.Contains(res, "iron_ingotx10") {
			t.Errorf("expected transfer details, got %q", res)
		}
	})

	t.Run("success_container_empty", func(t *testing.T) {
		mc := &mockClient{
			WithdrawFromContainerFunc: func(_ context.Context, _ *pb.WithdrawFromContainerRequest) (*pb.WithdrawFromContainerResponse, error) {
				return &pb.WithdrawFromContainerResponse{
					Result:           okResult("Withdrew items."),
					TransferredCount: 5,
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "withdraw_from_container", json.RawMessage(`{"block_type":"chest","item_name":"diamond","count":5}`))
		if !strings.Contains(res, "now empty") {
			t.Errorf("expected empty notice, got %q", res)
		}
	})
}

func TestHandleDepositToContainer(t *testing.T) {
	t.Run("no_item_name", func(t *testing.T) {
		mc := &mockClient{}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "deposit_to_container", json.RawMessage(`{"position":{"x":0,"y":0,"z":0}}`))
		if !strings.Contains(res, "requires item_name") {
			t.Errorf("expected validation error, got %q", res)
		}
	})

	t.Run("no_target", func(t *testing.T) {
		mc := &mockClient{}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "deposit_to_container", json.RawMessage(`{"item_name":"coal"}`))
		if !strings.Contains(res, "requires position or block_type") {
			t.Errorf("expected validation error, got %q", res)
		}
	})

	t.Run("success_with_remaining", func(t *testing.T) {
		mc := &mockClient{
			DepositToContainerFunc: func(_ context.Context, _ *pb.DepositToContainerRequest) (*pb.DepositToContainerResponse, error) {
				return &pb.DepositToContainerResponse{
					Result:             okResult("Deposited items."),
					TransferredCount:   8,
					ContainerRemaining: []*pb.InventoryItem{{Name: "coal", Count: 8}, {Name: "iron_ingot", Count: 5}},
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "deposit_to_container", json.RawMessage(`{"position":{"x":1,"y":2,"z":3},"item_name":"coal","count":8}`))
		if !strings.Contains(res, "deposited 8") || !strings.Contains(res, "Container now holds") {
			t.Errorf("expected deposit details, got %q", res)
		}
	})
}

func TestHandleCraftItem(t *testing.T) {
	t.Run("no_item_name", func(t *testing.T) {
		mc := &mockClient{}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "craft_item", json.RawMessage(`{}`))
		if !strings.Contains(res, "requires item_name") {
			t.Errorf("expected validation error, got %q", res)
		}
	})

	t.Run("count_default", func(t *testing.T) {
		mc := &mockClient{
			CraftItemFunc: func(_ context.Context, _ string, count int32) (*pb.CraftItemResponse, error) {
				if count != 1 {
					t.Errorf("expected default count 1, got %d", count)
				}
				return &pb.CraftItemResponse{
					Result:       okResult("Crafted."),
					CraftedCount: 1,
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		_, _ = d.Execute(context.Background(), "craft_item", json.RawMessage(`{"item_name":"stick"}`))
	})

	t.Run("success_with_inventory", func(t *testing.T) {
		mc := &mockClient{
			CraftItemFunc: func(_ context.Context, _ string, _ int32) (*pb.CraftItemResponse, error) {
				return &pb.CraftItemResponse{
					Result:         okResult("Crafted sticks."),
					CraftedCount:   4,
					InventoryAfter: []*pb.InventoryItem{{Name: "stick", Count: 4}},
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "craft_item", json.RawMessage(`{"item_name":"stick","count":4}`))
		if !strings.Contains(res, "crafted 4") || !strings.Contains(res, "Inventory: stickx4") {
			t.Errorf("expected craft details, got %q", res)
		}
	})

	t.Run("partial_failure", func(t *testing.T) {
		mc := &mockClient{
			CraftItemFunc: func(_ context.Context, _ string, _ int32) (*pb.CraftItemResponse, error) {
				return &pb.CraftItemResponse{
					Result:       failResult("missing materials"),
					CraftedCount: 2,
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "craft_item", json.RawMessage(`{"item_name":"planks","count":5}`))
		if !strings.Contains(res, "craft_item failed") || !strings.Contains(res, "crafted 2 before failing") {
			t.Errorf("expected partial failure, got %q", res)
		}
	})
}

func TestHandleSmeltItem(t *testing.T) {
	t.Run("no_item_name", func(t *testing.T) {
		mc := &mockClient{}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "smelt_item", json.RawMessage(`{}`))
		if !strings.Contains(res, "requires item_name") {
			t.Errorf("expected validation error, got %q", res)
		}
	})

	t.Run("success", func(t *testing.T) {
		mc := &mockClient{
			SmeltItemFunc: func(_ context.Context, itemName string, count int32, fuel string) (*pb.SmeltItemResponse, error) {
				if itemName != "raw_iron" || count != 1 || fuel != "coal" {
					t.Errorf("unexpected args: %q %d %q", itemName, count, fuel)
				}
				return &pb.SmeltItemResponse{Result: okResult("Smelted raw_iron.")}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "smelt_item", json.RawMessage(`{"item_name":"raw_iron","fuel":"coal"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "Smelted") {
			t.Errorf("got %q", res)
		}
	})
}

func TestHandleEat(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mc := &mockClient{
			EatFunc: func(_ context.Context, foodName string) (*pb.EatResponse, error) {
				if foodName != "cooked_beef" {
					t.Errorf("expected cooked_beef, got %q", foodName)
				}
				return &pb.EatResponse{
					Result:         okResult("Ate food."),
					FoodUsed:       "cooked_beef",
					HungerRestored: 8,
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "eat", json.RawMessage(`{"food_name":"cooked_beef"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, "cooked_beef") || !strings.Contains(res, "+8 hunger") {
			t.Errorf("expected food details, got %q", res)
		}
	})

	t.Run("failure", func(t *testing.T) {
		mc := &mockClient{
			EatFunc: func(_ context.Context, _ string) (*pb.EatResponse, error) {
				return &pb.EatResponse{Result: failResult("no food")}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "eat", json.RawMessage(`{}`))
		if !strings.Contains(res, "eat failed") {
			t.Errorf("expected failure, got %q", res)
		}
	})
}
