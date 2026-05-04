package game

import (
	"context"
	"encoding/json"
	"fmt"

	sidecar "github.com/mab-go/golem/internal/grpc"
	"github.com/mab-go/golem/internal/grpc/pb"
)

func (d *Dispatcher) handleHarvestBlock(_ context.Context, input json.RawMessage) (string, error) {
	var in struct {
		BlockType   string `json:"block_type"`
		Count       int32  `json:"count"`
		MaxDistance int32  `json:"max_distance"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.BlockType == "" {
		return "harvest_block requires block_type", nil
	}
	if in.Count <= 0 {
		in.Count = 1
	}
	if in.MaxDistance <= 0 {
		in.MaxDistance = 16
	}
	req := &pb.HarvestBlockRequest{
		BlockType:   in.BlockType,
		Count:       in.Count,
		MaxDistance: in.MaxDistance,
	}
	if msg, err := d.launchTask("harvest_block", func(ctx context.Context) (<-chan sidecar.TaskEvent, error) {
		return d.client.HarvestBlock(ctx, req)
	}); msg != "" || err != nil {
		return msg, err
	}
	return fmt.Sprintf(
		"Task started: harvesting %d %s. Progress will appear in your perception stream. Use cancel_task to abort.",
		in.Count, in.BlockType), nil
}

func (d *Dispatcher) handleGather(_ context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Resource string `json:"resource"`
		Quantity int32  `json:"quantity"`
		Radius   int32  `json:"radius"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.Resource == "" {
		return "gather requires `resource`.", nil
	}
	if in.Quantity <= 0 {
		in.Quantity = 1
	}
	req := &pb.GatherRequest{
		Resource: in.Resource,
		Quantity: in.Quantity,
		Radius:   in.Radius,
	}
	if msg, err := d.launchTask("gather", func(ctx context.Context) (<-chan sidecar.TaskEvent, error) {
		return d.client.Gather(ctx, req)
	}); msg != "" || err != nil {
		return msg, err
	}
	return fmt.Sprintf("Task started: gathering %d %s. Progress will appear in your perception stream. Use cancel_task to abort.", in.Quantity, in.Resource), nil
}

func (d *Dispatcher) handleBuildStructure(_ context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Type       string    `json:"type"`
		Location   vec3Input `json:"location"`
		Dimensions vec3Input `json:"dimensions"`
		Material   string    `json:"material"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.Type == "" {
		in.Type = "shelter"
	}
	if in.Material == "" {
		return "build_structure requires `material`.", nil
	}
	req := &pb.BuildStructureRequest{
		Type:       in.Type,
		Location:   in.Location.toPB(),
		Dimensions: in.Dimensions.toPB(),
		Material:   in.Material,
	}
	if msg, err := d.launchTask("build_structure", func(ctx context.Context) (<-chan sidecar.TaskEvent, error) {
		return d.client.BuildStructure(ctx, req)
	}); msg != "" || err != nil {
		return msg, err
	}
	return fmt.Sprintf("Task started: building %s structure with %s. Progress will appear in your perception stream. Use cancel_task to abort.", in.Type, in.Material), nil
}

func (d *Dispatcher) handleProcessAll(_ context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Item string `json:"item"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.Item == "" {
		return "process_all requires `item`.", nil
	}
	req := &pb.ProcessAllRequest{Item: in.Item}
	if msg, err := d.launchTask("process_all", func(ctx context.Context) (<-chan sidecar.TaskEvent, error) {
		return d.client.ProcessAll(ctx, req)
	}); msg != "" || err != nil {
		return msg, err
	}
	return fmt.Sprintf("Task started: processing all %s. Progress will appear in your perception stream. Use cancel_task to abort.", in.Item), nil
}

func (d *Dispatcher) handleOrganizeInventory(_ context.Context, input json.RawMessage) (string, error) {
	var in struct {
		StashJunk bool `json:"stash_junk"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	req := &pb.OrganizeInventoryRequest{StashJunk: in.StashJunk}
	if msg, err := d.launchTask("organize_inventory", func(ctx context.Context) (<-chan sidecar.TaskEvent, error) {
		return d.client.OrganizeInventory(ctx, req)
	}); msg != "" || err != nil {
		return msg, err
	}
	return "Task started: organizing inventory. Progress will appear in your perception stream. Use cancel_task to abort.", nil
}

func (d *Dispatcher) handleClearArea(_ context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Center vec3Input `json:"center"`
		Radius int32     `json:"radius"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.Radius <= 0 {
		in.Radius = 4
	}
	req := &pb.ClearAreaRequest{
		Center: in.Center.toPB(),
		Radius: in.Radius,
	}
	if msg, err := d.launchTask("clear_area", func(ctx context.Context) (<-chan sidecar.TaskEvent, error) {
		return d.client.ClearArea(ctx, req)
	}); msg != "" || err != nil {
		return msg, err
	}
	return fmt.Sprintf("Task started: clearing area (radius %d). Progress will appear in your perception stream. Use cancel_task to abort.", in.Radius), nil
}

func (d *Dispatcher) handleFarm(_ context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Crop     string    `json:"crop"`
		Location vec3Input `json:"location"`
		Radius   int32     `json:"radius"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.Crop == "" {
		return "farm requires `crop`.", nil
	}
	req := &pb.FarmRequest{
		Crop:     in.Crop,
		Location: in.Location.toPB(),
		Radius:   in.Radius,
	}
	if msg, err := d.launchTask("farm", func(ctx context.Context) (<-chan sidecar.TaskEvent, error) {
		return d.client.Farm(ctx, req)
	}); msg != "" || err != nil {
		return msg, err
	}
	return fmt.Sprintf("Task started: farming %s. Progress will appear in your perception stream. Use cancel_task to abort.", in.Crop), nil
}

func (d *Dispatcher) launchTask(name string, dial func(context.Context) (<-chan sidecar.TaskEvent, error)) (string, error) {
	taskCtx, taskCancel := d.taskMgr.NewTaskContext()
	ch, err := dial(taskCtx)
	if err != nil {
		taskCancel()
		return "", err
	}
	if err := d.taskMgr.Start(taskCtx, taskCancel, name, ch); err != nil {
		taskCancel()
		return err.Error(), nil
	}
	return "", nil
}
