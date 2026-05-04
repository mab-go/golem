package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mab-go/golem/internal/agent"
	"github.com/mab-go/golem/internal/claude"
	sidecar "github.com/mab-go/golem/internal/grpc"
	"github.com/mab-go/golem/internal/logging"
	"github.com/mab-go/golem/internal/memory"
	"github.com/mab-go/golem/internal/perception"
	"github.com/mab-go/golem/internal/publisher"
)

type orchestratorConfig struct {
	NoAgent        bool
	SidecarDir     string
	SidecarAddress string
	SidecarPort    string

	MCHost    string
	MCPort    string
	MCVersion string
	MCAuth    string

	BotUsername string

	APIKey    string
	MaxTokens int64

	MemoryDir              string
	PerceptionFormat       string
	PerceptionRadius       int
	HistoryMessages        int
	PerceptionTick         time.Duration
	Heartbeat              time.Duration
	GatekeeperTimeout      time.Duration
	TaskTimeout            time.Duration
	MetricsSummaryInterval time.Duration

	ServerEnabled    bool
	ServerImage      string
	ServerPort       string
	ServerName       string
	ServerVolume     string
	ServerRemove     bool
	ServerRemoveData bool
	SidecarRestart   bool
	LogDir           string
}

type orchestrator struct {
	cfg          orchestratorConfig
	program      *tea.Program
	pub          *tuiPublisher
	log          logging.Logger
	logFiles     *logFiles
	sidecar      *subprocess
	client       *sidecar.Client
	cancel       context.CancelFunc
	shutdownOnce sync.Once
	docker       *dockerServer
	restartCount int
	restartMu    sync.Mutex
}

func newOrchestrator(cfg orchestratorConfig) *orchestrator {
	return &orchestrator{cfg: cfg}
}

func (o *orchestrator) Start(ctx context.Context) {
	if err := o.startDockerIfEnabled(ctx); err != nil {
		return
	}

	if err := o.launchSidecar(ctx); err != nil {
		return
	}

	if o.cfg.NoAgent {
		o.pub.PublishComponentStatus("player", publisher.StatusDegraded, "Skipped (--no-agent)")
		o.log.Info(logging.Event("agent.skip"), "agent disabled via --no-agent")
		<-ctx.Done()
		return
	}

	perceptionFormat, err := perception.ParseFormat(o.cfg.PerceptionFormat)
	if err != nil {
		o.pub.PublishComponentStatus("player", publisher.StatusDown, err.Error())
		o.log.WithError(err).Error(logging.Event("config.error"), "invalid perception format")
		return
	}

	claude.InitModels()

	mem, err := memory.New(o.cfg.MemoryDir)
	if err != nil {
		o.pub.PublishComponentStatus("player", publisher.StatusDown, err.Error())
		o.log.WithError(err).Error(logging.Event("memory.error"), "failed to create memory manager")
		return
	}

	ai, metrics, err := o.setupClaudeClient()
	if err != nil {
		o.pub.PublishComponentStatus("player", publisher.StatusDown, err.Error())
		o.log.WithError(err).Error(logging.Event("config.error"), err.Error())
		return
	}

	agentCtx, agentCancel := context.WithCancel(ctx)
	o.cancel = agentCancel

	go metrics.StartSummaryLoop(agentCtx, o.cfg.MetricsSummaryInterval, o.log)

	agentCfg := agent.Config{
		BotUsername:            o.cfg.BotUsername,
		PerceptionFormat:       perceptionFormat,
		PerceptionRadius:       int32(o.cfg.PerceptionRadius),
		HistoryMessages:        o.cfg.HistoryMessages,
		TaskTimeout:            o.cfg.TaskTimeout,
		PerceptionTickInterval: o.cfg.PerceptionTick,
		HeartbeatInterval:      o.cfg.Heartbeat,
		GatekeeperTimeout:      o.cfg.GatekeeperTimeout,
	}

	ag := agent.New(agentCtx, agentCfg, o.client, ai, mem, agentCancel, o.pub, o.log)
	o.pub.PublishComponentStatus("player", publisher.StatusOK, "Running")
	o.log.Info(logging.Event("agent.start"), "agent loop started")

	if err := ag.Run(agentCtx); err != nil {
		o.pub.PublishComponentStatus("player", publisher.StatusDown, err.Error())
		o.log.WithError(err).Error(logging.Event("agent.stop"), "agent loop exited with error")
	} else {
		o.pub.PublishComponentStatus("player", publisher.StatusDown, "Stopped")
		o.log.Info(logging.Event("agent.stop"), "agent loop stopped")
	}
}

func (o *orchestrator) startDockerIfEnabled(ctx context.Context) error {
	if !o.cfg.ServerEnabled {
		return nil
	}
	if o.cfg.ServerRemoveData && !o.cfg.ServerRemove {
		o.log.Warn(logging.Event("server.config"), "--server-remove-data has no effect without --server-remove")
	}
	o.docker = newDockerServer(o.cfg.ServerName, o.cfg.ServerImage, o.cfg.ServerPort, o.cfg.ServerVolume, o.cfg.MCVersion, o.cfg.MCAuth, o.cfg.ServerRemove, o.cfg.ServerRemoveData)
	o.docker.pub = o.pub
	o.docker.program = o.program
	o.docker.logFiles = o.logFiles
	o.docker.log = o.log
	if err := o.docker.Start(ctx); err != nil {
		o.log.WithError(err).Error(logging.Event("server.start"), "failed to start Minecraft server")
		return err
	}
	o.log.Info(logging.Event("server.start"), "Minecraft server ready")
	containerName := o.docker.containerName
	o.program.Send(ServerReadyMsg{
		ExecCmd: func(cmd string) (string, error) {
			args := append([]string{"exec", containerName, "rcon-cli", "--"}, strings.Fields(cmd)...)
			return o.docker.dockerExec(args...)
		},
	})
	return nil
}

func (o *orchestrator) launchSidecar(ctx context.Context) error {
	o.pub.PublishComponentStatus("controller", publisher.StatusDegraded, "Starting sidecar...")

	sub, err := o.startSidecarProcess()
	if err != nil {
		o.pub.PublishComponentStatus("controller", publisher.StatusDown, err.Error())
		o.log.WithError(err).Error(logging.Event("sidecar.launch"), "failed to start sidecar")
		return err
	}
	o.sidecar = sub
	o.pumpSubprocessOutput(sub)
	go o.monitorSidecarWithRestart(ctx)

	o.pub.PublishComponentStatus("controller", publisher.StatusDegraded, "Connecting to gRPC...")
	client, err := waitForSidecar(ctx, o.cfg.SidecarAddress, 180*time.Second)
	if err != nil {
		o.pub.PublishComponentStatus("controller", publisher.StatusDown, err.Error())
		o.log.WithError(err).Error(logging.Event("sidecar.connect"), "failed to connect to sidecar")
		return err
	}
	o.client = client
	o.pub.PublishComponentStatus("controller", publisher.StatusOK, "Connected")
	o.log.WithField("address", o.cfg.SidecarAddress).Info(logging.Event("sidecar.connect"), "connected to sidecar")
	return nil
}

func (o *orchestrator) Shutdown() {
	o.shutdownOnce.Do(func() {
		if o.cancel != nil {
			o.cancel()
		}
		if o.sidecar != nil {
			_ = o.sidecar.Stop(5 * time.Second)
		}
		if o.client != nil {
			_ = o.client.Close()
		}
		if o.docker != nil {
			_ = o.docker.Stop()
		}
	})
}

func (o *orchestrator) startSidecarProcess() (*subprocess, error) {
	return startSubprocess(o.cfg.SidecarDir, "node", []string{"dist/index.js"}, []string{
		"GOLEM_SIDECAR_PORT=" + o.cfg.SidecarPort,
		"MINECRAFT_HOST=" + o.cfg.MCHost,
		"MINECRAFT_PORT=" + o.cfg.MCPort,
		"MINECRAFT_USERNAME=" + o.cfg.BotUsername,
		"MINECRAFT_VERSION=" + o.cfg.MCVersion,
		"MINECRAFT_AUTH=" + o.cfg.MCAuth,
	})
}

func (o *orchestrator) setupClaudeClient() (*claude.Client, *claude.Metrics, error) {
	apiKey := o.cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return nil, nil, fmt.Errorf("no Anthropic API key: set GOLEM_ANTHROPIC_API_KEY, --anthropic-api-key, or ANTHROPIC_API_KEY")
	}

	metrics := claude.NewMetrics()
	ai := claude.NewClient(apiKey, o.cfg.MaxTokens, metrics, o.log)
	ai.TextDeltaFunc = func(delta string) {
		o.pub.PublishTextDelta(delta)
	}
	return ai, metrics, nil
}

func (o *orchestrator) pumpSubprocessOutput(sub *subprocess) {
	go func() {
		for line := range sub.stdout.lines {
			t := time.Now()
			if o.logFiles != nil {
				o.logFiles.WriteSidecarLog(t, "stdout", line)
			}
			o.program.Send(SidecarLogMsg{
				Time:   t,
				Line:   line,
				Stream: "stdout",
			})
		}
	}()
	go func() {
		for line := range sub.stderr.lines {
			t := time.Now()
			if o.logFiles != nil {
				o.logFiles.WriteSidecarLog(t, "stderr", line)
			}
			o.program.Send(SidecarLogMsg{
				Time:   t,
				Line:   line,
				Stream: "stderr",
			})
		}
	}()
}

func (o *orchestrator) monitorSidecarWithRestart(ctx context.Context) {
	for {
		err := <-o.sidecar.done
		detail := "sidecar process exited"
		if err != nil {
			detail += ": " + err.Error()
		}
		o.pub.PublishComponentStatus("controller", publisher.StatusDown, detail)
		if err != nil {
			o.log.WithError(err).Warn(logging.Event("sidecar.exit"), detail)
		} else {
			o.log.Warn(logging.Event("sidecar.exit"), detail)
		}

		if !o.cfg.SidecarRestart {
			return
		}

		o.restartMu.Lock()
		if o.restartCount >= 5 {
			o.restartMu.Unlock()
			o.pub.PublishComponentStatus("controller", publisher.StatusDown, "Too many restarts")
			o.log.Error(logging.Event("sidecar.restart"), "exceeded max restart attempts (5)")
			return
		}
		o.restartCount++
		count := o.restartCount
		o.restartMu.Unlock()

		o.log.WithField("attempt", count).Info(logging.Event("sidecar.restart"), "restarting sidecar...")
		o.pub.PublishComponentStatus("controller", publisher.StatusDegraded, fmt.Sprintf("Restarting (%d/5)...", count))

		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}

		if err := o.restartSidecar(ctx); err != nil {
			o.pub.PublishComponentStatus("controller", publisher.StatusDown, err.Error())
			o.log.WithError(err).Error(logging.Event("sidecar.restart"), "restart failed")
			return
		}
	}
}

func (o *orchestrator) restartSidecar(ctx context.Context) error {
	if o.client != nil {
		_ = o.client.Close()
		o.client = nil
	}

	sub, err := o.startSidecarProcess()
	if err != nil {
		return fmt.Errorf("start subprocess: %w", err)
	}
	o.sidecar = sub
	o.pumpSubprocessOutput(sub)

	client, err := waitForSidecar(ctx, o.cfg.SidecarAddress, 180*time.Second)
	if err != nil {
		return fmt.Errorf("gRPC reconnect: %w", err)
	}
	o.client = client
	o.pub.PublishComponentStatus("controller", publisher.StatusOK, "Reconnected")
	o.log.Info(logging.Event("sidecar.restart"), "sidecar restarted successfully")
	return nil
}

func waitForSidecar(ctx context.Context, address string, timeout time.Duration) (*sidecar.Client, error) {
	deadline := time.After(timeout)
	backoff := 500 * time.Millisecond

	for {
		client, err := sidecar.NewClient(address)
		if err == nil {
			checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, err = client.GetVitalSigns(checkCtx)
			cancel()
			if err == nil {
				return client, nil
			}
			_ = client.Close()
		}

		select {
		case <-deadline:
			return nil, fmt.Errorf("sidecar not ready after %v", timeout)
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			backoff = min(backoff*2, 5*time.Second)
		}
	}
}
