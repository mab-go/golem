package claude

import "github.com/spf13/viper"

// Default model IDs per tier. Phase C introduced the four-tier cognitive
// architecture: Sonnet handles moment-to-moment gameplay (conscious mind),
// Opus is the on-demand strategic advisor (deep mind), Sonnet also handles
// prose synthesis (writer), and Haiku handles reflexes (workhorse).
const (
	DefaultModelPlayer    = "claude-sonnet-4-6"
	DefaultModelWriter    = "claude-sonnet-4-6"
	DefaultModelWorkhorse = "claude-haiku-4-5-20251001"
	DefaultModelDeep      = "claude-opus-4-7"
)

// Viper config keys. Each is independent so tiers can be swapped without
// touching callers.
const (
	ViperKeyModelPlayer    = "model_player"
	ViperKeyModelWriter    = "model_writer"
	ViperKeyModelWorkhorse = "model_workhorse"
	ViperKeyModelDeep      = "model_deep"
)

// Model tier vars -- set by InitModels from Viper.
//
//   - ModelPlayer    -- conscious mind: moment-to-moment gameplay
//   - ModelWriter    -- journal/knowledge prose synthesis
//   - ModelWorkhorse -- reflexes: event classification, gatekeeper
//   - ModelDeep      -- strategic advisor: on-demand escalation via think_deeply
//
// Callers pass the appropriate constant to SendMessage; a Viper override
// replaces the default without any code change.
var (
	ModelPlayer    string
	ModelWriter    string
	ModelWorkhorse string
	ModelDeep      string
)

// InitModels reads GOLEM_MODEL_PLAYER / _WRITER / _WORKHORSE from Viper and
// populates the package-level model vars.
func InitModels() {
	viper.SetDefault(ViperKeyModelPlayer, DefaultModelPlayer)
	viper.SetDefault(ViperKeyModelWriter, DefaultModelWriter)
	viper.SetDefault(ViperKeyModelWorkhorse, DefaultModelWorkhorse)
	viper.SetDefault(ViperKeyModelDeep, DefaultModelDeep)

	ModelPlayer = viper.GetString(ViperKeyModelPlayer)
	ModelWriter = viper.GetString(ViperKeyModelWriter)
	ModelWorkhorse = viper.GetString(ViperKeyModelWorkhorse)
	ModelDeep = viper.GetString(ViperKeyModelDeep)
}
