package game

import (
	"context"

	sidecar "github.com/mab-go/golem/internal/grpc"
	"github.com/mab-go/golem/internal/grpc/pb"
)

// Client is the subset of sidecar.Client that the Dispatcher calls.
// Defined here (consumer side) so the game package does not depend on the
// concrete client struct for testing.
type Client interface {
	// Tier 0
	MoveTo(ctx context.Context, target *pb.Vec3, rng float32, sprint bool) (*pb.MoveToResponse, error)
	LookAt(ctx context.Context, target *pb.Vec3) (*pb.LookAtResponse, error)
	PlaceBlock(ctx context.Context, pos *pb.Vec3, face, blockName string) (*pb.PlaceBlockResponse, error)
	DigBlock(ctx context.Context, pos *pb.Vec3) (*pb.DigBlockResponse, error)
	EquipItem(ctx context.Context, itemName, destination string) (*pb.EquipItemResponse, error)
	UseItem(ctx context.Context) (*pb.UseItemResponse, error)
	AttackEntity(ctx context.Context, entityID int32) (*pb.AttackEntityResponse, error)
	Jump(ctx context.Context) (*pb.JumpResponse, error)
	SetSneak(ctx context.Context, enabled bool) (*pb.SetSneakResponse, error)
	SendChat(ctx context.Context, message string) error

	// Tier 1
	NavigateTo(ctx context.Context, req *pb.NavigateToRequest) (*pb.NavigateToResponse, error)
	InteractWithEntity(ctx context.Context, req *pb.InteractWithEntityRequest) (*pb.InteractWithEntityResponse, error)
	OpenContainer(ctx context.Context, req *pb.OpenContainerRequest) (*pb.OpenContainerResponse, error)
	WithdrawFromContainer(ctx context.Context, req *pb.WithdrawFromContainerRequest) (*pb.WithdrawFromContainerResponse, error)
	DepositToContainer(ctx context.Context, req *pb.DepositToContainerRequest) (*pb.DepositToContainerResponse, error)
	CraftItem(ctx context.Context, itemName string, count int32) (*pb.CraftItemResponse, error)
	SmeltItem(ctx context.Context, itemName string, count int32, fuel string) (*pb.SmeltItemResponse, error)
	Eat(ctx context.Context, foodName string) (*pb.EatResponse, error)

	// Tier 2 (streaming)
	HarvestBlock(ctx context.Context, req *pb.HarvestBlockRequest) (<-chan sidecar.TaskEvent, error)
	Gather(ctx context.Context, req *pb.GatherRequest) (<-chan sidecar.TaskEvent, error)
	BuildStructure(ctx context.Context, req *pb.BuildStructureRequest) (<-chan sidecar.TaskEvent, error)
	ProcessAll(ctx context.Context, req *pb.ProcessAllRequest) (<-chan sidecar.TaskEvent, error)
	OrganizeInventory(ctx context.Context, req *pb.OrganizeInventoryRequest) (<-chan sidecar.TaskEvent, error)
	ClearArea(ctx context.Context, req *pb.ClearAreaRequest) (<-chan sidecar.TaskEvent, error)
	Farm(ctx context.Context, req *pb.FarmRequest) (<-chan sidecar.TaskEvent, error)

	// Tier 3
	SurveyArea(ctx context.Context, radius int32) (*pb.SurveyAreaResponse, error)
	FindNearest(ctx context.Context, req *pb.FindNearestRequest) (*pb.FindNearestResponse, error)
	WhatCanICraft(ctx context.Context) (*pb.WhatCanICraftResponse, error)
	AssessThreat(ctx context.Context, radius int32) (*pb.AssessThreatResponse, error)
	PlanPath(ctx context.Context, destination *pb.Vec3, allowDig bool) (*pb.PlanPathResponse, error)

	// Perception
	GetSurroundings(ctx context.Context, radius int32, fullUpdate bool) (*pb.GetSurroundingsResponse, error)
	GetInventory(ctx context.Context, includeCraftSuggestions bool) (*pb.GetInventoryResponse, error)
	TakeScreenshot(ctx context.Context, res pb.ScreenshotResolution, lookAt *pb.Vec3) (*pb.TakeScreenshotResponse, error)
}
