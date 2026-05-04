// Package main is the entry point for the golem-tui binary.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/mab-go/golem/internal/claude"
	"github.com/mab-go/golem/internal/logging"
	"github.com/mab-go/golem/internal/version"
)

func main() {
	cobra.OnInitialize(func() {
		viper.SetEnvPrefix("GOLEM")
		viper.AutomaticEnv()
	})

	root := &cobra.Command{
		Use:     "golem-tui",
		Short:   "TUI mission control for golem",
		Version: version.Full(),
		RunE:    runTUI,
	}
	root.SetGlobalNormalizationFunc(wordSepNormalizeFunc)

	f := root.Flags()

	// TUI-specific flags.
	f.Bool("demo", false, "run with simulated data (no backend services required)")
	f.Bool("no-agent", false, "start server and sidecar but skip the Claude agent (dev mode)")
	f.String("sidecar-dir", "./sidecar", "sidecar working directory")
	f.String("sidecar-port", "50051", "port for sidecar to listen on")
	f.Bool("sidecar-restart", true, "auto-restart sidecar on crash (max 5 attempts)")
	f.String("log-dir", "", "write agent/sidecar/server logs to files in this directory")

	// Server management flags.
	f.Bool("server", false, "manage a Minecraft server via Docker")
	f.String("server-image", "itzg/minecraft-server", "Docker image for MC server")
	f.String("server-port", "25565", "Minecraft server port")
	f.String("server-name", "golem-mc-server", "Docker container name")
	f.Bool("server-remove", false, "remove server container on exit")
	f.String("server-volume", "", "Docker volume for world data (default: <server-name>-data)")
	f.Bool("server-remove-data", false, "remove world data volume on exit (requires --server-remove)")

	// Shared with headless binary.
	f.String("sidecar-address", "localhost:50051", "sidecar gRPC address (host:port)")
	f.String("anthropic-api-key", "", "Anthropic API key (falls back to ANTHROPIC_API_KEY env)")
	f.String("bot-username", "claude", "bot username in Minecraft")
	f.String("mc-host", "localhost", "Minecraft server host")
	f.String("mc-port", "25565", "Minecraft server port")
	f.String("mc-version", "1.21.9", "Minecraft version")
	f.String("mc-auth", "offline", "Minecraft auth mode (offline|microsoft)")
	f.String("memory-dir", "./memory", "memory files directory")
	f.String("perception-format", "prose", "perception format (prose|structured)")
	f.Int("perception-radius", 16, "block radius for look_around perception")
	f.Int("history-messages", 80, "retained conversation history length")
	f.Duration("perception-tick", 3*time.Second, "gatekeeper perception tick interval")
	f.Duration("heartbeat", 45*time.Second, "heartbeat interval for temporal awareness")
	f.Duration("gatekeeper-timeout", 5*time.Second, "timeout for gatekeeper Haiku calls")
	f.Duration("task-timeout", 10*time.Minute, "max duration for background Tier 2 tasks")
	f.Int64("max-tokens", claude.DefaultMaxTokens, "max tokens per API response")
	f.Duration("metrics-summary-interval", 5*time.Minute, "metrics summary log interval (0 disables)")

	// Bind all flags to Viper.
	_ = viper.BindPFlag("no_agent", f.Lookup("no-agent"))
	_ = viper.BindPFlag("sidecar_dir", f.Lookup("sidecar-dir"))
	_ = viper.BindPFlag("sidecar_port", f.Lookup("sidecar-port"))
	_ = viper.BindPFlag("sidecar_restart", f.Lookup("sidecar-restart"))
	_ = viper.BindPFlag("log_dir", f.Lookup("log-dir"))
	_ = viper.BindPFlag("server", f.Lookup("server"))
	_ = viper.BindPFlag("server_image", f.Lookup("server-image"))
	_ = viper.BindPFlag("server_port", f.Lookup("server-port"))
	_ = viper.BindPFlag("server_name", f.Lookup("server-name"))
	_ = viper.BindPFlag("server_remove", f.Lookup("server-remove"))
	_ = viper.BindPFlag("server_volume", f.Lookup("server-volume"))
	_ = viper.BindPFlag("server_remove_data", f.Lookup("server-remove-data"))
	_ = viper.BindPFlag("sidecar_address", f.Lookup("sidecar-address"))
	_ = viper.BindPFlag("anthropic_api_key", f.Lookup("anthropic-api-key"))
	_ = viper.BindPFlag("bot_username", f.Lookup("bot-username"))
	_ = viper.BindPFlag("mc_host", f.Lookup("mc-host"))
	_ = viper.BindPFlag("mc_port", f.Lookup("mc-port"))
	_ = viper.BindPFlag("mc_version", f.Lookup("mc-version"))
	_ = viper.BindPFlag("mc_auth", f.Lookup("mc-auth"))
	_ = viper.BindPFlag("memory_dir", f.Lookup("memory-dir"))
	_ = viper.BindPFlag("perception_format", f.Lookup("perception-format"))
	_ = viper.BindPFlag("perception_radius", f.Lookup("perception-radius"))
	_ = viper.BindPFlag("history_messages", f.Lookup("history-messages"))
	_ = viper.BindPFlag("perception_tick", f.Lookup("perception-tick"))
	_ = viper.BindPFlag("heartbeat", f.Lookup("heartbeat"))
	_ = viper.BindPFlag("gatekeeper_timeout", f.Lookup("gatekeeper-timeout"))
	_ = viper.BindPFlag("task_timeout", f.Lookup("task-timeout"))
	_ = viper.BindPFlag("max_tokens", f.Lookup("max-tokens"))
	_ = viper.BindPFlag("metrics_summary_interval", f.Lookup("metrics-summary-interval"))

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func wordSepNormalizeFunc(_ *pflag.FlagSet, name string) pflag.NormalizedName {
	name = strings.ReplaceAll(name, "_", "-")
	return pflag.NormalizedName(name)
}

func runTUI(cmd *cobra.Command, _ []string) error {
	demo, _ := cmd.Flags().GetBool("demo")
	if demo {
		m := newModel(true)
		p := tea.NewProgram(m)
		go runDemoDriver(p)
		_, err := p.Run()
		return err
	}

	cfg := buildConfig()
	switch cfg.MCAuth {
	case "offline", "microsoft":
	default:
		return fmt.Errorf("invalid --mc-auth value %q (want offline|microsoft)", cfg.MCAuth)
	}

	var lf *logFiles
	if cfg.LogDir != "" {
		var err error
		lf, err = openLogFiles(cfg.LogDir)
		if err != nil {
			return fmt.Errorf("open log files: %w", err)
		}
		defer lf.Close()
	}

	orch := newOrchestrator(cfg)
	orch.logFiles = lf
	m := newModel(cfg.ServerEnabled)
	m.logFiles = lf
	p := tea.NewProgram(m)

	log := logging.NewTUILogger(func(entry logging.LogEntry) {
		if lf != nil {
			lf.WriteAgentLog(entry)
		}
		p.Send(LogMsg{
			Time:    entry.Time,
			Level:   entry.Level,
			Event:   entry.Event,
			Message: entry.Message,
			Fields:  entry.Fields,
		})
	})

	pub := newTUIPublisher(func(msg tea.Msg) { p.Send(msg) })

	orch.program = p
	orch.pub = pub
	orch.log = log

	ctx, cancel := context.WithCancel(context.Background())
	go orch.Start(ctx)

	_, err := p.Run()

	cancel()
	if cfg.ServerEnabled {
		fmt.Fprintln(os.Stderr, "Cleaning up Docker resources...")
	}
	orch.Shutdown()

	return err
}

func buildConfig() orchestratorConfig {
	cfg := orchestratorConfig{
		NoAgent:                viper.GetBool("no_agent"),
		SidecarDir:             viper.GetString("sidecar_dir"),
		SidecarAddress:         viper.GetString("sidecar_address"),
		SidecarPort:            viper.GetString("sidecar_port"),
		MCHost:                 viper.GetString("mc_host"),
		MCPort:                 viper.GetString("mc_port"),
		MCVersion:              viper.GetString("mc_version"),
		MCAuth:                 viper.GetString("mc_auth"),
		BotUsername:            viper.GetString("bot_username"),
		APIKey:                 viper.GetString("anthropic_api_key"),
		MaxTokens:              viper.GetInt64("max_tokens"),
		MemoryDir:              viper.GetString("memory_dir"),
		PerceptionFormat:       viper.GetString("perception_format"),
		PerceptionRadius:       viper.GetInt("perception_radius"),
		HistoryMessages:        viper.GetInt("history_messages"),
		PerceptionTick:         viper.GetDuration("perception_tick"),
		Heartbeat:              viper.GetDuration("heartbeat"),
		GatekeeperTimeout:      viper.GetDuration("gatekeeper_timeout"),
		TaskTimeout:            viper.GetDuration("task_timeout"),
		MetricsSummaryInterval: viper.GetDuration("metrics_summary_interval"),
		ServerEnabled:          viper.GetBool("server"),
		ServerImage:            viper.GetString("server_image"),
		ServerPort:             viper.GetString("server_port"),
		ServerName:             viper.GetString("server_name"),
		ServerVolume:           viper.GetString("server_volume"),
		ServerRemove:           viper.GetBool("server_remove"),
		ServerRemoveData:       viper.GetBool("server_remove_data"),
		SidecarRestart:         viper.GetBool("sidecar_restart"),
		LogDir:                 viper.GetString("log_dir"),
	}
	if cfg.ServerVolume == "" {
		cfg.ServerVolume = cfg.ServerName + "-data"
	}
	return cfg
}
