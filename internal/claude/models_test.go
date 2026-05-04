package claude

import (
	"testing"

	"github.com/spf13/viper"
)

func TestInitModelsAppliesPerTierDefaults(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	InitModels()

	if ModelPlayer != DefaultModelPlayer {
		t.Errorf("ModelPlayer = %q, want %q", ModelPlayer, DefaultModelPlayer)
	}
	if ModelWriter != DefaultModelWriter {
		t.Errorf("ModelWriter = %q, want %q", ModelWriter, DefaultModelWriter)
	}
	if ModelWorkhorse != DefaultModelWorkhorse {
		t.Errorf("ModelWorkhorse = %q, want %q", ModelWorkhorse, DefaultModelWorkhorse)
	}
	if ModelDeep != DefaultModelDeep {
		t.Errorf("ModelDeep = %q, want %q", ModelDeep, DefaultModelDeep)
	}
	if DefaultModelWorkhorse == DefaultModelPlayer {
		t.Errorf("workhorse and player should use different models, both are %q", DefaultModelPlayer)
	}
	if DefaultModelDeep == DefaultModelPlayer {
		t.Errorf("deep and player should use different models, both are %q", DefaultModelPlayer)
	}
}

func TestInitModelsHonorsViperOverrides(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	viper.Set(ViperKeyModelPlayer, "override-player")
	viper.Set(ViperKeyModelWriter, "override-writer")
	viper.Set(ViperKeyModelWorkhorse, "override-workhorse")
	viper.Set(ViperKeyModelDeep, "override-deep")

	InitModels()

	if ModelPlayer != "override-player" {
		t.Errorf("ModelPlayer = %q, want override-player", ModelPlayer)
	}
	if ModelWriter != "override-writer" {
		t.Errorf("ModelWriter = %q, want override-writer", ModelWriter)
	}
	if ModelWorkhorse != "override-workhorse" {
		t.Errorf("ModelWorkhorse = %q, want override-workhorse", ModelWorkhorse)
	}
	if ModelDeep != "override-deep" {
		t.Errorf("ModelDeep = %q, want override-deep", ModelDeep)
	}
}
