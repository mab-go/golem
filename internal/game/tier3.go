package game

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mab-go/golem/internal/grpc/pb"
)

const maxSurveyRadius int32 = 128

func (d *Dispatcher) handleSurveyArea(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Radius int32 `json:"radius"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.Radius <= 0 {
		in.Radius = 32
	}
	if in.Radius > maxSurveyRadius {
		in.Radius = maxSurveyRadius
	}
	resp, err := d.client.SurveyArea(ctx, in.Radius)
	if err != nil {
		return "", err
	}
	return formatSurveyAreaResult(resp), nil
}

func formatSurveyAreaResult(resp *pb.SurveyAreaResponse) string {
	if resp == nil {
		return "Survey returned no data."
	}
	if resp.Summary != "" {
		return resp.Summary
	}
	parts := []string{
		fmt.Sprintf("%d block clusters", len(resp.BlockClusters)),
		fmt.Sprintf("%d entities", len(resp.Entities)),
		fmt.Sprintf("%d structures", len(resp.Structures)),
	}
	return "Surveyed area: " + strings.Join(parts, ", ")
}

func (d *Dispatcher) handleFindNearest(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		BlockType     string `json:"block_type"`
		EntityType    string `json:"entity_type"`
		StructureType string `json:"structure_type"`
		Radius        int32  `json:"radius"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.Radius <= 0 {
		in.Radius = 64
	}
	if in.Radius > maxSurveyRadius {
		in.Radius = maxSurveyRadius
	}
	req := &pb.FindNearestRequest{Radius: in.Radius}
	switch {
	case in.BlockType != "":
		req.Target = &pb.FindNearestRequest_BlockType{BlockType: in.BlockType}
	case in.EntityType != "":
		req.Target = &pb.FindNearestRequest_EntityType{EntityType: in.EntityType}
	case in.StructureType != "":
		req.Target = &pb.FindNearestRequest_StructureType{StructureType: in.StructureType}
	default:
		return "find_nearest requires exactly one of block_type, entity_type, or structure_type.", nil
	}

	resp, err := d.client.FindNearest(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Found {
		return fmt.Sprintf("No %s found within %d blocks.", queryLabel(in), in.Radius), nil
	}
	pos := resp.Position
	if pos == nil {
		pos = &pb.Vec3{}
	}
	return fmt.Sprintf("Found %s at (%.1f, %.1f, %.1f), %.1fm away.",
		resp.Type, pos.X, pos.Y, pos.Z, resp.Distance), nil
}

func queryLabel(in struct {
	BlockType     string `json:"block_type"`
	EntityType    string `json:"entity_type"`
	StructureType string `json:"structure_type"`
	Radius        int32  `json:"radius"`
}) string {
	switch {
	case in.BlockType != "":
		return in.BlockType
	case in.EntityType != "":
		return in.EntityType
	case in.StructureType != "":
		return in.StructureType
	}
	return "target"
}

func (d *Dispatcher) handleWhatCanICraft(ctx context.Context, _ json.RawMessage) (string, error) {
	resp, err := d.client.WhatCanICraft(ctx)
	if err != nil {
		return "", err
	}
	if len(resp.Items) == 0 {
		return "Nothing can be crafted from current inventory.", nil
	}
	lines := make([]string, 0, len(resp.Items))
	for _, it := range resp.Items {
		marker := ""
		if it.RequiresCraftingTable {
			marker = " (table)"
		}
		lines = append(lines, fmt.Sprintf("%sx%d%s", it.ItemName, it.MaxCount, marker))
	}
	return "Craftable: " + strings.Join(lines, ", "), nil
}

func (d *Dispatcher) handleAssessThreat(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Radius int32 `json:"radius"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	if in.Radius <= 0 {
		in.Radius = 24
	}
	resp, err := d.client.AssessThreat(ctx, in.Radius)
	if err != nil {
		return "", err
	}
	if resp.Summary != "" {
		return resp.Summary, nil
	}
	return fmt.Sprintf("Threat: %s, %d hostiles, light %d, nightfall in %ds.",
		threatLevelLabel(resp.ThreatLevel), len(resp.HostileEntities),
		resp.LocalLightLevel, resp.TimeUntilNightfall), nil
}

func threatLevelLabel(lv pb.ThreatLevel) string {
	switch lv {
	case pb.ThreatLevel_THREAT_SAFE:
		return "safe"
	case pb.ThreatLevel_THREAT_LOW:
		return "low"
	case pb.ThreatLevel_THREAT_MODERATE:
		return "moderate"
	case pb.ThreatLevel_THREAT_HIGH:
		return "high"
	case pb.ThreatLevel_THREAT_CRITICAL:
		return "critical"
	}
	return "unknown"
}

func (d *Dispatcher) handlePlanPath(ctx context.Context, input json.RawMessage) (string, error) {
	var in struct {
		Destination vec3Input `json:"destination"`
		AllowDig    bool      `json:"allow_dig"`
	}
	if err := decode(input, &in); err != nil {
		return err.Error(), nil
	}
	resp, err := d.client.PlanPath(ctx, in.Destination.toPB(), in.AllowDig)
	if err != nil {
		return "", err
	}
	if !resp.Reachable {
		hazards := "none"
		if len(resp.Hazards) > 0 {
			hazards = strings.Join(resp.Hazards, ", ")
		}
		return fmt.Sprintf("Destination unreachable (hazards: %s).", hazards), nil
	}
	hazards := ""
	if len(resp.Hazards) > 0 {
		hazards = fmt.Sprintf(" hazards: %s;", strings.Join(resp.Hazards, ", "))
	}
	return fmt.Sprintf("Reachable: %.1fm,%s ~%ds, %d waypoints.",
		resp.Distance, hazards, resp.EstimatedSeconds, len(resp.Waypoints)), nil
}
