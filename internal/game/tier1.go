package game

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mab-go/golem/internal/grpc/pb"
)

func (d *Dispatcher) handleNavigateTo(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Position   *vec3Input `json:"position,omitempty"`
		EntityName string     `json:"entity_name"`
		BlockType  string     `json:"block_type"`
		Range      float32    `json:"range"`
		AllowDig   bool       `json:"allow_dig"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	req := &pb.NavigateToRequest{Range: in.Range, AllowDig: in.AllowDig}
	switch {
	case in.Position != nil:
		req.Target = &pb.NavigateToRequest_Position{Position: in.Position.toPB()}
	case in.EntityName != "":
		req.Target = &pb.NavigateToRequest_EntityName{EntityName: in.EntityName}
	case in.BlockType != "":
		req.Target = &pb.NavigateToRequest_BlockType{BlockType: in.BlockType}
	default:
		return "navigate_to requires one of position, entity_name, or block_type", nil
	}
	resp, err := d.client.NavigateTo(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return formatFailure("navigate_to", resp.Result.Error), nil
	}
	return fmt.Sprintf("%s (traveled %.1f blocks)", resp.Result.Message, resp.DistanceTraveled), nil
}

var interactionKinds = map[string]pb.InteractionType{
	"harvest": pb.InteractionType_INTERACTION_HARVEST,
	"attack":  pb.InteractionType_INTERACTION_ATTACK,
	"feed":    pb.InteractionType_INTERACTION_FEED,
	"trade":   pb.InteractionType_INTERACTION_TRADE,
	"mount":   pb.InteractionType_INTERACTION_MOUNT,
	"shear":   pb.InteractionType_INTERACTION_SHEAR,
}

func (d *Dispatcher) handleInteractWithEntity(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		EntityName string `json:"entity_name"`
		EntityID   int32  `json:"entity_id"`
		Action     string `json:"action"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	kind, ok := interactionKinds[strings.ToLower(in.Action)]
	if !ok {
		return fmt.Sprintf("unknown interaction action %q (want harvest|attack|feed|trade|mount|shear)", in.Action), nil
	}
	if in.EntityName == "" && in.EntityID == 0 {
		return "interact_with_entity requires entity_name or entity_id", nil
	}
	req := &pb.InteractWithEntityRequest{
		EntityName: in.EntityName,
		EntityId:   in.EntityID,
		Action:     kind,
	}
	resp, err := d.client.InteractWithEntity(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return formatFailure("interact_with_entity", resp.Result.Error), nil
	}
	msg := resp.Result.Message
	if resp.Description != "" {
		msg = resp.Description
	}
	if len(resp.Drops) > 0 {
		msg += " (drops: " + formatItemList(resp.Drops) + ")"
	}
	return msg, nil
}

func (d *Dispatcher) handleOpenContainer(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Position  *vec3Input `json:"position,omitempty"`
		BlockType string     `json:"block_type"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	req := &pb.OpenContainerRequest{}
	switch {
	case in.Position != nil:
		req.Target = &pb.OpenContainerRequest_Position{Position: in.Position.toPB()}
	case in.BlockType != "":
		req.Target = &pb.OpenContainerRequest_BlockType{BlockType: in.BlockType}
	default:
		return "open_container requires position or block_type", nil
	}
	resp, err := d.client.OpenContainer(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return formatFailure("open_container", resp.Result.Error), nil
	}
	if len(resp.Contents) == 0 {
		return fmt.Sprintf("opened %s (empty)", resp.ContainerType), nil
	}
	return fmt.Sprintf("opened %s: %s", resp.ContainerType, formatItemList(resp.Contents)), nil
}

func (d *Dispatcher) handleWithdrawFromContainer(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Position  *vec3Input `json:"position,omitempty"`
		BlockType string     `json:"block_type"`
		ItemName  string     `json:"item_name"`
		Count     int32      `json:"count"`
		Slot      string     `json:"slot"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.ItemName == "" {
		return "withdraw_from_container requires item_name", nil
	}
	req := &pb.WithdrawFromContainerRequest{
		ItemName: in.ItemName,
		Count:    in.Count,
		Slot:     in.Slot,
	}
	switch {
	case in.Position != nil:
		req.Target = &pb.WithdrawFromContainerRequest_Position{Position: in.Position.toPB()}
	case in.BlockType != "":
		req.Target = &pb.WithdrawFromContainerRequest_BlockType{BlockType: in.BlockType}
	default:
		return "withdraw_from_container requires position or block_type", nil
	}
	resp, err := d.client.WithdrawFromContainer(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return formatFailure("withdraw_from_container", resp.Result.Error), nil
	}
	msg := fmt.Sprintf("%s (took %d)", resp.Result.Message, resp.TransferredCount)
	if len(resp.ContainerRemaining) > 0 {
		msg += "\nContainer remaining: " + formatItemList(resp.ContainerRemaining)
	} else {
		msg += "\nContainer is now empty."
	}
	return msg, nil
}

func (d *Dispatcher) handleDepositToContainer(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Position  *vec3Input `json:"position,omitempty"`
		BlockType string     `json:"block_type"`
		ItemName  string     `json:"item_name"`
		Count     int32      `json:"count"`
		Slot      string     `json:"slot"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.ItemName == "" {
		return "deposit_to_container requires item_name", nil
	}
	req := &pb.DepositToContainerRequest{
		ItemName: in.ItemName,
		Count:    in.Count,
		Slot:     in.Slot,
	}
	switch {
	case in.Position != nil:
		req.Target = &pb.DepositToContainerRequest_Position{Position: in.Position.toPB()}
	case in.BlockType != "":
		req.Target = &pb.DepositToContainerRequest_BlockType{BlockType: in.BlockType}
	default:
		return "deposit_to_container requires position or block_type", nil
	}
	resp, err := d.client.DepositToContainer(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return formatFailure("deposit_to_container", resp.Result.Error), nil
	}
	msg := fmt.Sprintf("%s (deposited %d)", resp.Result.Message, resp.TransferredCount)
	if len(resp.ContainerRemaining) > 0 {
		msg += "\nContainer now holds: " + formatItemList(resp.ContainerRemaining)
	}
	return msg, nil
}

func (d *Dispatcher) handleCraftItem(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		ItemName string `json:"item_name"`
		Count    int32  `json:"count"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.ItemName == "" {
		return "craft_item requires item_name", nil
	}
	if in.Count <= 0 {
		in.Count = 1
	}
	resp, err := d.client.CraftItem(ctx, in.ItemName, in.Count)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		msg := formatFailure("craft_item", resp.Result.Error)
		if resp.CraftedCount > 0 {
			msg += fmt.Sprintf(" (crafted %d before failing)", resp.CraftedCount)
		}
		msg += formatInventoryAfter(resp.InventoryAfter)
		return msg, nil
	}
	msg := fmt.Sprintf("%s (crafted %d)", resp.Result.Message, resp.CraftedCount)
	msg += formatInventoryAfter(resp.InventoryAfter)
	return msg, nil
}

func (d *Dispatcher) handleSmeltItem(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		ItemName string `json:"item_name"`
		Count    int32  `json:"count"`
		Fuel     string `json:"fuel"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.ItemName == "" {
		return "smelt_item requires item_name", nil
	}
	if in.Count <= 0 {
		in.Count = 1
	}
	resp, err := d.client.SmeltItem(ctx, in.ItemName, in.Count, in.Fuel)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return formatFailure("smelt_item", resp.Result.Error), nil
	}
	return resp.Result.Message, nil
}

func (d *Dispatcher) handleEat(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		FoodName string `json:"food_name"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	resp, err := d.client.Eat(ctx, in.FoodName)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return formatFailure("eat", resp.Result.Error), nil
	}
	return fmt.Sprintf("%s (%s, +%d hunger)", resp.Result.Message, resp.FoodUsed, resp.HungerRestored), nil
}

func formatItemList(items []*pb.InventoryItem) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, fmt.Sprintf("%sx%d", it.Name, it.Count))
	}
	return strings.Join(parts, ", ")
}

func formatInventoryAfter(items []*pb.InventoryItem) string {
	if len(items) == 0 {
		return ""
	}
	return "\nInventory: " + formatItemList(items)
}
