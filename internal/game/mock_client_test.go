package game

import (
	"context"
	"testing"
	"time"

	sidecar "github.com/mab-go/golem/internal/grpc"
	pb "github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/logging"
	"github.com/mab-go/golem/internal/memory"
	"github.com/mab-go/golem/internal/perception"
	"github.com/mab-go/golem/internal/task"
)

var _ Client = (*mockClient)(nil)

type mockClient struct {
	MoveToFunc                func(ctx context.Context, target *pb.Vec3, rng float32, sprint bool) (*pb.MoveToResponse, error)
	LookAtFunc                func(ctx context.Context, target *pb.Vec3) (*pb.LookAtResponse, error)
	PlaceBlockFunc            func(ctx context.Context, pos *pb.Vec3, face, blockName string) (*pb.PlaceBlockResponse, error)
	DigBlockFunc              func(ctx context.Context, pos *pb.Vec3) (*pb.DigBlockResponse, error)
	EquipItemFunc             func(ctx context.Context, itemName, destination string) (*pb.EquipItemResponse, error)
	UseItemFunc               func(ctx context.Context) (*pb.UseItemResponse, error)
	AttackEntityFunc          func(ctx context.Context, entityID int32) (*pb.AttackEntityResponse, error)
	JumpFunc                  func(ctx context.Context) (*pb.JumpResponse, error)
	SetSneakFunc              func(ctx context.Context, enabled bool) (*pb.SetSneakResponse, error)
	SendChatFunc              func(ctx context.Context, message string) error
	NavigateToFunc            func(ctx context.Context, req *pb.NavigateToRequest) (*pb.NavigateToResponse, error)
	InteractWithEntityFunc    func(ctx context.Context, req *pb.InteractWithEntityRequest) (*pb.InteractWithEntityResponse, error)
	OpenContainerFunc         func(ctx context.Context, req *pb.OpenContainerRequest) (*pb.OpenContainerResponse, error)
	WithdrawFromContainerFunc func(ctx context.Context, req *pb.WithdrawFromContainerRequest) (*pb.WithdrawFromContainerResponse, error)
	DepositToContainerFunc    func(ctx context.Context, req *pb.DepositToContainerRequest) (*pb.DepositToContainerResponse, error)
	CraftItemFunc             func(ctx context.Context, itemName string, count int32) (*pb.CraftItemResponse, error)
	SmeltItemFunc             func(ctx context.Context, itemName string, count int32, fuel string) (*pb.SmeltItemResponse, error)
	EatFunc                   func(ctx context.Context, foodName string) (*pb.EatResponse, error)
	HarvestBlockFunc          func(ctx context.Context, req *pb.HarvestBlockRequest) (<-chan sidecar.TaskEvent, error)
	GatherFunc                func(ctx context.Context, req *pb.GatherRequest) (<-chan sidecar.TaskEvent, error)
	BuildStructureFunc        func(ctx context.Context, req *pb.BuildStructureRequest) (<-chan sidecar.TaskEvent, error)
	ProcessAllFunc            func(ctx context.Context, req *pb.ProcessAllRequest) (<-chan sidecar.TaskEvent, error)
	OrganizeInventoryFunc     func(ctx context.Context, req *pb.OrganizeInventoryRequest) (<-chan sidecar.TaskEvent, error)
	ClearAreaFunc             func(ctx context.Context, req *pb.ClearAreaRequest) (<-chan sidecar.TaskEvent, error)
	FarmFunc                  func(ctx context.Context, req *pb.FarmRequest) (<-chan sidecar.TaskEvent, error)
	SurveyAreaFunc            func(ctx context.Context, radius int32) (*pb.SurveyAreaResponse, error)
	FindNearestFunc           func(ctx context.Context, req *pb.FindNearestRequest) (*pb.FindNearestResponse, error)
	WhatCanICraftFunc         func(ctx context.Context) (*pb.WhatCanICraftResponse, error)
	AssessThreatFunc          func(ctx context.Context, radius int32) (*pb.AssessThreatResponse, error)
	PlanPathFunc              func(ctx context.Context, destination *pb.Vec3, allowDig bool) (*pb.PlanPathResponse, error)
	GetSurroundingsFunc       func(ctx context.Context, radius int32, fullUpdate bool) (*pb.GetSurroundingsResponse, error)
	GetInventoryFunc          func(ctx context.Context, includeCraftSuggestions bool) (*pb.GetInventoryResponse, error)
	TakeScreenshotFunc        func(ctx context.Context, res pb.ScreenshotResolution, lookAt *pb.Vec3) (*pb.TakeScreenshotResponse, error)
}

func (m *mockClient) MoveTo(ctx context.Context, target *pb.Vec3, rng float32, sprint bool) (*pb.MoveToResponse, error) {
	return m.MoveToFunc(ctx, target, rng, sprint)
}

func (m *mockClient) LookAt(ctx context.Context, target *pb.Vec3) (*pb.LookAtResponse, error) {
	return m.LookAtFunc(ctx, target)
}

func (m *mockClient) PlaceBlock(ctx context.Context, pos *pb.Vec3, face, blockName string) (*pb.PlaceBlockResponse, error) {
	return m.PlaceBlockFunc(ctx, pos, face, blockName)
}

func (m *mockClient) DigBlock(ctx context.Context, pos *pb.Vec3) (*pb.DigBlockResponse, error) {
	return m.DigBlockFunc(ctx, pos)
}

func (m *mockClient) EquipItem(ctx context.Context, itemName, destination string) (*pb.EquipItemResponse, error) {
	return m.EquipItemFunc(ctx, itemName, destination)
}

func (m *mockClient) UseItem(ctx context.Context) (*pb.UseItemResponse, error) {
	return m.UseItemFunc(ctx)
}

func (m *mockClient) AttackEntity(ctx context.Context, entityID int32) (*pb.AttackEntityResponse, error) {
	return m.AttackEntityFunc(ctx, entityID)
}

func (m *mockClient) Jump(ctx context.Context) (*pb.JumpResponse, error) {
	return m.JumpFunc(ctx)
}

func (m *mockClient) SetSneak(ctx context.Context, enabled bool) (*pb.SetSneakResponse, error) {
	return m.SetSneakFunc(ctx, enabled)
}

func (m *mockClient) SendChat(ctx context.Context, message string) error {
	return m.SendChatFunc(ctx, message)
}

func (m *mockClient) NavigateTo(ctx context.Context, req *pb.NavigateToRequest) (*pb.NavigateToResponse, error) {
	return m.NavigateToFunc(ctx, req)
}

func (m *mockClient) InteractWithEntity(ctx context.Context, req *pb.InteractWithEntityRequest) (*pb.InteractWithEntityResponse, error) {
	return m.InteractWithEntityFunc(ctx, req)
}

func (m *mockClient) OpenContainer(ctx context.Context, req *pb.OpenContainerRequest) (*pb.OpenContainerResponse, error) {
	return m.OpenContainerFunc(ctx, req)
}

func (m *mockClient) WithdrawFromContainer(ctx context.Context, req *pb.WithdrawFromContainerRequest) (*pb.WithdrawFromContainerResponse, error) {
	return m.WithdrawFromContainerFunc(ctx, req)
}

func (m *mockClient) DepositToContainer(ctx context.Context, req *pb.DepositToContainerRequest) (*pb.DepositToContainerResponse, error) {
	return m.DepositToContainerFunc(ctx, req)
}

func (m *mockClient) CraftItem(ctx context.Context, itemName string, count int32) (*pb.CraftItemResponse, error) {
	return m.CraftItemFunc(ctx, itemName, count)
}

func (m *mockClient) SmeltItem(ctx context.Context, itemName string, count int32, fuel string) (*pb.SmeltItemResponse, error) {
	return m.SmeltItemFunc(ctx, itemName, count, fuel)
}

func (m *mockClient) Eat(ctx context.Context, foodName string) (*pb.EatResponse, error) {
	return m.EatFunc(ctx, foodName)
}

func (m *mockClient) HarvestBlock(ctx context.Context, req *pb.HarvestBlockRequest) (<-chan sidecar.TaskEvent, error) {
	return m.HarvestBlockFunc(ctx, req)
}

func (m *mockClient) Gather(ctx context.Context, req *pb.GatherRequest) (<-chan sidecar.TaskEvent, error) {
	return m.GatherFunc(ctx, req)
}

func (m *mockClient) BuildStructure(ctx context.Context, req *pb.BuildStructureRequest) (<-chan sidecar.TaskEvent, error) {
	return m.BuildStructureFunc(ctx, req)
}

func (m *mockClient) ProcessAll(ctx context.Context, req *pb.ProcessAllRequest) (<-chan sidecar.TaskEvent, error) {
	return m.ProcessAllFunc(ctx, req)
}

func (m *mockClient) OrganizeInventory(ctx context.Context, req *pb.OrganizeInventoryRequest) (<-chan sidecar.TaskEvent, error) {
	return m.OrganizeInventoryFunc(ctx, req)
}

func (m *mockClient) ClearArea(ctx context.Context, req *pb.ClearAreaRequest) (<-chan sidecar.TaskEvent, error) {
	return m.ClearAreaFunc(ctx, req)
}

func (m *mockClient) Farm(ctx context.Context, req *pb.FarmRequest) (<-chan sidecar.TaskEvent, error) {
	return m.FarmFunc(ctx, req)
}

func (m *mockClient) SurveyArea(ctx context.Context, radius int32) (*pb.SurveyAreaResponse, error) {
	return m.SurveyAreaFunc(ctx, radius)
}

func (m *mockClient) FindNearest(ctx context.Context, req *pb.FindNearestRequest) (*pb.FindNearestResponse, error) {
	return m.FindNearestFunc(ctx, req)
}

func (m *mockClient) WhatCanICraft(ctx context.Context) (*pb.WhatCanICraftResponse, error) {
	return m.WhatCanICraftFunc(ctx)
}

func (m *mockClient) AssessThreat(ctx context.Context, radius int32) (*pb.AssessThreatResponse, error) {
	return m.AssessThreatFunc(ctx, radius)
}

func (m *mockClient) PlanPath(ctx context.Context, destination *pb.Vec3, allowDig bool) (*pb.PlanPathResponse, error) {
	return m.PlanPathFunc(ctx, destination, allowDig)
}

func (m *mockClient) GetSurroundings(ctx context.Context, radius int32, fullUpdate bool) (*pb.GetSurroundingsResponse, error) {
	return m.GetSurroundingsFunc(ctx, radius, fullUpdate)
}

func (m *mockClient) GetInventory(ctx context.Context, includeCraftSuggestions bool) (*pb.GetInventoryResponse, error) {
	return m.GetInventoryFunc(ctx, includeCraftSuggestions)
}

func (m *mockClient) TakeScreenshot(ctx context.Context, res pb.ScreenshotResolution, lookAt *pb.Vec3) (*pb.TakeScreenshotResponse, error) {
	return m.TakeScreenshotFunc(ctx, res, lookAt)
}

func newTestDispatcherWithClient(t *testing.T, mc *mockClient) *Dispatcher {
	t.Helper()
	dir := t.TempDir()
	mem, err := memory.New(dir)
	if err != nil {
		t.Fatalf("memory.New: %v", err)
	}
	st := &fakeState{verbosity: perception.VerbosityStandard}
	return NewDispatcher("testbot", mc, mem, st, nil, nil, logging.NewDefaultLogger())
}

func newTestDispatcherWithClientAndTasks(t *testing.T, mc *mockClient) *Dispatcher {
	t.Helper()
	dir := t.TempDir()
	mem, err := memory.New(dir)
	if err != nil {
		t.Fatalf("memory.New: %v", err)
	}
	st := &fakeState{verbosity: perception.VerbosityStandard}
	tm := task.NewManager(context.Background(), func(*pb.GameEvent) {}, func() {}, 10*time.Minute, logging.NewDefaultLogger())
	return NewDispatcher("testbot", mc, mem, st, tm, nil, logging.NewDefaultLogger())
}

func okResult(msg string) *pb.ActionResult {
	return &pb.ActionResult{Success: true, Message: msg}
}

func failResult(errMsg string) *pb.ActionResult {
	return &pb.ActionResult{Success: false, Error: errMsg}
}
