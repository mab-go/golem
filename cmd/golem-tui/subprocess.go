package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type lineReader struct {
	lines chan string
}

type subprocess struct {
	cmd    *exec.Cmd
	stdout *lineReader
	stderr *lineReader
	done   chan error
}

func startSubprocess(dir, name string, args, env []string) (*subprocess, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", name, err)
	}

	stdoutReader := &lineReader{lines: make(chan string, 256)}
	stderrReader := &lineReader{lines: make(chan string, 256)}
	done := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			stdoutReader.lines <- scanner.Text()
		}
		close(stdoutReader.lines)
	}()

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			stderrReader.lines <- scanner.Text()
		}
		close(stderrReader.lines)
	}()

	go func() {
		done <- cmd.Wait()
		close(done)
	}()

	return &subprocess{
		cmd:    cmd,
		stdout: stdoutReader,
		stderr: stderrReader,
		done:   done,
	}, nil
}

func (s *subprocess) Stop(timeout time.Duration) error {
	if s.cmd.Process == nil {
		return nil
	}
	if err := s.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("sigterm: %w", err)
	}
	select {
	case err := <-s.done:
		return err
	case <-time.After(timeout):
		if err := s.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("sigkill: %w", err)
		}
		return <-s.done
	}
}
