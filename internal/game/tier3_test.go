package game

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	pb "github.com/mab-go/golem/internal/grpc/pb"
)

func TestHandleSurveyArea(t *testing.T) {
	t.Run("radius_defaults", func(t *testing.T) {
		mc := &mockClient{
			SurveyAreaFunc: func(_ context.Context, radius int32) (*pb.SurveyAreaResponse, error) {
				if radius != 32 {
					t.Errorf("expected default radius 32, got %d", radius)
				}
				return &pb.SurveyAreaResponse{Summary: "Forest clearing."}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, err := d.Execute(context.Background(), "survey_area", json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != "Forest clearing." {
			t.Errorf("got %q", res)
		}
	})

	t.Run("radius_capped", func(t *testing.T) {
		mc := &mockClient{
			SurveyAreaFunc: func(_ context.Context, radius int32) (*pb.SurveyAreaResponse, error) {
				if radius != 128 {
					t.Errorf("expected capped radius 128, got %d", radius)
				}
				return &pb.SurveyAreaResponse{Summary: "Wide survey."}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		_, _ = d.Execute(context.Background(), "survey_area", json.RawMessage(`{"radius":500}`))
	})

	t.Run("transport_error", func(t *testing.T) {
		mc := &mockClient{
			SurveyAreaFunc: func(_ context.Context, _ int32) (*pb.SurveyAreaResponse, error) {
				return nil, errors.New("rpc failed")
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		_, err := d.Execute(context.Background(), "survey_area", json.RawMessage(`{}`))
		if err == nil {
			t.Fatal("expected transport error")
		}
	})
}

func TestHandleFindNearest(t *testing.T) {
	t.Run("no_target", func(t *testing.T) {
		mc := &mockClient{}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "find_nearest", json.RawMessage(`{}`))
		if !strings.Contains(res, "requires exactly one of") {
			t.Errorf("expected validation error, got %q", res)
		}
	})

	t.Run("by_block_type_found", func(t *testing.T) {
		mc := &mockClient{
			FindNearestFunc: func(_ context.Context, req *pb.FindNearestRequest) (*pb.FindNearestResponse, error) {
				if req.GetBlockType() != "diamond_ore" {
					t.Errorf("expected diamond_ore, got %q", req.GetBlockType())
				}
				return &pb.FindNearestResponse{
					Found:    true,
					Type:     "diamond_ore",
					Position: &pb.Vec3{X: 10, Y: 12, Z: -5},
					Distance: 15.3,
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "find_nearest", json.RawMessage(`{"block_type":"diamond_ore"}`))
		if !strings.Contains(res, "diamond_ore") || !strings.Contains(res, "15.3m") {
			t.Errorf("expected found result, got %q", res)
		}
	})

	t.Run("by_entity_not_found", func(t *testing.T) {
		mc := &mockClient{
			FindNearestFunc: func(_ context.Context, _ *pb.FindNearestRequest) (*pb.FindNearestResponse, error) {
				return &pb.FindNearestResponse{Found: false}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "find_nearest", json.RawMessage(`{"entity_type":"villager","radius":50}`))
		if !strings.Contains(res, "No") || !strings.Contains(res, "50 blocks") {
			t.Errorf("expected not-found, got %q", res)
		}
	})

	t.Run("found_nil_position", func(t *testing.T) {
		mc := &mockClient{
			FindNearestFunc: func(_ context.Context, _ *pb.FindNearestRequest) (*pb.FindNearestResponse, error) {
				return &pb.FindNearestResponse{
					Found:    true,
					Type:     "village",
					Position: nil,
					Distance: 100,
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "find_nearest", json.RawMessage(`{"structure_type":"village"}`))
		if !strings.Contains(res, "Found village") {
			t.Errorf("expected found, got %q", res)
		}
	})

	t.Run("radius_defaults_and_caps", func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			input      string
			wantRadius int32
		}{
			{"default", `{"block_type":"iron_ore"}`, 64},
			{"capped", `{"block_type":"iron_ore","radius":200}`, 128},
		} {
			t.Run(tc.name, func(t *testing.T) {
				mc := &mockClient{
					FindNearestFunc: func(_ context.Context, req *pb.FindNearestRequest) (*pb.FindNearestResponse, error) {
						if req.Radius != tc.wantRadius {
							t.Errorf("expected radius %d, got %d", tc.wantRadius, req.Radius)
						}
						return &pb.FindNearestResponse{Found: false}, nil
					},
				}
				d := newTestDispatcherWithClient(t, mc)
				_, _ = d.Execute(context.Background(), "find_nearest", json.RawMessage(tc.input))
			})
		}
	})
}

func TestHandleWhatCanICraft(t *testing.T) {
	t.Run("nothing_craftable", func(t *testing.T) {
		mc := &mockClient{
			WhatCanICraftFunc: func(_ context.Context) (*pb.WhatCanICraftResponse, error) {
				return &pb.WhatCanICraftResponse{}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "what_can_i_craft", json.RawMessage(`{}`))
		if !strings.Contains(res, "Nothing can be crafted") {
			t.Errorf("expected empty result, got %q", res)
		}
	})

	t.Run("items_with_table", func(t *testing.T) {
		mc := &mockClient{
			WhatCanICraftFunc: func(_ context.Context) (*pb.WhatCanICraftResponse, error) {
				return &pb.WhatCanICraftResponse{
					Items: []*pb.CraftableItem{
						{ItemName: "stick", MaxCount: 4, RequiresCraftingTable: false},
						{ItemName: "wooden_pickaxe", MaxCount: 1, RequiresCraftingTable: true},
					},
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "what_can_i_craft", json.RawMessage(`{}`))
		if !strings.Contains(res, "stickx4") || !strings.Contains(res, "wooden_pickaxex1 (table)") {
			t.Errorf("expected items with table marker, got %q", res)
		}
	})
}

func TestHandleAssessThreat(t *testing.T) {
	t.Run("with_summary", func(t *testing.T) {
		mc := &mockClient{
			AssessThreatFunc: func(_ context.Context, radius int32) (*pb.AssessThreatResponse, error) {
				if radius != 24 {
					t.Errorf("expected default radius 24, got %d", radius)
				}
				return &pb.AssessThreatResponse{Summary: "All clear."}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "assess_threat", json.RawMessage(`{}`))
		if res != "All clear." {
			t.Errorf("got %q", res)
		}
	})

	t.Run("structured_response", func(t *testing.T) {
		mc := &mockClient{
			AssessThreatFunc: func(_ context.Context, _ int32) (*pb.AssessThreatResponse, error) {
				return &pb.AssessThreatResponse{
					ThreatLevel:        pb.ThreatLevel_THREAT_HIGH,
					HostileEntities:    []*pb.Entity{{}, {}},
					LocalLightLevel:    3,
					TimeUntilNightfall: 120,
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "assess_threat", json.RawMessage(`{"radius":32}`))
		if !strings.Contains(res, "high") || !strings.Contains(res, "2 hostiles") {
			t.Errorf("expected structured threat, got %q", res)
		}
	})
}

func TestHandlePlanPathReachable(t *testing.T) {
	t.Run("with_hazards", func(t *testing.T) {
		mc := &mockClient{
			PlanPathFunc: func(_ context.Context, dest *pb.Vec3, allowDig bool) (*pb.PlanPathResponse, error) {
				if dest.X != 100 || !allowDig {
					t.Errorf("unexpected args: dest=%v allowDig=%v", dest, allowDig)
				}
				return &pb.PlanPathResponse{
					Reachable:        true,
					Distance:         45.2,
					EstimatedSeconds: 30,
					Hazards:          []string{"lava", "ravine"},
					Waypoints:        []*pb.Vec3{{}, {}, {}},
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "plan_path", json.RawMessage(`{"destination":{"x":100,"y":64,"z":0},"allow_dig":true}`))
		if !strings.Contains(res, "Reachable") || !strings.Contains(res, "lava") || !strings.Contains(res, "3 waypoints") {
			t.Errorf("expected path details, got %q", res)
		}
	})

	t.Run("no_hazards", func(t *testing.T) {
		mc := &mockClient{
			PlanPathFunc: func(_ context.Context, _ *pb.Vec3, _ bool) (*pb.PlanPathResponse, error) {
				return &pb.PlanPathResponse{
					Reachable:        true,
					Distance:         10,
					EstimatedSeconds: 5,
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "plan_path", json.RawMessage(`{"destination":{"x":10,"y":64,"z":0}}`))
		if !strings.Contains(res, "Reachable") || strings.Contains(res, "hazards") {
			t.Errorf("expected clean path, got %q", res)
		}
	})
}

func TestHandlePlanPathUnreachable(t *testing.T) {
	t.Run("with_hazards", func(t *testing.T) {
		mc := &mockClient{
			PlanPathFunc: func(_ context.Context, _ *pb.Vec3, _ bool) (*pb.PlanPathResponse, error) {
				return &pb.PlanPathResponse{
					Reachable: false,
					Hazards:   []string{"void"},
				}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "plan_path", json.RawMessage(`{"destination":{"x":0,"y":0,"z":0}}`))
		if !strings.Contains(res, "unreachable") || !strings.Contains(res, "void") {
			t.Errorf("expected unreachable, got %q", res)
		}
	})

	t.Run("no_hazards", func(t *testing.T) {
		mc := &mockClient{
			PlanPathFunc: func(_ context.Context, _ *pb.Vec3, _ bool) (*pb.PlanPathResponse, error) {
				return &pb.PlanPathResponse{Reachable: false}, nil
			},
		}
		d := newTestDispatcherWithClient(t, mc)
		res, _ := d.Execute(context.Background(), "plan_path", json.RawMessage(`{"destination":{"x":0,"y":0,"z":0}}`))
		if !strings.Contains(res, "unreachable") || !strings.Contains(res, "none") {
			t.Errorf("expected unreachable with none hazards, got %q", res)
		}
	})
}
