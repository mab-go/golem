package perception

import "strings"

// EntityCategory classifies a Minecraft entity for display and decision-making.
type EntityCategory int

// Entity categories used for display grouping in the gatekeeper snapshot.
const (
	CategoryPlayer EntityCategory = iota
	CategoryHostile
	CategoryNeutral
	CategoryPassive
)

func (c EntityCategory) String() string {
	switch c {
	case CategoryPlayer:
		return "Players"
	case CategoryHostile:
		return "Hostile"
	case CategoryNeutral:
		return "Neutral"
	case CategoryPassive:
		return "Passive"
	default:
		return "Unknown"
	}
}

var hostileTypes = map[string]struct{}{
	"zombie": {}, "zombie_villager": {}, "husk": {}, "drowned": {},
	"skeleton": {}, "stray": {}, "wither_skeleton": {}, "bogged": {},
	"creeper": {}, "spider": {}, "cave_spider": {},
	"enderman": {}, "endermite": {},
	"witch": {}, "slime": {}, "magma_cube": {},
	"blaze": {}, "ghast": {}, "wither": {},
	"phantom": {}, "shulker": {},
	"pillager": {}, "vindicator": {}, "evoker": {}, "ravager": {}, "vex": {},
	"guardian": {}, "elder_guardian": {},
	"silverfish": {}, "zoglin": {}, "hoglin": {}, "piglin_brute": {},
	"warden": {}, "breeze": {},
}

var neutralTypes = map[string]struct{}{
	"wolf": {}, "bee": {}, "iron_golem": {},
	"piglin": {}, "zombified_piglin": {},
	"polar_bear": {}, "panda": {}, "goat": {},
	"llama": {}, "trader_llama": {}, "dolphin": {}, "fox": {},
}

// ClassifyEntity returns the EntityCategory for a Minecraft entity type string.
// Unknown types default to CategoryPassive.
func ClassifyEntity(entityType string) EntityCategory {
	t := strings.ToLower(entityType)
	if t == "player" {
		return CategoryPlayer
	}
	if _, ok := hostileTypes[t]; ok {
		return CategoryHostile
	}
	if _, ok := neutralTypes[t]; ok {
		return CategoryNeutral
	}
	return CategoryPassive
}
