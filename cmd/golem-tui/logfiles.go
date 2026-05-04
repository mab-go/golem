package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mab-go/golem/internal/logging"
)

type logFiles struct {
	agent   *fileWriter
	sidecar *fileWriter
	server  *fileWriter
}

type fileWriter struct {
	mu     sync.Mutex
	buf    *bufio.Writer
	f      *os.File
	closed bool
}

func openLogFiles(dir string) (*logFiles, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	agent, err := openFileWriter(filepath.Join(dir, "agent.log"))
	if err != nil {
		return nil, err
	}

	sidecar, err := openFileWriter(filepath.Join(dir, "sidecar.log"))
	if err != nil {
		agent.close()
		return nil, err
	}

	server, err := openFileWriter(filepath.Join(dir, "server.log"))
	if err != nil {
		agent.close()
		sidecar.close()
		return nil, err
	}

	return &logFiles{agent: agent, sidecar: sidecar, server: server}, nil
}

func openFileWriter(path string) (*fileWriter, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return &fileWriter{buf: bufio.NewWriter(f), f: f}, nil
}

func (w *fileWriter) writeLine(line string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	_, _ = w.buf.WriteString(line)
	_, _ = w.buf.WriteString("\n")
}

func (w *fileWriter) writeBlock(block string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	_, _ = w.buf.WriteString(block)
}

func (w *fileWriter) close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	w.closed = true
	_ = w.buf.Flush()
	_ = w.f.Close()
}

func (lf *logFiles) Close() {
	lf.agent.close()
	lf.sidecar.close()
	lf.server.close()
}

func (lf *logFiles) WriteAgentLog(entry logging.LogEntry) {
	var b strings.Builder
	b.WriteString(entry.Time.Format("2006-01-02T15:04:05.000"))
	b.WriteString(" ")
	fmt.Fprintf(&b, "%-5s", entry.Level.String())

	if entry.Event != "" {
		b.WriteString(" ")
		b.WriteString(entry.Event)
	}

	if entry.Message != "" && entry.Message != entry.Event {
		b.WriteString("  ")
		b.WriteString(entry.Message)
	}

	if len(entry.Fields) > 0 {
		keys := make([]string, 0, len(entry.Fields))
		for k := range entry.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, " %s=%v", k, entry.Fields[k])
		}
	}

	lf.agent.writeLine(b.String())
}

func (lf *logFiles) WriteSidecarLog(t time.Time, stream, line string) {
	lf.sidecar.writeLine(fmt.Sprintf("%s [%s] %s", t.Format("2006-01-02T15:04:05.000"), stream, line))
}

func (lf *logFiles) WriteServerLog(t time.Time, stream, line string) {
	lf.server.writeLine(fmt.Sprintf("%s [%s] %s", t.Format("2006-01-02T15:04:05.000"), stream, line))
}

func (lf *logFiles) WriteServerCmd(t time.Time, command, output string, err error) {
	ts := t.Format("2006-01-02T15:04:05.000")
	var b strings.Builder
	fmt.Fprintf(&b, "%s [rcon] > %s\n", ts, command)
	if err != nil {
		fmt.Fprintf(&b, "%s [rcon] error: %s\n", ts, err.Error())
	} else if output != "" {
		for line := range strings.SplitSeq(output, "\n") {
			fmt.Fprintf(&b, "%s [rcon] %s\n", ts, line)
		}
	}
	lf.server.writeBlock(b.String())
}
