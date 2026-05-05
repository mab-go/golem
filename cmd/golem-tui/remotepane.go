package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	sidecar "github.com/mab-go/golem/internal/grpc"
	"github.com/mab-go/golem/internal/grpc/pb"
)

type remoteTickMsg struct{}

type commandFunc func(ctx context.Context, args []string) (string, error)

type remotePane struct {
	scrollableViewport
	client   *sidecar.Client
	ring     *ringBuffer
	hud      string
	commands map[string]commandFunc
}

func newRemotePane() *remotePane {
	p := &remotePane{
		scrollableViewport: newScrollableViewport(),
		ring:               newRingBuffer(2000),
	}
	p.commands = p.buildCommandTable()
	return p
}

var _ Pane = (*remotePane)(nil)

func (p *remotePane) SetClient(c *sidecar.Client) tea.Cmd {
	p.client = c
	return p.tickCmd()
}

func (p *remotePane) Init() tea.Cmd { return nil }

func (p *remotePane) Update(msg tea.Msg) (Pane, tea.Cmd) {
	switch msg := msg.(type) {
	case RemoteResultMsg:
		p.appendResult(msg)
		return p, nil

	case remoteTickMsg:
		p.refreshHUD()
		return p, p.tickCmd()

	case tea.KeyPressMsg:
		if p.handleScrollKey(msg) {
			return p, nil
		}
	}

	return p, p.delegateViewport(msg)
}

func (p *remotePane) View() string {
	if p.dirty {
		p.rebuildFromLines(p.ring.Lines(), 0)
	}
	hudHeight := strings.Count(p.hud, "\n") + 1
	logHeight := max(0, p.height-hudHeight-2)

	logViewport := p.viewport.View()
	innerW := p.contentWidth()
	hudRendered := truncateToWidth(p.hud, innerW)

	content := hudRendered + "\n" + strings.Repeat("-", min(innerW, 40)) + "\n" + logViewport
	_ = logHeight
	return renderBorderedPane(content, p.Title(), p.width, p.height, p.focused)
}

func (p *remotePane) Title() string     { return "Remote" }
func (p *remotePane) Focused() bool     { return p.focused }
func (p *remotePane) SetFocused(f bool) { p.focused = f }

func (p *remotePane) SetSize(w, h int) {
	p.width = w
	p.height = h
	hudHeight := strings.Count(p.hud, "\n") + 1
	vpHeight := max(0, h-hudHeight-4)
	p.viewport.SetWidth(max(0, w-2))
	p.viewport.SetHeight(vpHeight)
	p.dirty = true
}

func (p *remotePane) Execute(command string) tea.Cmd {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil
	}

	p.ring.Push(dimStyle.Render("> " + command))
	p.dirty = true

	name := strings.ToLower(parts[0])
	args := parts[1:]

	handler, ok := p.commands[name]
	if !ok {
		return func() tea.Msg {
			return RemoteResultMsg{
				Command: command,
				Err:     fmt.Errorf("unknown command: %s (try 'help')", name),
			}
		}
	}

	if p.client == nil {
		return func() tea.Msg {
			return RemoteResultMsg{
				Command: command,
				Err:     fmt.Errorf("sidecar not connected"),
			}
		}
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := handler(ctx, args)
		return RemoteResultMsg{Command: command, Output: out, Err: err}
	}
}

func (p *remotePane) appendResult(msg RemoteResultMsg) {
	if msg.Err != nil {
		p.ring.Push(statusDownStyle.Render("ERR: " + msg.Err.Error()))
	} else if msg.Output != "" {
		for _, line := range strings.Split(msg.Output, "\n") {
			p.ring.Push(line)
		}
	}
	p.dirty = true
}

func (p *remotePane) refreshHUD() {
	if p.client == nil {
		p.hud = dimStyle.Render("Waiting for sidecar...")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	vitals, vErr := p.client.GetVitalSigns(ctx)
	surr, sErr := p.client.GetSurroundings(ctx, 8, false)

	var lines []string
	if vErr != nil {
		lines = append(lines, dimStyle.Render("HUD: "+vErr.Error()))
	} else {
		hp := fmt.Sprintf("HP: %.0f/%.0f", vitals.Health, vitals.MaxHealth)
		food := fmt.Sprintf("Food: %d/20", vitals.Food)
		armor := fmt.Sprintf("Armor: %d", vitals.Armor)
		xp := fmt.Sprintf("XP: %d (%.0f%%)", vitals.XpLevel, vitals.XpProgress*100)
		lines = append(lines, fmt.Sprintf("%s  %s  %s  %s", hp, food, armor, xp))
		hand := "empty"
		if vitals.MainHand != nil && vitals.MainHand.Name != "" {
			hand = vitals.MainHand.Name
		}
		lines = append(lines, fmt.Sprintf("Hand: %s  Mode: %s", hand, vitals.GameMode))
	}
	if sErr != nil {
		lines = append(lines, dimStyle.Render("Pos: "+sErr.Error()))
	} else {
		pos := surr.Position
		lines = append(lines, fmt.Sprintf("Pos: %.1f %.1f %.1f  Yaw: %.0f",
			pos.X, pos.Y, pos.Z, surr.Yaw))
		lines = append(lines, fmt.Sprintf("Biome: %s  Time: %s  Light: %d",
			surr.Biome, surr.TimeOfDay, surr.LightLevel))
		weather := ""
		if surr.IsThundering {
			weather = "  [thunder]"
		} else if surr.IsRaining {
			weather = "  [rain]"
		}
		if weather != "" {
			lines[len(lines)-1] += weather
		}
	}
	p.hud = strings.Join(lines, "\n")
}

func (p *remotePane) tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return remoteTickMsg{}
	})
}

func (p *remotePane) buildCommandTable() map[string]commandFunc {
	return map[string]commandFunc{
		"nav":      p.cmdNav,
		"move":     p.cmdMove,
		"look":     p.cmdLook,
		"jump":     p.cmdJump,
		"sneak":    p.cmdSneak,
		"status":   p.cmdStatus,
		"inv":      p.cmdInventory,
		"find":     p.cmdFind,
		"threat":   p.cmdThreat,
		"craft?":   p.cmdWhatCanICraft,
		"survey":   p.cmdSurvey,
		"eat":      p.cmdEat,
		"equip":    p.cmdEquip,
		"use":      p.cmdUse,
		"dig":      p.cmdDig,
		"place":    p.cmdPlace,
		"attack":   p.cmdAttack,
		"interact": p.cmdInteract,
		"craft":    p.cmdCraft,
		"chat":     p.cmdChat,
		"help":     p.cmdHelp,
	}
}

func (p *remotePane) cmdNav(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: nav <x> <y> <z> [dig] | nav <name> [dig] | nav block:<type> [dig]")
	}
	allowDig := len(args) > 0 && args[len(args)-1] == "dig"
	if allowDig {
		args = args[:len(args)-1]
	}
	if len(args) == 0 {
		return "", fmt.Errorf("usage: nav <x> <y> <z> [dig] | nav <name> [dig] | nav block:<type> [dig]")
	}
	req := &pb.NavigateToRequest{AllowDig: allowDig}
	if len(args) >= 3 {
		v, err := parseVec3(args[:3])
		if err == nil {
			req.Target = &pb.NavigateToRequest_Position{Position: v}
			resp, err := p.client.NavigateTo(ctx, req)
			if err != nil {
				return "", err
			}
			return formatNavResult(resp), nil
		}
	}
	target := strings.Join(args, " ")
	if after, ok := strings.CutPrefix(target, "block:"); ok {
		req.Target = &pb.NavigateToRequest_BlockType{BlockType: after}
	} else {
		req.Target = &pb.NavigateToRequest_EntityName{EntityName: target}
	}
	resp, err := p.client.NavigateTo(ctx, req)
	if err != nil {
		return "", err
	}
	return formatNavResult(resp), nil
}

func (p *remotePane) cmdMove(ctx context.Context, args []string) (string, error) {
	if len(args) < 3 {
		return "", fmt.Errorf("usage: move <x> <y> <z>")
	}
	v, err := parseVec3(args[:3])
	if err != nil {
		return "", err
	}
	resp, err := p.client.MoveTo(ctx, v, 1, false)
	if err != nil {
		return "", err
	}
	r := resp.Result
	if !r.Success {
		return "FAILED: " + r.Error, nil
	}
	return fmt.Sprintf("OK: %s (remaining: %.1f)", r.Message, resp.DistanceRemaining), nil
}

func (p *remotePane) cmdLook(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return p.cmdSurroundings(ctx)
	}
	if len(args) < 3 {
		return "", fmt.Errorf("usage: look <x> <y> <z> (or 'look' with no args for surroundings)")
	}
	v, err := parseVec3(args[:3])
	if err != nil {
		return "", err
	}
	resp, err := p.client.LookAt(ctx, v)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return "FAILED: " + resp.Result.Error, nil
	}
	return "OK: looking at target", nil
}

func (p *remotePane) cmdJump(ctx context.Context, _ []string) (string, error) {
	resp, err := p.client.Jump(ctx)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return "FAILED: " + resp.Result.Error, nil
	}
	return "OK: jumped", nil
}

func (p *remotePane) cmdSneak(ctx context.Context, args []string) (string, error) {
	enabled := len(args) == 0 || (args[0] != "off" && args[0] != "false")
	resp, err := p.client.SetSneak(ctx, enabled)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return "FAILED: " + resp.Result.Error, nil
	}
	if enabled {
		return "OK: sneaking", nil
	}
	return "OK: stopped sneaking", nil
}

func (p *remotePane) cmdStatus(ctx context.Context, _ []string) (string, error) {
	resp, err := p.client.GetVitalSigns(ctx)
	if err != nil {
		return "", err
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("Health: %.1f/%.1f", resp.Health, resp.MaxHealth))
	lines = append(lines, fmt.Sprintf("Food: %d/20  Saturation: %.1f", resp.Food, resp.FoodSaturation))
	lines = append(lines, fmt.Sprintf("Armor: %d  XP: %d (%.0f%%)", resp.Armor, resp.XpLevel, resp.XpProgress*100))
	lines = append(lines, fmt.Sprintf("Oxygen: %d/300  Mode: %s", resp.Oxygen, resp.GameMode))
	if resp.MainHand != nil && resp.MainHand.Name != "" {
		lines = append(lines, fmt.Sprintf("Main hand: %s x%d", resp.MainHand.Name, resp.MainHand.Count))
	}
	if resp.OffHand != nil && resp.OffHand.Name != "" {
		lines = append(lines, fmt.Sprintf("Off hand: %s x%d", resp.OffHand.Name, resp.OffHand.Count))
	}
	for _, e := range resp.Effects {
		lines = append(lines, fmt.Sprintf("Effect: %s (lvl %d, %ds)", e.Name, e.Amplifier+1, e.DurationTicks/20))
	}
	return strings.Join(lines, "\n"), nil
}

func (p *remotePane) cmdSurroundings(ctx context.Context) (string, error) {
	resp, err := p.client.GetSurroundings(ctx, 16, false)
	if err != nil {
		return "", err
	}
	var lines []string
	pos := resp.Position
	lines = append(lines, fmt.Sprintf("Position: %.1f %.1f %.1f", pos.X, pos.Y, pos.Z))
	lines = append(lines, fmt.Sprintf("Biome: %s  Dimension: %s", resp.Biome, resp.Dimension))
	lines = append(lines, fmt.Sprintf("Time: %s  Light: %d", resp.TimeOfDay, resp.LightLevel))
	if len(resp.NearbyEntities) > 0 {
		lines = append(lines, fmt.Sprintf("Entities (%d):", len(resp.NearbyEntities)))
		limit := min(10, len(resp.NearbyEntities))
		for _, e := range resp.NearbyEntities[:limit] {
			lines = append(lines, fmt.Sprintf("  %s (%.1fm)", e.Name, e.Distance))
		}
		if len(resp.NearbyEntities) > 10 {
			lines = append(lines, fmt.Sprintf("  ... and %d more", len(resp.NearbyEntities)-10))
		}
	}
	if len(resp.NotableBlocks) > 0 {
		lines = append(lines, fmt.Sprintf("Blocks (%d):", len(resp.NotableBlocks)))
		limit := min(10, len(resp.NotableBlocks))
		for _, b := range resp.NotableBlocks[:limit] {
			lines = append(lines, fmt.Sprintf("  %s at %.0f,%.0f,%.0f (%.1fm)",
				b.Type, b.Position.X, b.Position.Y, b.Position.Z, b.Distance))
		}
	}
	return strings.Join(lines, "\n"), nil
}

func (p *remotePane) cmdInventory(ctx context.Context, _ []string) (string, error) {
	resp, err := p.client.GetInventory(ctx, false)
	if err != nil {
		return "", err
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("Inventory (%d empty slots):", resp.EmptySlots))
	for _, item := range resp.Items {
		lines = append(lines, fmt.Sprintf("  [%d] %s x%d", item.Slot, item.Name, item.Count))
	}
	if len(resp.ArmorItems) > 0 {
		lines = append(lines, "Armor:")
		for _, item := range resp.ArmorItems {
			lines = append(lines, fmt.Sprintf("  %s", item.Name))
		}
	}
	return strings.Join(lines, "\n"), nil
}

func (p *remotePane) cmdFind(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: find <block_type|entity_type>")
	}
	target := strings.Join(args, "_")
	req := &pb.FindNearestRequest{Radius: 64}
	if strings.Contains(target, "_") || isBlockName(target) {
		req.Target = &pb.FindNearestRequest_BlockType{BlockType: target}
	} else {
		req.Target = &pb.FindNearestRequest_EntityType{EntityType: target}
	}
	resp, err := p.client.FindNearest(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Found {
		return "Not found within radius", nil
	}
	return fmt.Sprintf("Found %s at %.0f,%.0f,%.0f (%.1fm away)",
		resp.Type, resp.Position.X, resp.Position.Y, resp.Position.Z, resp.Distance), nil
}

func (p *remotePane) cmdThreat(ctx context.Context, _ []string) (string, error) {
	resp, err := p.client.AssessThreat(ctx, 32)
	if err != nil {
		return "", err
	}
	return resp.Summary, nil
}

func (p *remotePane) cmdWhatCanICraft(ctx context.Context, _ []string) (string, error) {
	resp, err := p.client.WhatCanICraft(ctx)
	if err != nil {
		return "", err
	}
	if len(resp.Items) == 0 {
		return "Nothing craftable with current inventory", nil
	}
	var lines []string
	for _, item := range resp.Items {
		table := ""
		if item.RequiresCraftingTable {
			table = " [table]"
		}
		lines = append(lines, fmt.Sprintf("  %s x%d%s", item.ItemName, item.MaxCount, table))
	}
	return "Craftable:\n" + strings.Join(lines, "\n"), nil
}

func (p *remotePane) cmdSurvey(ctx context.Context, args []string) (string, error) {
	radius := int32(32)
	if len(args) > 0 {
		r, err := strconv.Atoi(args[0])
		if err == nil && r > 0 {
			radius = int32(r)
		}
	}
	resp, err := p.client.SurveyArea(ctx, radius)
	if err != nil {
		return "", err
	}
	return resp.Summary, nil
}

func (p *remotePane) cmdEat(ctx context.Context, args []string) (string, error) {
	food := ""
	if len(args) > 0 {
		food = strings.Join(args, "_")
	}
	resp, err := p.client.Eat(ctx, food)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return "FAILED: " + resp.Result.Error, nil
	}
	return "OK: " + resp.Result.Message, nil
}

func (p *remotePane) cmdEquip(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: equip <item> [hand|off-hand|head|torso|legs|feet]")
	}
	dest := "hand"
	if len(args) > 1 {
		dest = args[len(args)-1]
		args = args[:len(args)-1]
	}
	item := strings.Join(args, "_")
	resp, err := p.client.EquipItem(ctx, item, dest)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return "FAILED: " + resp.Result.Error, nil
	}
	return "OK: " + resp.Result.Message, nil
}

func (p *remotePane) cmdUse(ctx context.Context, _ []string) (string, error) {
	resp, err := p.client.UseItem(ctx)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return "FAILED: " + resp.Result.Error, nil
	}
	return "OK: " + resp.Result.Message, nil
}

func (p *remotePane) cmdDig(ctx context.Context, args []string) (string, error) {
	if len(args) < 3 {
		return "", fmt.Errorf("usage: dig <x> <y> <z>")
	}
	v, err := parseVec3(args[:3])
	if err != nil {
		return "", err
	}
	resp, err := p.client.DigBlock(ctx, v)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return "FAILED: " + resp.Result.Error, nil
	}
	drops := ""
	if len(resp.Drops) > 0 {
		var names []string
		for _, d := range resp.Drops {
			names = append(names, fmt.Sprintf("%s x%d", d.Name, d.Count))
		}
		drops = " -> " + strings.Join(names, ", ")
	}
	return "OK: " + resp.Result.Message + drops, nil
}

func (p *remotePane) cmdPlace(ctx context.Context, args []string) (string, error) {
	if len(args) < 4 {
		return "", fmt.Errorf("usage: place <block> <x> <y> <z> [face]")
	}
	block := args[0]
	v, err := parseVec3(args[1:4])
	if err != nil {
		return "", err
	}
	face := "top"
	if len(args) > 4 {
		face = args[4]
	}
	resp, err := p.client.PlaceBlock(ctx, v, face, block)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return "FAILED: " + resp.Result.Error, nil
	}
	return "OK: " + resp.Result.Message, nil
}

func (p *remotePane) cmdAttack(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: attack <entity_id>")
	}
	id, err := strconv.Atoi(args[0])
	if err != nil {
		return "", fmt.Errorf("entity_id must be a number: %w", err)
	}
	resp, err := p.client.AttackEntity(ctx, int32(id))
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return "FAILED: " + resp.Result.Error, nil
	}
	return "OK: " + resp.Result.Message, nil
}

func (p *remotePane) cmdInteract(ctx context.Context, args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: interact <entity> <harvest|attack|feed|trade|mount|shear>")
	}
	action := strings.ToLower(args[len(args)-1])
	entity := strings.Join(args[:len(args)-1], " ")
	actionMap := map[string]pb.InteractionType{
		"harvest": pb.InteractionType_INTERACTION_HARVEST,
		"attack":  pb.InteractionType_INTERACTION_ATTACK,
		"feed":    pb.InteractionType_INTERACTION_FEED,
		"trade":   pb.InteractionType_INTERACTION_TRADE,
		"mount":   pb.InteractionType_INTERACTION_MOUNT,
		"shear":   pb.InteractionType_INTERACTION_SHEAR,
	}
	iType, ok := actionMap[action]
	if !ok {
		return "", fmt.Errorf("unknown action %q (want: harvest, attack, feed, trade, mount, shear)", action)
	}
	req := &pb.InteractWithEntityRequest{
		EntityName: entity,
		Action:     iType,
	}
	resp, err := p.client.InteractWithEntity(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return "FAILED: " + resp.Result.Error, nil
	}
	return "OK: " + resp.Description, nil
}

func (p *remotePane) cmdCraft(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: craft <item> [count]")
	}
	count := int32(1)
	item := args[0]
	if len(args) > 1 {
		n, err := strconv.Atoi(args[len(args)-1])
		if err == nil {
			count = int32(n)
			item = strings.Join(args[:len(args)-1], "_")
		} else {
			item = strings.Join(args, "_")
		}
	}
	resp, err := p.client.CraftItem(ctx, item, count)
	if err != nil {
		return "", err
	}
	if !resp.Result.Success {
		return "FAILED: " + resp.Result.Error, nil
	}
	return "OK: " + resp.Result.Message, nil
}

func (p *remotePane) cmdChat(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: chat <message>")
	}
	message := strings.Join(args, " ")
	err := p.client.SendChat(ctx, message)
	if err != nil {
		return "", err
	}
	return "OK: sent", nil
}

func (p *remotePane) cmdHelp(_ context.Context, _ []string) (string, error) {
	return "Navigation:\n" +
		"  nav <x> <y> <z> [dig]  Navigate to position\n" +
		"  nav <name> [dig]       Navigate to entity\n" +
		"  nav block:<type> [dig] Navigate to block type\n" +
		"  move <x> <y> <z>    Raw pathfinder move\n" +
		"  look <x> <y> <z>    Look at position\n" +
		"  jump                 Jump\n" +
		"  sneak [off]          Toggle sneak\n" +
		"Info:\n" +
		"  status               Vital signs (detailed)\n" +
		"  look                 Surroundings\n" +
		"  inv                  Inventory\n" +
		"  find <type>          Find nearest block/entity\n" +
		"  threat               Threat assessment\n" +
		"  craft?               What can I craft?\n" +
		"  survey [radius]      Survey area\n" +
		"Actions:\n" +
		"  eat [food]           Eat food\n" +
		"  equip <item> [slot]  Equip item\n" +
		"  use                  Use held item\n" +
		"  dig <x> <y> <z>     Dig block\n" +
		"  place <b> <x><y><z> Place block\n" +
		"  attack <id>          Attack entity\n" +
		"  interact <e> <act>   Interact with entity\n" +
		"  craft <item> [n]     Craft item\n" +
		"  chat <message>       Send chat", nil
}

func parseVec3(args []string) (*pb.Vec3, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("need 3 coordinates")
	}
	x, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid x: %w", err)
	}
	y, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid y: %w", err)
	}
	z, err := strconv.ParseFloat(args[2], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid z: %w", err)
	}
	return &pb.Vec3{X: x, Y: y, Z: z}, nil
}

func formatNavResult(resp *pb.NavigateToResponse) string {
	if !resp.Result.Success {
		return "FAILED: " + resp.Result.Error
	}
	pos := resp.FinalPosition
	return fmt.Sprintf("OK: arrived at %.1f,%.1f,%.1f (traveled %.1f blocks)",
		pos.X, pos.Y, pos.Z, resp.DistanceTraveled)
}

func isBlockName(s string) bool {
	blocks := []string{"stone", "dirt", "grass", "sand", "gravel", "wood", "log",
		"leaves", "chest", "furnace", "crafting", "iron", "gold", "diamond",
		"coal", "redstone", "lapis", "emerald", "obsidian", "water", "lava"}
	lower := strings.ToLower(s)
	for _, b := range blocks {
		if strings.Contains(lower, b) {
			return true
		}
	}
	return false
}

func truncateToWidth(s string, w int) string {
	if w <= 0 {
		return ""
	}
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if len(line) > w {
			line = line[:w]
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
