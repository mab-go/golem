package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mab-go/golem/internal/logging"
	"github.com/mab-go/golem/internal/publisher"
)

type dockerServer struct {
	containerName    string
	image            string
	port             string
	volumeName       string
	mcVersion        string
	onlineMode       bool
	removeOnExit     bool
	removeDataOnExit bool
	running          bool
	program          *tea.Program
	pub              *tuiPublisher
	logFiles         *logFiles
	log              logging.Logger
	cancel           context.CancelFunc
}

func newDockerServer(name, image, port, volumeName, mcVersion, mcAuth string, removeOnExit, removeDataOnExit bool) *dockerServer {
	return &dockerServer{
		containerName:    name,
		image:            image,
		port:             port,
		volumeName:       volumeName,
		mcVersion:        mcVersion,
		onlineMode:       mcAuth == "microsoft",
		removeOnExit:     removeOnExit,
		removeDataOnExit: removeDataOnExit,
	}
}

func (d *dockerServer) Start(ctx context.Context) error {
	d.publishStatus(publisher.StatusDegraded, "Checking container...")

	exists, running := d.inspectContainer()
	if running {
		d.running = true
		d.publishStatus(publisher.StatusOK, "Already running")
		go d.StreamLogs(ctx)
		return nil
	}

	if exists {
		d.publishStatus(publisher.StatusDegraded, "Starting existing container...")
		if _, err := d.dockerExec("start", d.containerName); err != nil {
			d.publishStatus(publisher.StatusDown, "Failed to start")
			return fmt.Errorf("docker start: %w", err)
		}
	} else {
		d.publishStatus(publisher.StatusDegraded, "Creating container...")
		onlineMode := "FALSE"
		if d.onlineMode {
			onlineMode = "TRUE"
		}
		if _, err := d.dockerExec("run", "-d",
			"--name", d.containerName,
			"-v", d.volumeName+":/data",
			"-p", d.port+":25565",
			"-e", "EULA=TRUE",
			"-e", "ONLINE_MODE="+onlineMode,
			"-e", "VERSION="+d.mcVersion,
			"-e", "SPAWN_PROTECTION=0",
			d.image,
		); err != nil {
			d.publishStatus(publisher.StatusDown, "Failed to create")
			return fmt.Errorf("docker run: %w", err)
		}
	}

	go d.StreamLogs(ctx)
	d.publishStatus(publisher.StatusDegraded, "Waiting for server...")
	if err := d.waitForReady(ctx, 300*time.Second); err != nil {
		d.publishStatus(publisher.StatusDown, "Not ready")
		return fmt.Errorf("server readiness: %w", err)
	}

	d.running = true
	d.publishStatus(publisher.StatusOK, "Running")
	return nil
}

func (d *dockerServer) Stop() error {
	if !d.running {
		return nil
	}
	d.publishStatus(publisher.StatusDegraded, "Stopping...")
	if d.cancel != nil {
		d.cancel()
	}

	var errs []error
	if _, err := d.dockerExec("stop", d.containerName); err != nil {
		d.logError("docker stop failed", err)
		errs = append(errs, fmt.Errorf("docker stop: %w", err))
	}
	if d.removeOnExit {
		if _, err := d.dockerExec("rm", "-f", d.containerName); err != nil {
			d.logError("docker rm failed", err)
			errs = append(errs, fmt.Errorf("docker rm: %w", err))
		}
		if d.removeDataOnExit {
			d.publishStatus(publisher.StatusDegraded, "Removing volume...")
			if _, err := d.dockerExec("volume", "rm", d.volumeName); err != nil {
				d.logError("docker volume rm failed", err)
				errs = append(errs, fmt.Errorf("docker volume rm: %w", err))
			}
		}
	}
	d.running = false
	d.publishStatus(publisher.StatusDown, "Stopped")
	return errors.Join(errs...)
}

func (d *dockerServer) inspectContainer() (exists, running bool) {
	out, err := d.dockerExec("inspect", "--format", "{{.State.Running}}", d.containerName)
	if err != nil {
		return false, false
	}
	return true, strings.TrimSpace(out) == "true"
}

func (d *dockerServer) waitForReady(ctx context.Context, timeout time.Duration) error {
	if d.hasHealthCheck() {
		return d.waitForHealthy(ctx, timeout)
	}
	return d.waitForTCP(ctx, timeout)
}

func (d *dockerServer) hasHealthCheck() bool {
	out, err := d.dockerExec("inspect", "--format", "{{if .State.Health}}true{{end}}", d.containerName)
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "true"
}

func (d *dockerServer) waitForHealthy(ctx context.Context, timeout time.Duration) error {
	deadline := time.After(timeout)
	for {
		out, err := d.dockerExec("inspect", "--format", "{{.State.Health.Status}}", d.containerName)
		if err == nil {
			status := strings.TrimSpace(out)
			switch status {
			case "healthy":
				return nil
			case "unhealthy":
				d.publishStatus(publisher.StatusDegraded, "Server unhealthy, retrying...")
			default:
				d.publishStatus(publisher.StatusDegraded, fmt.Sprintf("Waiting for server (%s)...", status))
			}
		}
		select {
		case <-deadline:
			return fmt.Errorf("server not healthy after %v", timeout)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func (d *dockerServer) waitForTCP(ctx context.Context, timeout time.Duration) error {
	deadline := time.After(timeout)
	for {
		conn, err := net.DialTimeout("tcp", "localhost:"+d.port, 2*time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		select {
		case <-deadline:
			return fmt.Errorf("server not ready after %v", timeout)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

func (d *dockerServer) StreamLogs(ctx context.Context) {
	logCtx, cancel := context.WithCancel(ctx)
	d.cancel = cancel

	cmd := exec.CommandContext(logCtx, "docker", "logs", "-f", "--tail", "50", d.containerName)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			t := time.Now()
			line := "[server] " + scanner.Text()
			if d.logFiles != nil {
				d.logFiles.WriteServerLog(t, "stdout", line)
			}
			d.program.Send(ServerLogMsg{
				Time:   t,
				Line:   line,
				Stream: "stdout",
			})
		}
		_ = cmd.Wait()
	}()
}

func (d *dockerServer) logError(msg string, err error) {
	if d.log != nil {
		d.log.WithError(err).Error(logging.Event("server.cleanup"), msg)
	}
}

func (d *dockerServer) publishStatus(status publisher.Status, detail string) {
	if d.pub != nil {
		d.pub.PublishComponentStatus("server", status, detail)
	}
}

func (d *dockerServer) dockerExec(args ...string) (string, error) {
	cmd := exec.Command("docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("docker %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
