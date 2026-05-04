// Package main is the main package for the golem application.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mab-go/golem/internal/agent"
	"github.com/mab-go/golem/internal/claude"
	sidecar "github.com/mab-go/golem/internal/grpc"
	"github.com/mab-go/golem/internal/grpc/pb"
	"github.com/mab-go/golem/internal/logging"
	"github.com/mab-go/golem/internal/memory"
	"github.com/mab-go/golem/internal/perception"
	"github.com/mab-go/golem/internal/publisher"
	"github.com/mab-go/golem/internal/version"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	cmd = &cobra.Command{
		Use:     "golem",
		Short:   "An autonomous AI agent that plays Minecraft",
		Long:    "golem is an autonomous AI agent (powered by Claude) that plays Minecraft as a genuine co-op survival partner.",
		Version: fmt.Sprintf("golem %s (%s; %s)", version.Version, version.ShortCommit(), version.Date),
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("golem %s (%s; %s)\n", version.Version, version.ShortCommit(), version.Date)
		},
	}

	testActionsCmd = &cobra.Command{
		Use:          "test-actions",
		Short:        "Run action integration tests against the sidecar",
		Long:         "Connect to the sidecar and run a scripted sequence of Tier 0 and Tier 1 action RPCs.",
		RunE:         runTestActions,
		SilenceUsage: true,
	}

	serveCmd = &cobra.Command{
		Use:          "serve",
		Short:        "Start the golem agent",
		Long:         "Connect to the Mineflayer sidecar and run the Perceive -> Think -> Act -> Remember loop.",
		RunE:         runServe,
		SilenceUsage: true,
	}
)

func init() {
	cobra.OnInitialize(func() {
		viper.SetEnvPrefix("GOLEM")
		viper.AutomaticEnv()
	})
	cmd.SetGlobalNormalizationFunc(wordSepNormalizeFunc)
	cmd.SetVersionTemplate("{{.Version}}\n")

	// Sidecar connection.
	serveCmd.Flags().String("sidecar-address", "localhost:50051", "Sidecar gRPC address (host:port)")
	_ = viper.BindPFlag("sidecar_address", serveCmd.Flags().Lookup("sidecar-address"))

	// Minecraft connection (read by the sidecar via its own env, included here for reference/status).
	serveCmd.Flags().String("minecraft-host", "localhost", "Minecraft server host")
	_ = viper.BindPFlag("minecraft_host", serveCmd.Flags().Lookup("minecraft-host"))
	serveCmd.Flags().Int("minecraft-port", 25565, "Minecraft server port")
	_ = viper.BindPFlag("minecraft_port", serveCmd.Flags().Lookup("minecraft-port"))
	serveCmd.Flags().String("minecraft-username", "claude", "Bot username")
	_ = viper.BindPFlag("minecraft_username", serveCmd.Flags().Lookup("minecraft-username"))
	serveCmd.Flags().String("minecraft-version", "1.21.9", "Minecraft version")
	_ = viper.BindPFlag("minecraft_version", serveCmd.Flags().Lookup("minecraft-version"))
	serveCmd.Flags().String("minecraft-auth", "offline", "Minecraft auth mode (offline|microsoft)")
	_ = viper.BindPFlag("minecraft_auth", serveCmd.Flags().Lookup("minecraft-auth"))

	// Agent tunables.
	serveCmd.Flags().String("memory-dir", "./memory", "Directory for persistent memory files")
	_ = viper.BindPFlag("memory_dir", serveCmd.Flags().Lookup("memory-dir"))
	serveCmd.Flags().String("perception-format", "prose", "Perception text format (prose|structured)")
	_ = viper.BindPFlag("perception_format", serveCmd.Flags().Lookup("perception-format"))
	serveCmd.Flags().Int("perception-radius", 16, "Block radius for look_around fetches")
	_ = viper.BindPFlag("perception_radius", serveCmd.Flags().Lookup("perception-radius"))
	serveCmd.Flags().Int("history-messages", 80, "Retained conversation history length (messages)")
	_ = viper.BindPFlag("history_messages", serveCmd.Flags().Lookup("history-messages"))
	serveCmd.Flags().Duration("perception-tick", 3*time.Second, "Interval between gatekeeper perception ticks")
	_ = viper.BindPFlag("perception_tick", serveCmd.Flags().Lookup("perception-tick"))
	serveCmd.Flags().Duration("heartbeat", 45*time.Second, "Heartbeat interval for temporal awareness")
	_ = viper.BindPFlag("heartbeat", serveCmd.Flags().Lookup("heartbeat"))
	serveCmd.Flags().Duration("gatekeeper-timeout", 5*time.Second, "Timeout for gatekeeper Haiku calls")
	_ = viper.BindPFlag("gatekeeper_timeout", serveCmd.Flags().Lookup("gatekeeper-timeout"))
	serveCmd.Flags().Duration("task-timeout", 10*time.Minute, "Max duration for a background Tier 2 task")
	_ = viper.BindPFlag("task_timeout", serveCmd.Flags().Lookup("task-timeout"))

	// Anthropic overrides.
	serveCmd.Flags().String("anthropic-api-key", "", "Anthropic API key (falls back to ANTHROPIC_API_KEY env)")
	_ = viper.BindPFlag("anthropic_api_key", serveCmd.Flags().Lookup("anthropic-api-key"))
	serveCmd.Flags().Int64("max-tokens", claude.DefaultMaxTokens, "Max tokens per API response")
	_ = viper.BindPFlag("max_tokens", serveCmd.Flags().Lookup("max-tokens"))
	serveCmd.Flags().String("model", "", "Override ModelPlayer for this run")
	_ = viper.BindPFlag(claude.ViperKeyModelPlayer, serveCmd.Flags().Lookup("model"))
	serveCmd.Flags().String("model-writer", "", "Override ModelWriter for this run")
	_ = viper.BindPFlag(claude.ViperKeyModelWriter, serveCmd.Flags().Lookup("model-writer"))
	serveCmd.Flags().String("model-workhorse", "", "Override ModelWorkhorse for this run")
	_ = viper.BindPFlag(claude.ViperKeyModelWorkhorse, serveCmd.Flags().Lookup("model-workhorse"))
	serveCmd.Flags().String("model-deep", "", "Override ModelDeep (strategic advisor) for this run")
	_ = viper.BindPFlag(claude.ViperKeyModelDeep, serveCmd.Flags().Lookup("model-deep"))
	serveCmd.Flags().Duration("metrics-summary-interval", 5*time.Minute, "Interval between per-model token summary log lines (0 disables)")
	_ = viper.BindPFlag("metrics_summary_interval", serveCmd.Flags().Lookup("metrics-summary-interval"))

	testActionsCmd.Flags().String("sidecar-address", "localhost:50051", "Sidecar gRPC address (host:port)")
	_ = viper.BindPFlag("sidecar_address", testActionsCmd.Flags().Lookup("sidecar-address"))

	cmd.AddCommand(serveCmd)
	cmd.AddCommand(testActionsCmd)
}

func wordSepNormalizeFunc(_ *pflag.FlagSet, name string) pflag.NormalizedName {
	name = strings.ReplaceAll(name, "_", "-")
	return pflag.NormalizedName(name)
}

func main() {
	if err := cmd.Execute(); err != nil {
		logging.WithError(err).Fatal(eventFatalCommand)
	}
}

// ---------------------------------------------------------------------------
// serve command
// ---------------------------------------------------------------------------

type serveConfig struct {
	address                string
	memDir                 string
	perceptionFormat       perception.Format
	perceptionRadius       int32
	historyMessages        int
	perceptionTick         time.Duration
	heartbeat              time.Duration
	gatekeeperTimeout      time.Duration
	maxTokens              int64
	apiKey                 string
	botUsername            string
	metricsSummaryInterval time.Duration
	taskTimeout            time.Duration
}

func readServeConfig() (serveConfig, error) {
	perceptionFormat, err := perception.ParseFormat(viper.GetString("perception_format"))
	if err != nil {
		return serveConfig{}, err
	}

	apiKey := viper.GetString("anthropic_api_key")
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return serveConfig{}, fmt.Errorf("no Anthropic API key: set GOLEM_ANTHROPIC_API_KEY, --anthropic-api-key, or ANTHROPIC_API_KEY")
	}

	return serveConfig{
		address:                viper.GetString("sidecar_address"),
		memDir:                 viper.GetString("memory_dir"),
		perceptionFormat:       perceptionFormat,
		perceptionRadius:       int32(viper.GetInt("perception_radius")),
		historyMessages:        viper.GetInt("history_messages"),
		perceptionTick:         viper.GetDuration("perception_tick"),
		heartbeat:              viper.GetDuration("heartbeat"),
		gatekeeperTimeout:      viper.GetDuration("gatekeeper_timeout"),
		maxTokens:              viper.GetInt64("max_tokens"),
		apiKey:                 apiKey,
		botUsername:            viper.GetString("minecraft_username"),
		metricsSummaryInterval: viper.GetDuration("metrics_summary_interval"),
		taskTimeout:            viper.GetDuration("task_timeout"),
	}, nil
}

func runServe(_ *cobra.Command, _ []string) error {
	mcAuth := viper.GetString("minecraft_auth")
	switch mcAuth {
	case "offline", "microsoft":
	default:
		return fmt.Errorf("invalid --minecraft-auth value %q (want offline|microsoft)", mcAuth)
	}

	cfg, err := readServeConfig()
	if err != nil {
		return err
	}

	claude.InitModels()
	log := logging.WithFields(logging.Fields{
		"address":   cfg.address,
		"player":    claude.ModelPlayer,
		"writer":    claude.ModelWriter,
		"workhorse": claude.ModelWorkhorse,
		"deep":      claude.ModelDeep,
		"memory":    cfg.memDir,
		"username":  cfg.botUsername,
	})
	log.Info(eventStarting)

	client, err := sidecar.NewClient(cfg.address)
	if err != nil {
		return fmt.Errorf("create gRPC client: %w", err)
	}
	defer func() { _ = client.Close() }()

	connCtx, connCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer connCancel()
	if _, err := client.GetVitalSigns(connCtx); err != nil {
		return fmt.Errorf("sidecar handshake (GetVitalSigns): %w", err)
	}

	mem, err := memory.New(cfg.memDir)
	if err != nil {
		return fmt.Errorf("memory manager: %w", err)
	}

	metrics := claude.NewMetrics()
	ai := claude.NewClient(cfg.apiKey, cfg.maxTokens, metrics, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go handleSignals(cancel, log)
	go metrics.StartSummaryLoop(ctx, cfg.metricsSummaryInterval, log)

	ag := agent.New(
		ctx,
		agent.Config{
			BotUsername:            cfg.botUsername,
			PerceptionFormat:       cfg.perceptionFormat,
			PerceptionRadius:       cfg.perceptionRadius,
			HistoryMessages:        cfg.historyMessages,
			TaskTimeout:            cfg.taskTimeout,
			PerceptionTickInterval: cfg.perceptionTick,
			HeartbeatInterval:      cfg.heartbeat,
			GatekeeperTimeout:      cfg.gatekeeperTimeout,
		},
		client,
		ai,
		mem,
		cancel,
		publisher.Nop(),
		log,
	)

	return ag.Run(ctx)
}

func handleSignals(cancel context.CancelFunc, log logging.Logger) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.WithField("signal", sig.String()).Info(eventShutdown)
	cancel()
}

// ---------------------------------------------------------------------------
// test-actions command
// ---------------------------------------------------------------------------

type testStep struct {
	name string
	fn   func(context.Context) (string, error)
}

type actionResponder interface {
	GetResult() *pb.ActionResult
}

func rpcMsg(resp actionResponder, err error) (string, error) {
	if err != nil {
		return "", err
	}
	return resp.GetResult().GetMessage(), nil
}

func runTestActions(_ *cobra.Command, _ []string) error {
	address := viper.GetString("sidecar_address")
	log := logging.WithField("address", address)
	log.Info(eventConnecting)

	client, err := sidecar.NewClient(address)
	if err != nil {
		return fmt.Errorf("create gRPC client: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	surr, err := client.GetSurroundings(ctx, 16, false)
	if err != nil {
		return fmt.Errorf("GetSurroundings (for position): %w", err)
	}
	botPos := surr.Position
	log.WithFields(logging.Fields{
		"x": fmt.Sprintf("%.1f", botPos.X),
		"y": fmt.Sprintf("%.1f", botPos.Y),
		"z": fmt.Sprintf("%.1f", botPos.Z),
	}).Info(eventBotPosition)

	steps := actionTestSteps(client, botPos)
	successes, failures := executeTestSteps(steps, log)

	log.WithFields(logging.Fields{
		"successes": successes,
		"failures":  failures,
		"total":     len(steps),
	}).Info(eventTestComplete)

	if failures > 0 {
		return fmt.Errorf("%d/%d action tests failed", failures, len(steps))
	}
	return nil
}

func actionTestSteps(client *sidecar.Client, botPos *pb.Vec3) []testStep {
	return []testStep{
		{
			name: "GetVitalSigns",
			fn: func(ctx context.Context) (string, error) {
				v, err := client.GetVitalSigns(ctx)
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("health=%.1f food=%d armor=%d", v.Health, v.Food, v.Armor), nil
			},
		},
		{
			name: "Jump",
			fn: func(ctx context.Context) (string, error) {
				return rpcMsg(client.Jump(ctx))
			},
		},
		{
			name: "LookAt",
			fn: func(ctx context.Context) (string, error) {
				return rpcMsg(client.LookAt(ctx, &pb.Vec3{X: botPos.X, Y: botPos.Y + 10, Z: botPos.Z}))
			},
		},
		{
			name: "MoveTo (+5 X)",
			fn: func(ctx context.Context) (string, error) {
				return rpcMsg(client.MoveTo(ctx, &pb.Vec3{X: botPos.X + 5, Y: botPos.Y, Z: botPos.Z}, 1, false))
			},
		},
		{
			name: "SetSneak (on/off)",
			fn: func(ctx context.Context) (string, error) {
				resp, err := client.SetSneak(ctx, true)
				if err != nil {
					return "", err
				}
				resp2, err := client.SetSneak(ctx, false)
				if err != nil {
					return "", err
				}
				return resp.Result.Message + " -> " + resp2.Result.Message, nil
			},
		},
		{
			name: "NavigateTo (+10 Z)",
			fn: func(ctx context.Context) (string, error) {
				resp, err := client.NavigateTo(ctx, &pb.NavigateToRequest{
					Target: &pb.NavigateToRequest_Position{
						Position: &pb.Vec3{X: botPos.X, Y: botPos.Y, Z: botPos.Z + 10},
					},
					Range: 2,
				})
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("%s (distance=%.1f)", resp.Result.Message, resp.DistanceTraveled), nil
			},
		},
		{
			name: "HarvestBlock (dirt x1)",
			fn: func(ctx context.Context) (string, error) {
				ch, err := client.HarvestBlock(ctx, &pb.HarvestBlockRequest{
					BlockType: "dirt", Count: 1, MaxDistance: 16,
				})
				if err != nil {
					return "", err
				}
				return drainTask(ch)
			},
		},
		{
			name: "Eat (auto-select)",
			fn: func(ctx context.Context) (string, error) {
				resp, err := client.Eat(ctx, "")
				if err != nil {
					return "", err
				}
				if !resp.Result.Success {
					return "skipped: " + resp.Result.Error, nil
				}
				return fmt.Sprintf("%s (food=%s, hunger_restored=%d)", resp.Result.Message, resp.FoodUsed, resp.HungerRestored), nil
			},
		},
		{
			name: "CraftItem (stick x1)",
			fn: func(ctx context.Context) (string, error) {
				resp, err := client.CraftItem(ctx, "stick", 1)
				if err != nil {
					return "", err
				}
				if !resp.Result.Success {
					return "skipped: " + resp.Result.Error, nil
				}
				return fmt.Sprintf("%s (crafted=%d)", resp.Result.Message, resp.CraftedCount), nil
			},
		},
	}
}

func drainTask(ch <-chan sidecar.TaskEvent) (string, error) {
	var last sidecar.TaskEvent
	for ev := range ch {
		last = ev
	}
	if last.Err != nil {
		return "", last.Err
	}
	if last.Progress != nil {
		return last.Progress.GetMessage(), nil
	}
	return "no progress received", nil
}

func executeTestSteps(steps []testStep, log logging.Logger) (successes, failures int) {
	for _, step := range steps {
		stepCtx, stepCancel := context.WithTimeout(context.Background(), 30*time.Second)
		result, err := step.fn(stepCtx)
		stepCancel()

		if err != nil {
			log.WithFields(logging.Fields{
				"step":  step.name,
				"error": err.Error(),
			}).Error(eventTestFail)
			failures++
		} else {
			log.WithFields(logging.Fields{
				"step":   step.name,
				"result": result,
			}).Info(eventTestOK)
			successes++
		}
	}
	return successes, failures
}
