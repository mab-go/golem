package game

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mab-go/golem/internal/grpc/pb"
)

// vec3Input is the JSON shape used by Claude for 3D coordinates.
type vec3Input struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

func (v vec3Input) toPB() *pb.Vec3 { return &pb.Vec3{X: v.X, Y: v.Y, Z: v.Z} }

func (d *Dispatcher) handleMoveTo(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Target vec3Input `json:"target"`
		Range  float32   `json:"range"`
		Sprint bool      `json:"sprint"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.Range <= 0 {
		in.Range = 1
	}
	resp, err := d.client.MoveTo(ctx, in.Target.toPB(), in.Range, in.Sprint)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		msg := formatFailure("move_to", resp.Result.Error)
		if resp.DistanceRemaining > 0 {
			msg += fmt.Sprintf(" (%.1f blocks remaining)", resp.DistanceRemaining)
		}
		return msg, nil
	}
	if resp.FinalPosition != nil {
		return fmt.Sprintf("%s (now at %.0f, %.0f, %.0f)",
			resp.Result.Message, resp.FinalPosition.X, resp.FinalPosition.Y, resp.FinalPosition.Z), nil
	}
	return resp.Result.Message, nil
}

func (d *Dispatcher) handleLookAt(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Target vec3Input `json:"target"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	resp, err := d.client.LookAt(ctx, in.Target.toPB())
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return formatFailure("look_at", resp.Result.Error), nil
	}
	return resp.Result.Message, nil
}

func (d *Dispatcher) handlePlaceBlock(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Position  vec3Input `json:"position"`
		Face      string    `json:"face"`
		BlockName string    `json:"block_name"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	resp, err := d.client.PlaceBlock(ctx, in.Position.toPB(), in.Face, in.BlockName)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return formatFailure("place_block", resp.Result.Error), nil
	}
	return resp.Result.Message, nil
}

func (d *Dispatcher) handleDigBlock(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Position vec3Input `json:"position"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	resp, err := d.client.DigBlock(ctx, in.Position.toPB())
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return formatFailure("dig_block", resp.Result.Error), nil
	}
	return resp.Result.Message, nil
}

func (d *Dispatcher) handleEquipItem(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		ItemName    string `json:"item_name"`
		Destination string `json:"destination"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	resp, err := d.client.EquipItem(ctx, in.ItemName, in.Destination)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return formatFailure("equip_item", resp.Result.Error), nil
	}
	return resp.Result.Message, nil
}

func (d *Dispatcher) handleUseItem(ctx context.Context, _ json.RawMessage) (string, error) {
	resp, err := d.client.UseItem(ctx)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return formatFailure("use_item", resp.Result.Error), nil
	}
	return resp.Result.Message, nil
}

func (d *Dispatcher) handleAttackEntity(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		EntityID int32 `json:"entity_id"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	resp, err := d.client.AttackEntity(ctx, in.EntityID)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return formatFailure("attack_entity", resp.Result.Error), nil
	}
	return resp.Result.Message, nil
}

func (d *Dispatcher) handleJump(ctx context.Context, _ json.RawMessage) (string, error) {
	resp, err := d.client.Jump(ctx)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return formatFailure("jump", resp.Result.Error), nil
	}
	return resp.Result.Message, nil
}

func (d *Dispatcher) handleSetSneak(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Enabled bool `json:"enabled"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	resp, err := d.client.SetSneak(ctx, in.Enabled)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return formatFailure("set_sneak", resp.Result.Error), nil
	}
	return resp.Result.Message, nil
}

func (d *Dispatcher) handleSendChat(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Message string `json:"message"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.Message == "" {
		return "send_chat skipped: empty message", nil
	}
	if err := d.client.SendChat(ctx, in.Message); err != nil {
		return "", err
	}
	d.pub.PublishChat(d.botUsername, in.Message, true)
	return fmt.Sprintf("sent chat: %q", in.Message), nil
}
