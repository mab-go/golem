package grpc

import (
	"context"
	"fmt"
	"io"

	"github.com/mab-go/golem/internal/grpc/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client wraps the generated MinecraftServiceClient with convenience methods.
type Client struct {
	conn *grpc.ClientConn
	svc  pb.MinecraftServiceClient
}

// NewClient dials the sidecar at the given address and returns a Client.
// The connection uses insecure credentials (plaintext gRPC).
func NewClient(address string) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial sidecar at %s: %w", address, err)
	}
	return &Client{
		conn: conn,
		svc:  pb.NewMinecraftServiceClient(conn),
	}, nil
}

// Close closes the underlying gRPC connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// GetVitalSigns fetches the bot's vital signs from the sidecar.
func (c *Client) GetVitalSigns(ctx context.Context) (*pb.GetVitalSignsResponse, error) {
	return c.svc.GetVitalSigns(ctx, &pb.GetVitalSignsRequest{})
}

// GetSurroundings fetches surrounding environment data from the sidecar.
func (c *Client) GetSurroundings(ctx context.Context, radius int32, fullUpdate bool) (*pb.GetSurroundingsResponse, error) {
	return c.svc.GetSurroundings(ctx, &pb.GetSurroundingsRequest{
		Radius:     radius,
		FullUpdate: fullUpdate,
	})
}

// GetInventory fetches the bot's inventory from the sidecar.
func (c *Client) GetInventory(ctx context.Context, includeCraftSuggestions bool) (*pb.GetInventoryResponse, error) {
	return c.svc.GetInventory(ctx, &pb.GetInventoryRequest{
		IncludeCraftSuggestions: includeCraftSuggestions,
	})
}

// SendChat sends a chat message through the sidecar.
func (c *Client) SendChat(ctx context.Context, message string) error {
	_, err := c.svc.SendChat(ctx, &pb.SendChatRequest{Message: message})
	return err
}

// SubscribeEvents opens a server-streaming subscription for game events.
// The caller must read from the returned stream and close it when done.
func (c *Client) SubscribeEvents(ctx context.Context, filterTypes ...pb.EventType) (pb.MinecraftService_SubscribeEventsClient, error) {
	return c.svc.SubscribeEvents(ctx, &pb.SubscribeEventsRequest{
		Filter: filterTypes,
	})
}

// ---------------------------------------------------------------------------
// Tier 0: Atomic Actions
// ---------------------------------------------------------------------------

// MoveTo moves the bot to a target position using pathfinder.
func (c *Client) MoveTo(ctx context.Context, target *pb.Vec3, rng float32, sprint bool) (*pb.MoveToResponse, error) {
	return c.svc.MoveTo(ctx, &pb.MoveToRequest{
		Target: target,
		Range:  rng,
		Sprint: sprint,
	})
}

// LookAt directs the bot's gaze at a target position.
func (c *Client) LookAt(ctx context.Context, target *pb.Vec3) (*pb.LookAtResponse, error) {
	return c.svc.LookAt(ctx, &pb.LookAtRequest{Target: target})
}

// PlaceBlock places a block from inventory onto the specified face of the reference block.
func (c *Client) PlaceBlock(ctx context.Context, pos *pb.Vec3, face, blockName string) (*pb.PlaceBlockResponse, error) {
	return c.svc.PlaceBlock(ctx, &pb.PlaceBlockRequest{
		Position:  pos,
		Face:      face,
		BlockName: blockName,
	})
}

// DigBlock digs the block at the given position.
func (c *Client) DigBlock(ctx context.Context, pos *pb.Vec3) (*pb.DigBlockResponse, error) {
	return c.svc.DigBlock(ctx, &pb.DigBlockRequest{Position: pos})
}

// EquipItem equips an item from inventory to the specified slot.
func (c *Client) EquipItem(ctx context.Context, itemName, destination string) (*pb.EquipItemResponse, error) {
	return c.svc.EquipItem(ctx, &pb.EquipItemRequest{
		ItemName:    itemName,
		Destination: destination,
	})
}

// UseItem activates the item currently held in the bot's hand.
func (c *Client) UseItem(ctx context.Context) (*pb.UseItemResponse, error) {
	return c.svc.UseItem(ctx, &pb.UseItemRequest{})
}

// AttackEntity attacks the entity with the given ID.
func (c *Client) AttackEntity(ctx context.Context, entityID int32) (*pb.AttackEntityResponse, error) {
	return c.svc.AttackEntity(ctx, &pb.AttackEntityRequest{EntityId: entityID})
}

// Jump makes the bot jump once.
func (c *Client) Jump(ctx context.Context) (*pb.JumpResponse, error) {
	return c.svc.Jump(ctx, &pb.JumpRequest{})
}

// SetSneak toggles the bot's sneak state.
func (c *Client) SetSneak(ctx context.Context, enabled bool) (*pb.SetSneakResponse, error) {
	return c.svc.SetSneak(ctx, &pb.SetSneakRequest{Enabled: enabled})
}

// ---------------------------------------------------------------------------
// Tier 1: Action Verbs
// ---------------------------------------------------------------------------

// NavigateTo navigates the bot to a target (position, entity, or block type).
func (c *Client) NavigateTo(ctx context.Context, req *pb.NavigateToRequest) (*pb.NavigateToResponse, error) {
	return c.svc.NavigateTo(ctx, req)
}

// InteractWithEntity interacts with an entity (harvest, attack, feed, trade, mount, shear).
func (c *Client) InteractWithEntity(ctx context.Context, req *pb.InteractWithEntityRequest) (*pb.InteractWithEntityResponse, error) {
	return c.svc.InteractWithEntity(ctx, req)
}

// HarvestBlock streams a harvest task that finds and mines blocks of the given type.
func (c *Client) HarvestBlock(ctx context.Context, req *pb.HarvestBlockRequest) (<-chan TaskEvent, error) {
	stream, err := c.svc.HarvestBlock(ctx, req)
	if err != nil {
		return nil, err
	}
	ch := make(chan TaskEvent, 8)
	go pumpTaskStream(stream, ch)
	return ch, nil
}

// OpenContainer opens a container at a position or finds the nearest of a block type.
func (c *Client) OpenContainer(ctx context.Context, req *pb.OpenContainerRequest) (*pb.OpenContainerResponse, error) {
	return c.svc.OpenContainer(ctx, req)
}

// WithdrawFromContainer takes items from a container into the bot's inventory.
func (c *Client) WithdrawFromContainer(ctx context.Context, req *pb.WithdrawFromContainerRequest) (*pb.WithdrawFromContainerResponse, error) {
	return c.svc.WithdrawFromContainer(ctx, req)
}

// DepositToContainer puts items from inventory into a container.
func (c *Client) DepositToContainer(ctx context.Context, req *pb.DepositToContainerRequest) (*pb.DepositToContainerResponse, error) {
	return c.svc.DepositToContainer(ctx, req)
}

// CraftItem crafts an item by name.
func (c *Client) CraftItem(ctx context.Context, itemName string, count int32) (*pb.CraftItemResponse, error) {
	return c.svc.CraftItem(ctx, &pb.CraftItemRequest{
		ItemName: itemName,
		Count:    count,
	})
}

// SmeltItem smelts an item in a nearby furnace.
func (c *Client) SmeltItem(ctx context.Context, itemName string, count int32, fuel string) (*pb.SmeltItemResponse, error) {
	return c.svc.SmeltItem(ctx, &pb.SmeltItemRequest{
		ItemName: itemName,
		Count:    count,
		Fuel:     fuel,
	})
}

// Eat consumes food. If foodName is empty, auto-selects the best available food.
func (c *Client) Eat(ctx context.Context, foodName string) (*pb.EatResponse, error) {
	return c.svc.Eat(ctx, &pb.EatRequest{FoodName: foodName})
}

// ---------------------------------------------------------------------------
// Tier 3: Strategic / Planning (read-only)
// ---------------------------------------------------------------------------

// SurveyArea scans the world around the bot and returns clusters, entities, structures.
func (c *Client) SurveyArea(ctx context.Context, radius int32) (*pb.SurveyAreaResponse, error) {
	return c.svc.SurveyArea(ctx, &pb.SurveyAreaRequest{Radius: radius})
}

// FindNearest searches for the nearest block, entity, or structure of a given type.
func (c *Client) FindNearest(ctx context.Context, req *pb.FindNearestRequest) (*pb.FindNearestResponse, error) {
	return c.svc.FindNearest(ctx, req)
}

// WhatCanICraft returns the items craftable from current inventory.
func (c *Client) WhatCanICraft(ctx context.Context) (*pb.WhatCanICraftResponse, error) {
	return c.svc.WhatCanICraft(ctx, &pb.WhatCanICraftRequest{})
}

// AssessThreat summarizes nearby hostile mobs, time until nightfall, and light level.
func (c *Client) AssessThreat(ctx context.Context, radius int32) (*pb.AssessThreatResponse, error) {
	return c.svc.AssessThreat(ctx, &pb.AssessThreatRequest{Radius: radius})
}

// PlanPath computes a path to the destination without executing it.
func (c *Client) PlanPath(ctx context.Context, destination *pb.Vec3, allowDig bool) (*pb.PlanPathResponse, error) {
	return c.svc.PlanPath(ctx, &pb.PlanPathRequest{Destination: destination, AllowDig: allowDig})
}

// ---------------------------------------------------------------------------
// Perception -- Screenshot
// ---------------------------------------------------------------------------

// TakeScreenshot requests a first-person PNG screenshot from the sidecar.
func (c *Client) TakeScreenshot(ctx context.Context, res pb.ScreenshotResolution, lookAt *pb.Vec3) (*pb.TakeScreenshotResponse, error) {
	return c.svc.TakeScreenshot(ctx, &pb.TakeScreenshotRequest{
		Resolution: res,
		LookAt:     lookAt,
	})
}

// ---------------------------------------------------------------------------
// Tier 2: Goal-Oriented Tasks (server-streaming)
// ---------------------------------------------------------------------------

// TaskEvent is one item emitted by a Tier 2 RPC stream. Progress carries the
// sidecar's TaskProgress message; Err is set only on transport failure. The
// channel returned from a Tier 2 wrapper is closed after a terminal
// TASK_COMPLETED / TASK_FAILED / TASK_CANCELLED progress message OR a
// non-nil Err.
type TaskEvent struct {
	Progress *pb.TaskProgress
	Err      error
}

// taskStream is the subset of the generated pb stream types we rely on. All
// six Tier 2 streaming client types satisfy this interface.
type taskStream interface {
	Recv() (*pb.TaskProgress, error)
}

// pumpTaskStream drains a server-streaming RPC into a TaskEvent channel.
// Closes the channel when the stream ends (io.EOF or ctx cancellation).
func pumpTaskStream(stream taskStream, out chan<- TaskEvent) {
	defer close(out)
	for {
		p, err := stream.Recv()
		if err == io.EOF {
			return
		}
		if err != nil {
			out <- TaskEvent{Err: err}
			return
		}
		out <- TaskEvent{Progress: p}
	}
}

// Gather opens a streaming Gather task. Returns a channel of TaskEvents that
// closes when the task terminates or the context is cancelled.
func (c *Client) Gather(ctx context.Context, req *pb.GatherRequest) (<-chan TaskEvent, error) {
	stream, err := c.svc.Gather(ctx, req)
	if err != nil {
		return nil, err
	}
	ch := make(chan TaskEvent, 8)
	go pumpTaskStream(stream, ch)
	return ch, nil
}

// BuildStructure opens a streaming BuildStructure task.
func (c *Client) BuildStructure(ctx context.Context, req *pb.BuildStructureRequest) (<-chan TaskEvent, error) {
	stream, err := c.svc.BuildStructure(ctx, req)
	if err != nil {
		return nil, err
	}
	ch := make(chan TaskEvent, 8)
	go pumpTaskStream(stream, ch)
	return ch, nil
}

// ProcessAll opens a streaming ProcessAll task (smelt every unit of an item).
func (c *Client) ProcessAll(ctx context.Context, req *pb.ProcessAllRequest) (<-chan TaskEvent, error) {
	stream, err := c.svc.ProcessAll(ctx, req)
	if err != nil {
		return nil, err
	}
	ch := make(chan TaskEvent, 8)
	go pumpTaskStream(stream, ch)
	return ch, nil
}

// OrganizeInventory opens a streaming OrganizeInventory task.
func (c *Client) OrganizeInventory(ctx context.Context, req *pb.OrganizeInventoryRequest) (<-chan TaskEvent, error) {
	stream, err := c.svc.OrganizeInventory(ctx, req)
	if err != nil {
		return nil, err
	}
	ch := make(chan TaskEvent, 8)
	go pumpTaskStream(stream, ch)
	return ch, nil
}

// ClearArea opens a streaming ClearArea task.
func (c *Client) ClearArea(ctx context.Context, req *pb.ClearAreaRequest) (<-chan TaskEvent, error) {
	stream, err := c.svc.ClearArea(ctx, req)
	if err != nil {
		return nil, err
	}
	ch := make(chan TaskEvent, 8)
	go pumpTaskStream(stream, ch)
	return ch, nil
}

// Farm opens a streaming Farm task.
func (c *Client) Farm(ctx context.Context, req *pb.FarmRequest) (<-chan TaskEvent, error) {
	stream, err := c.svc.Farm(ctx, req)
	if err != nil {
		return nil, err
	}
	ch := make(chan TaskEvent, 8)
	go pumpTaskStream(stream, ch)
	return ch, nil
}
