package claude

// Canonical tool names. Kept as constants so callers and dispatchers reference
// a single source of truth.
const (
	// Tier 0 -- atomic actions
	ToolMoveTo       = "move_to"
	ToolLookAt       = "look_at"
	ToolPlaceBlock   = "place_block"
	ToolDigBlock     = "dig_block"
	ToolEquipItem    = "equip_item"
	ToolUseItem      = "use_item"
	ToolAttackEntity = "attack_entity"
	ToolJump         = "jump"
	ToolSetSneak     = "set_sneak"
	ToolSendChat     = "send_chat"

	// Tier 1 -- action verbs
	ToolNavigateTo            = "navigate_to"
	ToolInteractWithEntity    = "interact_with_entity"
	ToolOpenContainer         = "open_container"
	ToolWithdrawFromContainer = "withdraw_from_container"
	ToolDepositToContainer    = "deposit_to_container"
	ToolCraftItem             = "craft_item"
	ToolSmeltItem             = "smelt_item"
	ToolEat                   = "eat"

	// Tier 2 -- goal-oriented tasks (background)
	ToolHarvestBlock      = "harvest_block"
	ToolGather            = "gather"
	ToolBuildStructure    = "build_structure"
	ToolProcessAll        = "process_all"
	ToolOrganizeInventory = "organize_inventory"
	ToolClearArea         = "clear_area"
	ToolFarm              = "farm"
	ToolCancelTask        = "cancel_task"

	// Tier 3 -- strategic / planning (read-only)
	ToolSurveyArea    = "survey_area"
	ToolFindNearest   = "find_nearest"
	ToolWhatCanICraft = "what_can_i_craft"
	ToolAssessThreat  = "assess_threat"
	ToolPlanPath      = "plan_path"

	// Perception
	ToolLookAround     = "look_around"
	ToolCheckInventory = "check_inventory"
	ToolTakeScreenshot = "take_screenshot"

	// Meta -- operate on agent state / memory
	ToolSetVerbosity         = "set_verbosity"
	ToolWriteJournal         = "write_journal"
	ToolUpdateGoals          = "update_goals"
	ToolUpdateWorldKnowledge = "update_world_knowledge"
	ToolUpdateInventoryNotes = "update_inventory_notes"
	ToolUpdateSocial         = "update_social"

	// Escalation -- invoke deeper reasoning
	ToolThinkDeeply = "think_deeply"
)

// Tools returns every tool definition golem exposes to Claude. Order matters
// only for display purposes in system_prompt.go (tier 0, tier 1, perception,
// meta) -- Claude reads them as a set.
func Tools() []Tool {
	tools := []Tool{}
	tools = append(tools, tier0Tools()...)
	tools = append(tools, tier1Tools()...)
	tools = append(tools, tier2Tools()...)
	tools = append(tools, cancelTaskTool()...)
	tools = append(tools, tier3Tools()...)
	tools = append(tools, perceptionTools()...)
	tools = append(tools, metaTools()...)
	tools = append(tools, escalationTools()...)
	return tools
}

// ToolsByName indexes the catalog for dispatcher lookups.
func ToolsByName() map[string]Tool {
	m := map[string]Tool{}
	for _, t := range Tools() {
		m[t.Name] = t
	}
	return m
}

// --- Tier 0 ------------------------------------------------------------------

func tier0Tools() []Tool {
	vec3 := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"x", "y", "z"},
		"properties": map[string]any{
			"x": map[string]any{"type": "number"},
			"y": map[string]any{"type": "number"},
			"z": map[string]any{"type": "number"},
		},
	}
	return []Tool{
		{
			Name:        ToolMoveTo,
			Description: "Walk to exact absolute coordinates using the pathfinder. Low-level; prefer navigate_to for higher-level goals.",
			InputSchema: objectSchema(
				[]string{"target"},
				map[string]any{
					"target": vec3,
					"range":  map[string]any{"type": "number", "description": "how close to get (default 1)"},
					"sprint": map[string]any{"type": "boolean"},
				},
			),
		},
		{
			Name:        ToolLookAt,
			Description: "Turn the bot's head to face absolute coordinates.",
			InputSchema: objectSchema([]string{"target"}, map[string]any{"target": vec3}),
		},
		{
			Name:        ToolPlaceBlock,
			Description: "Place a block from inventory adjacent to an existing block. `position` is the reference block; `face` is which face to place against.",
			InputSchema: objectSchema(
				[]string{"position", "face", "block_name"},
				map[string]any{
					"position":   vec3,
					"face":       map[string]any{"type": "string", "enum": []string{"top", "bottom", "north", "south", "east", "west"}},
					"block_name": map[string]any{"type": "string"},
				},
			),
		},
		{
			Name:        ToolDigBlock,
			Description: "Dig the block at the given position. The bot must be within reach.",
			InputSchema: objectSchema([]string{"position"}, map[string]any{"position": vec3}),
		},
		{
			Name:        ToolEquipItem,
			Description: "Equip an inventory item to a slot (hand, head, torso, legs, feet, off-hand).",
			InputSchema: objectSchema(
				[]string{"item_name", "destination"},
				map[string]any{
					"item_name":   map[string]any{"type": "string"},
					"destination": map[string]any{"type": "string", "enum": []string{"hand", "head", "torso", "legs", "feet", "off-hand"}},
				},
			),
		},
		{
			Name:        ToolUseItem,
			Description: "Activate the item currently held in the main hand (eat food, drink potion, shoot bow, etc.).",
			InputSchema: objectSchema(nil, map[string]any{}),
		},
		{
			Name:        ToolAttackEntity,
			Description: "Swing at the entity with the given numeric ID. Use this only for hostile mobs; prefer interact_with_entity for harvesting peaceful animals.",
			InputSchema: objectSchema(
				[]string{"entity_id"},
				map[string]any{"entity_id": map[string]any{"type": "integer"}},
			),
		},
		{
			Name:        ToolJump,
			Description: "Make the bot jump once.",
			InputSchema: objectSchema(nil, map[string]any{}),
		},
		{
			Name:        ToolSetSneak,
			Description: "Enable or disable sneaking.",
			InputSchema: objectSchema(
				[]string{"enabled"},
				map[string]any{"enabled": map[string]any{"type": "boolean"}},
			),
		},
		{
			Name:        ToolSendChat,
			Description: "Send a public chat message in-game. Use this to talk with other players.",
			InputSchema: objectSchema(
				[]string{"message"},
				map[string]any{"message": map[string]any{"type": "string"}},
			),
		},
	}
}

// --- Tier 1 ------------------------------------------------------------------

func tier1Tools() []Tool {
	return []Tool{
		{
			Name: ToolNavigateTo,
			Description: "High-level pathfinding toward a target. Provide exactly one of `position`, `entity_name`, or `block_type`. " +
				"Walks over existing terrain by default. Set `allow_dig=true` only when you explicitly want the pathfinder to mine through blocks " +
				"to reach the target -- risky, since it can dig straight down into lava or off a cliff. For gathering resources, prefer the `gather` tool.",
			InputSchema: objectSchema(
				nil,
				map[string]any{
					"position": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"x": map[string]any{"type": "number"},
							"y": map[string]any{"type": "number"},
							"z": map[string]any{"type": "number"},
						},
						"required": []string{"x", "y", "z"},
					},
					"entity_name": map[string]any{"type": "string", "description": "e.g. cow, villager, zombie"},
					"block_type":  map[string]any{"type": "string", "description": "e.g. oak_log, iron_ore"},
					"range":       map[string]any{"type": "number", "description": "how close to get (default 2)"},
					"allow_dig":   map[string]any{"type": "boolean", "description": "if true, pathfinder may mine/pillar to reach target (risky). Default false."},
				},
			),
		},
		{
			Name: ToolInteractWithEntity,
			Description: "Interact with a nearby entity using a named action. Actions: harvest, attack, feed, trade, mount, shear. " +
				"Use harvest for peaceful mobs you want to kill for drops (neutral vocabulary avoids guardrails).",
			InputSchema: objectSchema(
				[]string{"action"},
				map[string]any{
					"entity_name": map[string]any{"type": "string"},
					"entity_id":   map[string]any{"type": "integer"},
					"action":      map[string]any{"type": "string", "enum": []string{"harvest", "attack", "feed", "trade", "mount", "shear"}},
				},
			),
		},
		{
			Name:        ToolOpenContainer,
			Description: "Open a chest/furnace/etc. Provide either `position` of the block or `block_type` to find the nearest.",
			InputSchema: objectSchema(
				nil,
				map[string]any{
					"position": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"x": map[string]any{"type": "number"},
							"y": map[string]any{"type": "number"},
							"z": map[string]any{"type": "number"},
						},
						"required": []string{"x", "y", "z"},
					},
					"block_type": map[string]any{"type": "string"},
				},
			),
		},
		{
			Name:        ToolWithdrawFromContainer,
			Description: "Take items from a container (chest, furnace, barrel, etc.) into inventory. Provide either `position` or `block_type` (find nearest). For furnaces, defaults to the output slot; override with `slot`.",
			InputSchema: objectSchema(
				[]string{"item_name"},
				map[string]any{
					"position": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"x": map[string]any{"type": "number"},
							"y": map[string]any{"type": "number"},
							"z": map[string]any{"type": "number"},
						},
						"required": []string{"x", "y", "z"},
					},
					"block_type": map[string]any{"type": "string"},
					"item_name":  map[string]any{"type": "string", "description": "item to withdraw, e.g. iron_ingot"},
					"count":      map[string]any{"type": "integer", "description": "how many to take (0 or omit = all available)"},
					"slot":       map[string]any{"type": "string", "enum": []string{"input", "fuel", "output"}, "description": "furnace slot (default: output); ignored for non-furnace containers"},
				},
			),
		},
		{
			Name:        ToolDepositToContainer,
			Description: "Put items from inventory into a container (chest, furnace, barrel, etc.). Provide either `position` or `block_type` (find nearest). For furnaces, defaults to the input slot; override with `slot`.",
			InputSchema: objectSchema(
				[]string{"item_name"},
				map[string]any{
					"position": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"x": map[string]any{"type": "number"},
							"y": map[string]any{"type": "number"},
							"z": map[string]any{"type": "number"},
						},
						"required": []string{"x", "y", "z"},
					},
					"block_type": map[string]any{"type": "string"},
					"item_name":  map[string]any{"type": "string", "description": "item to deposit, e.g. coal"},
					"count":      map[string]any{"type": "integer", "description": "how many to deposit (0 or omit = all in inventory)"},
					"slot":       map[string]any{"type": "string", "enum": []string{"input", "fuel"}, "description": "furnace slot (default: input); ignored for non-furnace containers"},
				},
			),
		},
		{
			Name:        ToolCraftItem,
			Description: "Craft the named item. `count` is the desired number of output items (not recipe executions); for multi-output recipes the recipe runs just enough times to produce at least `count` items. Uses a crafting table automatically if required and one is nearby. Returns the full inventory after crafting -- no need to call check_inventory afterward.",
			InputSchema: objectSchema(
				[]string{"item_name"},
				map[string]any{
					"item_name": map[string]any{"type": "string"},
					"count":     map[string]any{"type": "integer", "description": "desired number of output items (default 1)"},
				},
			),
		},
		{
			Name:        ToolSmeltItem,
			Description: "Smelt `count` of the named item in a nearby furnace. Fuel is auto-selected if omitted.",
			InputSchema: objectSchema(
				[]string{"item_name"},
				map[string]any{
					"item_name": map[string]any{"type": "string"},
					"count":     map[string]any{"type": "integer", "description": "default 1"},
					"fuel":      map[string]any{"type": "string"},
				},
			),
		},
		{
			Name:        ToolEat,
			Description: "Eat food to restore hunger. Leave `food_name` empty to auto-pick the best available food.",
			InputSchema: objectSchema(
				nil,
				map[string]any{
					"food_name": map[string]any{"type": "string"},
				},
			),
		},
	}
}

// --- Tier 2 ------------------------------------------------------------------

func tier2Tools() []Tool {
	vec3 := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"x", "y", "z"},
		"properties": map[string]any{
			"x": map[string]any{"type": "number"},
			"y": map[string]any{"type": "number"},
			"z": map[string]any{"type": "number"},
		},
	}
	return []Tool{
		{
			Name: ToolHarvestBlock,
			Description: "Background task: find and mine N blocks of a given type within max_distance. " +
				"Returns immediately; progress appears in perception stream. Use cancel_task to abort.",
			InputSchema: objectSchema(
				[]string{"block_type"},
				map[string]any{
					"block_type":   map[string]any{"type": "string"},
					"count":        map[string]any{"type": "integer", "description": "default 1"},
					"max_distance": map[string]any{"type": "integer", "description": "search radius in blocks (default 16)"},
				},
			),
		},
		{
			Name: ToolGather,
			Description: "Background task: collect N of a block type within a radius. " +
				"Handles findBlock -> navigate -> equip tool -> dig -> collect drops in a loop. " +
				"Returns immediately; progress and completion appear in your perception stream. Use cancel_task to abort.",
			InputSchema: objectSchema(
				[]string{"resource", "quantity"},
				map[string]any{
					"resource": map[string]any{"type": "string", "description": "block type to gather, e.g. oak_log, iron_ore"},
					"quantity": map[string]any{"type": "integer", "description": "target count"},
					"radius":   map[string]any{"type": "integer", "description": "search radius in blocks (default 32)"},
				},
			),
		},
		{
			Name: ToolBuildStructure,
			Description: "Background task: construct a simple box shelter (4 walls + roof) at the given location with the given material. " +
				"Dimensions are widthxheightxdepth; all must be >=3. Supply must already be in inventory -- the task will stop if material runs out. " +
				"Returns immediately; progress and completion appear in your perception stream. Use cancel_task to abort.",
			InputSchema: objectSchema(
				[]string{"type", "location", "material"},
				map[string]any{
					"type":       map[string]any{"type": "string", "description": "structure type; currently only 'shelter' is supported"},
					"location":   vec3,
					"dimensions": vec3,
					"material":   map[string]any{"type": "string", "description": "block type to place, e.g. oak_planks, cobblestone"},
				},
			),
		},
		{
			Name: ToolProcessAll,
			Description: "Background task: smelt every unit of `item` in inventory at the nearest furnace/blast_furnace/smoker. Requires fuel (coal/charcoal/planks/sticks) in inventory. " +
				"Returns immediately; progress and completion appear in your perception stream. Use cancel_task to abort.",
			InputSchema: objectSchema(
				[]string{"item"},
				map[string]any{"item": map[string]any{"type": "string"}},
			),
		},
		{
			Name: ToolOrganizeInventory,
			Description: "Background task: reorganize inventory -- move weapons/tools/shield to hotbar. With `stash_junk: true`, also deposit low-value items (dirt, cobble, rotten flesh, etc.) in the nearest chest/barrel within 24 blocks. " +
				"Returns immediately; progress and completion appear in your perception stream. Use cancel_task to abort.",
			InputSchema: objectSchema(
				nil,
				map[string]any{"stash_junk": map[string]any{"type": "boolean", "description": "if true, deposit junk items in a nearby chest"}},
			),
		},
		{
			Name: ToolClearArea,
			Description: "Background task: dig every non-air, non-liquid block within `radius` of `center`, at center Y and Y+1, Y+2 (three-high corridor). Use to excavate a site or clear a staging area. " +
				"Returns immediately; progress and completion appear in your perception stream. Use cancel_task to abort.",
			InputSchema: objectSchema(
				[]string{"center", "radius"},
				map[string]any{
					"center": vec3,
					"radius": map[string]any{"type": "integer"},
				},
			),
		},
		{
			Name: ToolFarm,
			Description: "Background task: harvest mature crops and replant at the given location within radius. " +
				"Supported crops: wheat, carrots, potatoes, beetroots. Seeds must be in inventory for replanting. " +
				"Returns immediately; progress and completion appear in your perception stream. Use cancel_task to abort.",
			InputSchema: objectSchema(
				[]string{"crop", "location"},
				map[string]any{
					"crop":     map[string]any{"type": "string", "enum": []string{"wheat", "carrots", "potatoes", "beetroots"}},
					"location": vec3,
					"radius":   map[string]any{"type": "integer", "description": "search radius in blocks (default 8)"},
				},
			),
		},
	}
}

func cancelTaskTool() []Tool {
	return []Tool{
		{
			Name:        ToolCancelTask,
			Description: "Cancel the currently running background task. Returns a confirmation or a message if no task is active.",
			InputSchema: objectSchema(nil, map[string]any{}),
		},
	}
}

// --- Tier 3 ------------------------------------------------------------------

func tier3Tools() []Tool {
	vec3 := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"x", "y", "z"},
		"properties": map[string]any{
			"x": map[string]any{"type": "number"},
			"y": map[string]any{"type": "number"},
			"z": map[string]any{"type": "number"},
		},
	}
	return []Tool{
		{
			Name:        ToolSurveyArea,
			Description: "Scan the world around the bot and return clustered block types, nearby entities, and detected structures (villages, caves, etc.). Read-only strategic query -- use before committing to multi-step plans.",
			InputSchema: objectSchema(
				nil,
				map[string]any{
					"radius": map[string]any{"type": "integer", "description": "block radius to scan (default 32, max 128)"},
				},
			),
		},
		{
			Name: ToolFindNearest,
			Description: "Locate the nearest block, entity, or structure of a given type. Provide exactly one of `block_type`, `entity_type`, or `structure_type`. " +
				"Returns position and distance if found. Read-only -- does not move the bot.",
			InputSchema: objectSchema(
				nil,
				map[string]any{
					"block_type":     map[string]any{"type": "string"},
					"entity_type":    map[string]any{"type": "string"},
					"structure_type": map[string]any{"type": "string"},
					"radius":         map[string]any{"type": "integer", "description": "search radius in blocks (default 64, max 128)"},
				},
			),
		},
		{
			Name:        ToolWhatCanICraft,
			Description: "Enumerate every recipe currently craftable from inventory, with max counts and crafting-table requirements. Use when deciding what to build next.",
			InputSchema: objectSchema(nil, map[string]any{}),
		},
		{
			Name:        ToolAssessThreat,
			Description: "Return a threat level (safe/low/moderate/high/critical) based on nearby hostile mobs, time until nightfall, and local light. Read-only -- use before committing to risky plans or when deciding to shelter.",
			InputSchema: objectSchema(
				nil,
				map[string]any{
					"radius": map[string]any{"type": "integer", "description": "scan radius in blocks (default 24)"},
				},
			),
		},
		{
			Name:        ToolPlanPath,
			Description: "Compute a route to the destination without walking it. Returns reachability, distance, estimated travel seconds, hazards (lava, cliffs, water), and waypoints. Uses safe movement by default (no digging, no pillaring). Set allow_dig=true to check reachability with full movement (matches navigate_to allow_dig=true).",
			InputSchema: objectSchema(
				[]string{"destination"},
				map[string]any{
					"destination": vec3,
					"allow_dig":   map[string]any{"type": "boolean", "description": "if true, check reachability assuming pathfinder may dig/pillar (default false)"},
				},
			),
		},
	}
}

// --- Perception --------------------------------------------------------------

func perceptionTools() []Tool {
	vec3 := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"x", "y", "z"},
		"properties": map[string]any{
			"x": map[string]any{"type": "number"},
			"y": map[string]any{"type": "number"},
			"z": map[string]any{"type": "number"},
		},
	}
	return []Tool{
		{
			Name:        ToolLookAround,
			Description: "Re-fetch a full room description including nearby entities and notable blocks. Use when you need fresh perception mid-turn.",
			InputSchema: objectSchema(
				nil,
				map[string]any{
					"radius": map[string]any{"type": "integer", "description": "block radius to scan (default 16)"},
				},
			),
		},
		{
			Name:        ToolCheckInventory,
			Description: "Re-fetch the current inventory, including craft suggestions based on what is carried.",
			InputSchema: objectSchema(nil, map[string]any{}),
		},
		{
			Name: ToolTakeScreenshot,
			Description: "Capture a first-person PNG screenshot. Use for strategic visual reasoning -- spotting distant threats, identifying structures, verifying terrain -- not for casual curiosity. " +
				"Each call costs ~1.2K tokens (low) or ~4.7K (high) and invalidates prompt caching downstream. Prefer `look_around` or `survey_area` for routine awareness. " +
				"Optional `look_at` directs the camera at absolute coordinates before the snapshot.",
			InputSchema: objectSchema(
				nil,
				map[string]any{
					"resolution": map[string]any{"type": "string", "enum": []string{"low", "high"}, "description": "low (480x360) or high (960x720); default low"},
					"look_at":    vec3,
				},
			),
		},
	}
}

// --- Meta --------------------------------------------------------------------

func metaTools() []Tool {
	return []Tool{
		{
			Name:        ToolSetVerbosity,
			Description: "Change how verbose future perception descriptions are. Use terse when acting quickly, verbose when exploring.",
			InputSchema: objectSchema(
				[]string{"mode"},
				map[string]any{
					"mode": map[string]any{"type": "string", "enum": []string{"verbose", "standard", "terse"}},
				},
			),
		},
		{
			Name:        ToolWriteJournal,
			Description: "Append a timestamped entry to journal.md. Use this to record observations, discoveries, decisions, and reflections worth remembering across sessions.",
			InputSchema: objectSchema(
				[]string{"entry"},
				map[string]any{
					"entry": map[string]any{"type": "string", "description": "Markdown body; no date prefix -- the timestamp is added automatically."},
				},
			),
		},
		{
			Name:        ToolUpdateGoals,
			Description: "Replace goals.md with a new set of goals (Markdown). Use this when priorities shift; partial edits are not supported -- write the full file.",
			InputSchema: objectSchema(
				[]string{"content"},
				map[string]any{
					"content": map[string]any{"type": "string"},
				},
			),
		},
		{
			Name:        ToolUpdateWorldKnowledge,
			Description: "Replace world-knowledge.md with a new Markdown body capturing durable facts about the world -- ore locations, structures, biomes, hostile nests, useful coordinates. Partial edits are not supported; write the full file.",
			InputSchema: objectSchema(
				[]string{"content"},
				map[string]any{
					"content": map[string]any{"type": "string"},
				},
			),
		},
		{
			Name:        ToolUpdateInventoryNotes,
			Description: "Replace inventory-notes.json with a new JSON object tracking stashes, chests, and item locations that persist beyond the live inventory. Partial edits are not supported; write the full object. Shape is free-form -- use whatever keys make the notes easy to read back later.",
			InputSchema: objectSchema(
				[]string{"notes"},
				map[string]any{
					"notes": map[string]any{"type": "object", "description": "The full replacement JSON object."},
				},
			),
		},
		{
			Name:        ToolUpdateSocial,
			Description: "Replace social.md with a new Markdown body capturing what you've learned about other players and NPCs -- names, dispositions, agreements, notable past interactions. Partial edits are not supported; write the full file.",
			InputSchema: objectSchema(
				[]string{"content"},
				map[string]any{
					"content": map[string]any{"type": "string"},
				},
			),
		},
	}
}

// --- Escalation --------------------------------------------------------------

func escalationTools() []Tool {
	return []Tool{
		{
			Name: ToolThinkDeeply,
			Description: "Escalate a complex decision to deep strategic reasoning. Use when you " +
				"encounter a novel situation, need to form a long-term plan, face a decision " +
				"with significant consequences, or feel uncertain about the best approach. " +
				"Returns strategic guidance. Cost is high -- reserve for genuinely hard problems.",
			InputSchema: objectSchema(
				[]string{"situation"},
				map[string]any{
					"situation": map[string]any{
						"type":        "string",
						"description": "Describe the situation, what you're uncertain about, and what kind of guidance you need.",
					},
				},
			),
		},
	}
}

// objectSchema builds a JSON-Schema-2020-12 object schema with required props
// list flattened. Pass nil `required` for a fully-optional object.
func objectSchema(required []string, properties map[string]any) map[string]any {
	m := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           properties,
	}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}
