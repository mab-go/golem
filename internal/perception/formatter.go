package perception

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mab-go/golem/internal/grpc/pb"
)

// Format selects the perception rendering style.
//
// Prose produces text-adventure room descriptions. Structured produces compact
// key-value sections. The toggle exists so Phase 5.5 can A/B test which form
// Claude reasons with more reliably.
type Format int

const (
	// FormatProse renders natural-language descriptions.
	FormatProse Format = iota
	// FormatStructured renders compact key-value sections.
	FormatStructured
)

// ParseFormat converts a CLI/env string to a Format.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "prose":
		return FormatProse, nil
	case "structured":
		return FormatStructured, nil
	default:
		return FormatProse, fmt.Errorf("unknown perception format %q (want prose|structured)", s)
	}
}

// VerbosityMode controls how many details prose formatting emits. Structured
// format ignores verbosity.
type VerbosityMode int

const (
	// VerbosityStandard is the default -- concise but specific.
	VerbosityStandard VerbosityMode = iota
	// VerbosityVerbose emits flowing prose with atmosphere.
	VerbosityVerbose
	// VerbosityTerse emits one-liners.
	VerbosityTerse
)

// ParseVerbosity converts a CLI/env string to a VerbosityMode.
func ParseVerbosity(s string) (VerbosityMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "standard":
		return VerbosityStandard, nil
	case "verbose":
		return VerbosityVerbose, nil
	case "terse":
		return VerbosityTerse, nil
	default:
		return VerbosityStandard, fmt.Errorf("unknown verbosity %q (want verbose|standard|terse)", s)
	}
}

// String returns the canonical name of the verbosity mode.
func (v VerbosityMode) String() string {
	switch v {
	case VerbosityVerbose:
		return "verbose"
	case VerbosityTerse:
		return "terse"
	default:
		return "standard"
	}
}

// Formatter renders gRPC perception payloads into text for Claude.
//
// It is safe for concurrent use after construction: the only mutable field is
// Verbosity, which callers may update via SetVerbosity through a mutex if they
// share an instance across goroutines. The agent owns one Formatter and serializes
// access via the pacing state mutex.
type Formatter struct {
	Format    Format
	Verbosity VerbosityMode
}

// NewFormatter constructs a Formatter with the given format and verbosity.
func NewFormatter(f Format, v VerbosityMode) *Formatter {
	return &Formatter{Format: f, Verbosity: v}
}

// SetVerbosity updates the verbosity mode.
func (f *Formatter) SetVerbosity(v VerbosityMode) {
	f.Verbosity = v
}

// TaskStatus is a snapshot of an active background task for perception rendering.
type TaskStatus struct {
	Name    string
	Current int32
	Total   int32
	Elapsed time.Duration
	Message string
}

// FormatActiveTask renders a TaskStatus for inclusion in the perception
// snapshot. Returns "" when ts is nil (no active task).
func (f *Formatter) FormatActiveTask(ts *TaskStatus) string {
	if ts == nil {
		return ""
	}
	elapsed := ts.Elapsed.Truncate(time.Second)
	var line string
	if ts.Total > 0 {
		line = fmt.Sprintf("[Active Task] %s -- %d/%d (%s)", ts.Name, ts.Current, ts.Total, elapsed)
	} else {
		line = fmt.Sprintf("[Active Task] %s (%s)", ts.Name, elapsed)
	}
	if ts.Message != "" {
		line += "\n  " + ts.Message
	}
	line += "\n  Use cancel_task to abort."
	return line
}

// FormatSurroundings dispatches on the configured Format.
func (f *Formatter) FormatSurroundings(s *pb.GetSurroundingsResponse) string {
	if s == nil {
		return "(no surroundings data)"
	}
	if f.Format == FormatStructured {
		return formatSurroundingsStructured(s)
	}
	return formatSurroundingsProse(s, f.Verbosity)
}

// FormatVitalSigns renders a compact one-liner. Always the same format -- the
// HUD is token-efficient already.
func (f *Formatter) FormatVitalSigns(v *pb.GetVitalSignsResponse) string {
	if v == nil {
		return "(no vitals)"
	}
	parts := []string{
		fmt.Sprintf("HP %g/%g", v.Health, v.MaxHealth),
		fmt.Sprintf("food %d/20", v.Food),
		fmt.Sprintf("armor %d", v.Armor),
		fmt.Sprintf("xp %d (%.0f%%)", v.XpLevel, v.XpProgress*100),
	}
	if v.MainHand != nil && v.MainHand.Name != "" {
		parts = append(parts, "hand "+v.MainHand.Name)
	}
	if v.OffHand != nil && v.OffHand.Name != "" {
		parts = append(parts, "off "+v.OffHand.Name)
	}
	if len(v.Effects) > 0 {
		names := make([]string, 0, len(v.Effects))
		for _, e := range v.Effects {
			names = append(names, e.Name)
		}
		parts = append(parts, "effects "+strings.Join(names, ","))
	} else {
		parts = append(parts, "no effects")
	}
	if v.GameMode != "" {
		parts = append(parts, "mode "+v.GameMode)
	}
	return strings.Join(parts, " | ")
}

// FormatInventory groups items by stem (dropping "_sword", "_pickaxe", etc. is
// too lossy -- we group by item name and flag low durability).
func (f *Formatter) FormatInventory(inv *pb.GetInventoryResponse) string {
	if inv == nil {
		return "(no inventory)"
	}
	var b strings.Builder
	if len(inv.Items) == 0 {
		b.WriteString("Inventory: empty")
	} else {
		fmt.Fprintf(&b, "Inventory: %s", summarizeItems(inv.Items))
	}
	fmt.Fprintf(&b, " | %d empty slots", inv.EmptySlots)
	if len(inv.ArmorItems) > 0 {
		armor := make([]string, 0, len(inv.ArmorItems))
		for _, a := range inv.ArmorItems {
			armor = append(armor, a.Name)
		}
		fmt.Fprintf(&b, " | armor: %s", strings.Join(armor, ", "))
	}
	if len(inv.CraftSuggestions) > 0 {
		sug := make([]string, 0, len(inv.CraftSuggestions))
		for _, c := range inv.CraftSuggestions {
			tag := ""
			if c.RequiresCraftingTable {
				tag = " (table)"
			}
			sug = append(sug, fmt.Sprintf("%sx%d%s", c.ItemName, c.MaxCount, tag))
		}
		fmt.Fprintf(&b, " | craftable: %s", strings.Join(sug, ", "))
	}
	return b.String()
}

// ClassifiedEvent is a GameEvent paired with the priority a classifier
// assigned to it. The Priority field may differ from Event.Priority when a
// Haiku-driven reclassification runs before context assembly. Reason is an
// optional short explanation from the classifier.
type ClassifiedEvent struct {
	Event    *pb.GameEvent
	Priority pb.EventPriority
	Reason   string
}

// FormatClassifiedEvents renders classified events with the highest priority
// first. Critical and high events each get their own line with the
// classifier's reason; normal events get a line without a reason; low-priority
// events are collapsed into a one-line count summary at the tail so routine
// noise does not dominate the user message.
func (f *Formatter) FormatClassifiedEvents(events []ClassifiedEvent) string {
	if len(events) == 0 {
		return "No new events."
	}

	byPriority := groupByPriority(events)
	sortBucketsByTime(byPriority)

	var b strings.Builder
	b.WriteString("Events:\n")

	writeGroup := func(p pb.EventPriority) {
		for _, ce := range byPriority[p] {
			if ce.Event == nil {
				continue
			}
			desc := ce.Event.Description
			if desc == "" {
				desc = ce.Event.Type.String()
			}
			prefix := fmt.Sprintf("[%s %s]",
				time.UnixMilli(ce.Event.Timestamp).UTC().Format("15:04:05"),
				ce.Priority.String())
			reason := ""
			if strings.TrimSpace(ce.Reason) != "" {
				reason = " -- " + ce.Reason
			}
			fmt.Fprintf(&b, "  %s %s%s\n", prefix, desc, reason)
		}
	}
	writeGroup(pb.EventPriority_EVENT_PRIORITY_CRITICAL)
	writeGroup(pb.EventPriority_EVENT_PRIORITY_HIGH)
	writeGroup(pb.EventPriority_EVENT_PRIORITY_NORMAL)

	b.WriteString(collapsedLowSummary(byPriority[pb.EventPriority_EVENT_PRIORITY_LOW]))

	return strings.TrimRight(b.String(), "\n")
}

// FormatEvents emits chronological events with light deduplication. Repeated
// spawns of the same entity type are collapsed into a count.
func (f *Formatter) FormatEvents(events []*pb.GameEvent) string {
	if len(events) == 0 {
		return "No new events."
	}

	// Sort oldest-first for a chronological read.
	ordered := make([]*pb.GameEvent, len(events))
	copy(ordered, events)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Timestamp < ordered[j].Timestamp
	})

	var b strings.Builder
	b.WriteString("Events:\n")

	// Collapse repeated spawn events: track the previous key + count.
	type pending struct {
		event *pb.GameEvent
		count int
	}
	flush := func(p *pending) {
		if p.event == nil {
			return
		}
		prefix := formatEventPrefix(p.event)
		desc := p.event.Description
		if desc == "" {
			desc = p.event.Type.String()
		}
		if p.count > 1 {
			fmt.Fprintf(&b, "  %s %s (x%d)\n", prefix, desc, p.count)
		} else {
			fmt.Fprintf(&b, "  %s %s\n", prefix, desc)
		}
	}

	var cur pending
	for _, e := range ordered {
		key := dedupKey(e)
		curKey := dedupKey(cur.event)
		if cur.event != nil && key == curKey {
			cur.count++
			continue
		}
		flush(&cur)
		cur = pending{event: e, count: 1}
	}
	flush(&cur)

	return strings.TrimRight(b.String(), "\n")
}

func dedupKey(e *pb.GameEvent) string {
	if e == nil {
		return ""
	}
	entityName := ""
	if e.Entity != nil {
		entityName = e.Entity.Type
	}
	return fmt.Sprintf("%d|%s|%s", e.Type, entityName, e.PlayerName)
}

func formatEventPrefix(e *pb.GameEvent) string {
	ts := time.UnixMilli(e.Timestamp).UTC().Format("15:04:05")
	return fmt.Sprintf("[%s %s]", ts, e.Priority.String())
}

// --- Prose surroundings ------------------------------------------------------

func formatSurroundingsProse(s *pb.GetSurroundingsResponse, v VerbosityMode) string {
	biome := strings.ReplaceAll(s.Biome, "_", " ")
	if biome == "" {
		biome = "unknown terrain"
	}
	timeOfDay := s.TimeOfDay
	if timeOfDay == "" {
		timeOfDay = "an indeterminate time"
	}
	weather := weatherClause(s)

	switch v {
	case VerbosityTerse:
		return terseSurroundings(biome, timeOfDay, weather, s)
	case VerbosityVerbose:
		return verboseSurroundings(biome, timeOfDay, weather, s)
	default:
		return standardSurroundings(biome, timeOfDay, weather, s)
	}
}

func weatherClause(s *pb.GetSurroundingsResponse) string {
	switch {
	case s.IsThundering:
		return "a thunderstorm"
	case s.IsRaining:
		return "rain"
	default:
		return "clear"
	}
}

func terseSurroundings(biome, timeOfDay, weather string, s *pb.GetSurroundingsResponse) string {
	return fmt.Sprintf("%s, %s, %s. %s. %s.",
		biome, timeOfDay, weather,
		fallbackIfEmpty(shortEntityList(s.NearbyEntities, 4), "no entities nearby"),
		fallbackIfEmpty(shortBlockList(s.NotableBlocks, 4), "nothing notable"))
}

func verboseSurroundings(biome, timeOfDay, weather string, s *pb.GetSurroundingsResponse) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You stand at roughly %s in a %s. It is %s and the weather is %s (light %d).",
		formatPosition(s.Position), biome, timeOfDay, weather, s.LightLevel)
	if len(s.NearbyEntities) > 0 {
		fmt.Fprintf(&b, " Nearby you notice %s.", longEntityList(s.NearbyEntities))
	} else {
		b.WriteString(" Nothing lives within sight.")
	}
	if len(s.NotableBlocks) > 0 {
		fmt.Fprintf(&b, " Around you: %s.", longBlockList(s.NotableBlocks))
	}
	if s.Dimension != "" && s.Dimension != "overworld" {
		fmt.Fprintf(&b, " Dimension: %s.", s.Dimension)
	}
	return b.String()
}

func standardSurroundings(biome, timeOfDay, weather string, s *pb.GetSurroundingsResponse) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s, %s, %s (light %d). Position %s.",
		biome, timeOfDay, weather, s.LightLevel, formatPosition(s.Position))
	if entitiesShort := shortEntityList(s.NearbyEntities, 4); entitiesShort != "" {
		fmt.Fprintf(&b, " Nearby: %s.", entitiesShort)
	}
	if blocksShort := shortBlockList(s.NotableBlocks, 4); blocksShort != "" {
		fmt.Fprintf(&b, " Notable: %s.", blocksShort)
	}
	if s.Dimension != "" && s.Dimension != "overworld" {
		fmt.Fprintf(&b, " Dimension: %s.", s.Dimension)
	}
	return b.String()
}

// --- Structured surroundings -------------------------------------------------

func formatSurroundingsStructured(s *pb.GetSurroundingsResponse) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[Position] %s | %s | %s | %s\n",
		formatPosition(s.Position), fallbackIfEmpty(s.Biome, "unknown"),
		fallbackIfEmpty(s.TimeOfDay, "unknown"), weatherLabel(s))
	fmt.Fprintf(&b, "[Light] %d", s.LightLevel)
	if s.Dimension != "" {
		fmt.Fprintf(&b, " | [Dimension] %s", s.Dimension)
	}
	b.WriteString("\n")
	if len(s.NearbyEntities) > 0 {
		fmt.Fprintf(&b, "[Entities] %s\n", structuredEntityList(s.NearbyEntities))
	} else {
		b.WriteString("[Entities] (none)\n")
	}
	if len(s.NotableBlocks) > 0 {
		fmt.Fprintf(&b, "[Blocks] %s", structuredBlockList(s.NotableBlocks))
	} else {
		b.WriteString("[Blocks] (none)")
	}
	return b.String()
}

// --- Helpers -----------------------------------------------------------------

func groupByPriority(events []ClassifiedEvent) map[pb.EventPriority][]ClassifiedEvent {
	m := map[pb.EventPriority][]ClassifiedEvent{}
	for _, ce := range events {
		m[ce.Priority] = append(m[ce.Priority], ce)
	}
	return m
}

func sortBucketsByTime(buckets map[pb.EventPriority][]ClassifiedEvent) {
	for p := range buckets {
		slice := buckets[p]
		sort.SliceStable(slice, func(i, j int) bool {
			if slice[i].Event == nil || slice[j].Event == nil {
				return false
			}
			return slice[i].Event.Timestamp < slice[j].Event.Timestamp
		})
		buckets[p] = slice
	}
}

func collapsedLowSummary(events []ClassifiedEvent) string {
	if len(events) == 0 {
		return ""
	}
	kinds := map[string]int{}
	order := []string{}
	for _, ce := range events {
		if ce.Event == nil {
			continue
		}
		k := ce.Event.Type.String()
		if _, seen := kinds[k]; !seen {
			order = append(order, k)
		}
		kinds[k]++
	}
	parts := make([]string, 0, len(order))
	for _, k := range order {
		parts = append(parts, fmt.Sprintf("%sx%d", k, kinds[k]))
	}
	return fmt.Sprintf("  [LOW] %d routine (%s)\n", len(events), strings.Join(parts, ", "))
}

func summarizeItems(items []*pb.InventoryItem) string {
	counts := map[string]int32{}
	order := []string{}
	durability := map[string]string{}
	for _, it := range items {
		if _, seen := counts[it.Name]; !seen {
			order = append(order, it.Name)
		}
		counts[it.Name] += it.Count
		if it.MaxDurability > 0 {
			pct := float64(it.CurrentDurability) / float64(it.MaxDurability)
			if pct < 0.2 {
				durability[it.Name] = fmt.Sprintf(" (durability %.0f%%!)", pct*100)
			}
		}
	}
	sort.Strings(order)
	parts := make([]string, 0, len(order))
	for _, n := range order {
		parts = append(parts, fmt.Sprintf("%sx%d%s", n, counts[n], durability[n]))
	}
	return strings.Join(parts, ", ")
}

func formatPosition(p *pb.Vec3) string {
	if p == nil {
		return "?, ?, ?"
	}
	return fmt.Sprintf("%.0f, %.0f, %.0f", p.X, p.Y, p.Z)
}

func weatherLabel(s *pb.GetSurroundingsResponse) string {
	switch {
	case s.IsThundering:
		return "thundering"
	case s.IsRaining:
		return "raining"
	default:
		return "clear"
	}
}

func fallbackIfEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func shortEntityList(entities []*pb.Entity, limit int) string {
	if len(entities) == 0 {
		return ""
	}
	counts := map[string]int{}
	order := []string{}
	for _, e := range entities {
		if _, ok := counts[e.Type]; !ok {
			order = append(order, e.Type)
		}
		counts[e.Type]++
	}
	sort.Strings(order)
	if len(order) > limit {
		order = order[:limit]
	}
	parts := make([]string, 0, len(order))
	for _, t := range order {
		parts = append(parts, fmt.Sprintf("%d %s", counts[t], t))
	}
	return strings.Join(parts, ", ")
}

func longEntityList(entities []*pb.Entity) string {
	parts := make([]string, 0, len(entities))
	for _, e := range entities {
		name := e.Type
		if e.Name != "" && e.Name != e.Type {
			name = e.Name + " (" + e.Type + ")"
		}
		parts = append(parts, fmt.Sprintf("%s about %.0f blocks away", name, e.Distance))
	}
	return strings.Join(parts, ", ")
}

func structuredEntityList(entities []*pb.Entity) string {
	type bucket struct {
		typ     string
		count   int
		nearest float64
	}
	m := map[string]*bucket{}
	order := []string{}
	for _, e := range entities {
		b, ok := m[e.Type]
		if !ok {
			b = &bucket{typ: e.Type, nearest: e.Distance}
			m[e.Type] = b
			order = append(order, e.Type)
		}
		b.count++
		if e.Distance < b.nearest {
			b.nearest = e.Distance
		}
	}
	sort.Strings(order)
	parts := make([]string, 0, len(order))
	for _, t := range order {
		b := m[t]
		parts = append(parts, fmt.Sprintf("%s x%d (nearest %.0f)", b.typ, b.count, b.nearest))
	}
	return strings.Join(parts, ", ")
}

func shortBlockList(blocks []*pb.Block, limit int) string {
	if len(blocks) == 0 {
		return ""
	}
	counts := map[string]int{}
	order := []string{}
	for _, blk := range blocks {
		if _, ok := counts[blk.Type]; !ok {
			order = append(order, blk.Type)
		}
		counts[blk.Type]++
	}
	sort.Strings(order)
	if len(order) > limit {
		order = order[:limit]
	}
	parts := make([]string, 0, len(order))
	for _, t := range order {
		parts = append(parts, fmt.Sprintf("%d %s", counts[t], t))
	}
	return strings.Join(parts, ", ")
}

func longBlockList(blocks []*pb.Block) string {
	parts := make([]string, 0, len(blocks))
	for _, blk := range blocks {
		parts = append(parts, fmt.Sprintf("%s at %.0f blocks", blk.Type, blk.Distance))
	}
	return strings.Join(parts, ", ")
}

func structuredBlockList(blocks []*pb.Block) string {
	type bucket struct {
		typ     string
		count   int
		nearest float64
	}
	m := map[string]*bucket{}
	order := []string{}
	for _, blk := range blocks {
		b, ok := m[blk.Type]
		if !ok {
			b = &bucket{typ: blk.Type, nearest: blk.Distance}
			m[blk.Type] = b
			order = append(order, blk.Type)
		}
		b.count++
		if blk.Distance < b.nearest {
			b.nearest = blk.Distance
		}
	}
	sort.Strings(order)
	parts := make([]string, 0, len(order))
	for _, t := range order {
		b := m[t]
		parts = append(parts, fmt.Sprintf("%s x%d (nearest %.0f)", b.typ, b.count, b.nearest))
	}
	return strings.Join(parts, ", ")
}
