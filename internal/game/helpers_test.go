package game

import (
	"encoding/json"
	"testing"
	"time"

	pb "github.com/mab-go/golem/internal/grpc/pb"
)

func TestFormatFailure(t *testing.T) {
	tests := []struct {
		kind, detail, want string
	}{
		{"move_to", "blocked by wall", "move_to failed: blocked by wall"},
		{"jump", "", "jump failed."},
	}
	for _, tt := range tests {
		if got := formatFailure(tt.kind, tt.detail); got != tt.want {
			t.Errorf("formatFailure(%q, %q) = %q, want %q", tt.kind, tt.detail, got, tt.want)
		}
	}
}

func TestTruncatePreview(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"short", "hello", "hello"},
		{"exactly_80", "01234567890123456789012345678901234567890123456789012345678901234567890123456789", "01234567890123456789012345678901234567890123456789012345678901234567890123456789"},
		{"over_80", "012345678901234567890123456789012345678901234567890123456789012345678901234567890", "01234567890123456789012345678901234567890123456789012345678901234567890123456789..."},
		{"whitespace_trimmed", "  hello  ", "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncatePreview(tt.input); got != tt.want {
				t.Errorf("truncatePreview(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		var out struct{ Name string }
		if err := decode(json.RawMessage(`{"name":"test"}`), &out); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Name != "test" {
			t.Errorf("Name = %q, want %q", out.Name, "test")
		}
	})

	t.Run("invalid", func(t *testing.T) {
		var out struct{ Name string }
		err := decode(json.RawMessage(`not json`), &out)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
		if got := err.Error(); got == "" {
			t.Error("expected non-empty error message")
		}
	})
}

func TestVec3InputToPB(t *testing.T) {
	v := vec3Input{X: 1.5, Y: -2.0, Z: 3.25}
	got := v.toPB()
	if got.X != 1.5 || got.Y != -2.0 || got.Z != 3.25 {
		t.Errorf("toPB() = (%v, %v, %v), want (1.5, -2, 3.25)", got.X, got.Y, got.Z)
	}
}

func TestScaledTimeout(t *testing.T) {
	tests := []struct {
		name  string
		base  time.Duration
		per   time.Duration
		count int32
		want  time.Duration
	}{
		{"normal", 30 * time.Second, 15 * time.Second, 4, 90 * time.Second},
		{"capped", 30 * time.Second, 15 * time.Second, 200, maxToolTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scaledTimeout(tt.base, tt.per, tt.count); got != tt.want {
				t.Errorf("scaledTimeout = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatItemList(t *testing.T) {
	tests := []struct {
		name  string
		items []*pb.InventoryItem
		want  string
	}{
		{"empty", nil, ""},
		{"single", []*pb.InventoryItem{{Name: "diamond", Count: 3}}, "diamondx3"},
		{"multiple", []*pb.InventoryItem{
			{Name: "iron_ingot", Count: 5},
			{Name: "coal", Count: 12},
		}, "iron_ingotx5, coalx12"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatItemList(tt.items); got != tt.want {
				t.Errorf("formatItemList = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatInventoryAfter(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if got := formatInventoryAfter(nil); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("non_empty", func(t *testing.T) {
		items := []*pb.InventoryItem{{Name: "stick", Count: 4}}
		got := formatInventoryAfter(items)
		want := "\nInventory: stickx4"
		if got != want {
			t.Errorf("formatInventoryAfter = %q, want %q", got, want)
		}
	})
}

func TestFormatSurveyAreaResult(t *testing.T) {
	tests := []struct {
		name string
		resp *pb.SurveyAreaResponse
		want string
	}{
		{"nil", nil, "Survey returned no data."},
		{"with_summary", &pb.SurveyAreaResponse{Summary: "A forest clearing."}, "A forest clearing."},
		{"without_summary", &pb.SurveyAreaResponse{
			BlockClusters: []*pb.BlockCluster{{}, {}},
			Entities:      []*pb.Entity{{}},
			Structures:    nil,
		}, "Surveyed area: 2 block clusters, 1 entities, 0 structures"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSurveyAreaResult(tt.resp); got != tt.want {
				t.Errorf("formatSurveyAreaResult = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestQueryLabel(t *testing.T) {
	type input struct {
		BlockType     string `json:"block_type"`
		EntityType    string `json:"entity_type"`
		StructureType string `json:"structure_type"`
		Radius        int32  `json:"radius"`
	}
	tests := []struct {
		name string
		in   input
		want string
	}{
		{"block", input{BlockType: "diamond_ore"}, "diamond_ore"},
		{"entity", input{EntityType: "cow"}, "cow"},
		{"structure", input{StructureType: "village"}, "village"},
		{"none", input{}, "target"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := queryLabel(tt.in); got != tt.want {
				t.Errorf("queryLabel = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestThreatLevelLabel(t *testing.T) {
	tests := []struct {
		lv   pb.ThreatLevel
		want string
	}{
		{pb.ThreatLevel_THREAT_SAFE, "safe"},
		{pb.ThreatLevel_THREAT_LOW, "low"},
		{pb.ThreatLevel_THREAT_MODERATE, "moderate"},
		{pb.ThreatLevel_THREAT_HIGH, "high"},
		{pb.ThreatLevel_THREAT_CRITICAL, "critical"},
		{pb.ThreatLevel(999), "unknown"},
	}
	for _, tt := range tests {
		if got := threatLevelLabel(tt.lv); got != tt.want {
			t.Errorf("threatLevelLabel(%v) = %q, want %q", tt.lv, got, tt.want)
		}
	}
}
