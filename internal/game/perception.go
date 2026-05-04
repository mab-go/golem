package game

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mab-go/golem/internal/grpc/pb"
)

func (d *Dispatcher) handleLookAround(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Radius int32 `json:"radius"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.Radius <= 0 {
		in.Radius = 16
	}
	resp, err := d.client.GetSurroundings(ctx, in.Radius, false)
	if err != nil {
		return "", err
	}
	// Return a terse summary directly; the next turn's perception block will
	// carry the full render via the formatter.
	entities := len(resp.NearbyEntities)
	blocks := len(resp.NotableBlocks)
	biome := resp.Biome
	if biome == "" {
		biome = "unknown biome"
	}
	return fmt.Sprintf("Looked around: %s, %s, %d entities, %d notable blocks, light %d",
		strings.ReplaceAll(biome, "_", " "), resp.TimeOfDay, entities, blocks, resp.LightLevel), nil
}

func (d *Dispatcher) handleCheckInventory(ctx context.Context, _ json.RawMessage) (string, error) {
	resp, err := d.client.GetInventory(ctx, true)
	if err != nil {
		return "", err
	}
	if len(resp.Items) == 0 {
		return fmt.Sprintf("Inventory empty (%d slots free)", resp.EmptySlots), nil
	}
	parts := make([]string, 0, len(resp.Items))
	for _, it := range resp.Items {
		parts = append(parts, fmt.Sprintf("%sx%d", it.Name, it.Count))
	}
	return fmt.Sprintf("Inventory: %s (%d slots free)", strings.Join(parts, ", "), resp.EmptySlots), nil
}

// handleTakeScreenshot captures a first-person PNG via the sidecar and returns
// camera metadata plus the raw image. The player model interprets the image
// directly in its next reasoning step.
func (d *Dispatcher) handleTakeScreenshot(ctx context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Resolution string     `json:"resolution"`
		LookAt     *vec3Input `json:"look_at,omitempty"`
	}
	if err := decode(input, &in); err != nil {
		return Result{Text: err.Error()}, nil
	}

	res := pb.ScreenshotResolution_SCREENSHOT_RESOLUTION_LOW
	if in.Resolution == "high" {
		res = pb.ScreenshotResolution_SCREENSHOT_RESOLUTION_HIGH
	}

	var lookAt *pb.Vec3
	if in.LookAt != nil {
		lookAt = in.LookAt.toPB()
	}

	resp, err := d.client.TakeScreenshot(ctx, res, lookAt)
	if err != nil {
		return Result{Text: fmt.Sprintf("screenshot unavailable: %v", err)}, nil
	}

	meta := fmt.Sprintf("Screenshot %dx%d from (%.1f, %.1f, %.1f), yaw=%.1f pitch=%.1f.",
		resp.Width, resp.Height,
		resp.CameraPosition.GetX(), resp.CameraPosition.GetY(), resp.CameraPosition.GetZ(),
		resp.CameraYaw, resp.CameraPitch)

	return Result{Text: meta, ImagePNG: resp.ImageData}, nil
}
