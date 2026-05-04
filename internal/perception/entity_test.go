package perception

import "testing"

func TestClassifyEntity(t *testing.T) {
	tests := []struct {
		entityType string
		want       EntityCategory
	}{
		// Players
		{"player", CategoryPlayer},
		{"Player", CategoryPlayer},

		// Hostile
		{"zombie", CategoryHostile},
		{"skeleton", CategoryHostile},
		{"creeper", CategoryHostile},
		{"spider", CategoryHostile},
		{"cave_spider", CategoryHostile},
		{"enderman", CategoryHostile},
		{"witch", CategoryHostile},
		{"slime", CategoryHostile},
		{"magma_cube", CategoryHostile},
		{"blaze", CategoryHostile},
		{"ghast", CategoryHostile},
		{"wither_skeleton", CategoryHostile},
		{"husk", CategoryHostile},
		{"stray", CategoryHostile},
		{"phantom", CategoryHostile},
		{"drowned", CategoryHostile},
		{"pillager", CategoryHostile},
		{"vindicator", CategoryHostile},
		{"evoker", CategoryHostile},
		{"ravager", CategoryHostile},
		{"vex", CategoryHostile},
		{"guardian", CategoryHostile},
		{"elder_guardian", CategoryHostile},
		{"silverfish", CategoryHostile},
		{"endermite", CategoryHostile},
		{"zoglin", CategoryHostile},
		{"hoglin", CategoryHostile},
		{"piglin_brute", CategoryHostile},
		{"warden", CategoryHostile},
		{"wither", CategoryHostile},
		{"shulker", CategoryHostile},
		{"breeze", CategoryHostile},
		{"bogged", CategoryHostile},
		{"zombie_villager", CategoryHostile},

		// Neutral
		{"wolf", CategoryNeutral},
		{"bee", CategoryNeutral},
		{"iron_golem", CategoryNeutral},
		{"piglin", CategoryNeutral},
		{"zombified_piglin", CategoryNeutral},
		{"polar_bear", CategoryNeutral},
		{"goat", CategoryNeutral},
		{"llama", CategoryNeutral},

		// Passive (known)
		{"cow", CategoryPassive},
		{"sheep", CategoryPassive},
		{"chicken", CategoryPassive},
		{"pig", CategoryPassive},
		{"horse", CategoryPassive},
		{"villager", CategoryPassive},
		{"cat", CategoryPassive},
		{"rabbit", CategoryPassive},
		{"bat", CategoryPassive},

		// Passive (unknown / default)
		{"modded_boss", CategoryPassive},
		{"", CategoryPassive},
	}

	for _, tt := range tests {
		t.Run(tt.entityType, func(t *testing.T) {
			got := ClassifyEntity(tt.entityType)
			if got != tt.want {
				t.Errorf("ClassifyEntity(%q) = %v, want %v", tt.entityType, got, tt.want)
			}
		})
	}
}

func TestEntityCategory_String(t *testing.T) {
	tests := []struct {
		cat  EntityCategory
		want string
	}{
		{CategoryPlayer, "Players"},
		{CategoryHostile, "Hostile"},
		{CategoryNeutral, "Neutral"},
		{CategoryPassive, "Passive"},
		{EntityCategory(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.cat.String(); got != tt.want {
				t.Errorf("EntityCategory(%d).String() = %q, want %q", tt.cat, got, tt.want)
			}
		})
	}
}
