# logging

Structured logging following the mab-go pattern. **DO NOT MODIFY** --
this package is shared infrastructure across mab-go projects.

## Overview

Wraps logrus behind a clean `Logger` interface with structured field
support, context injection, and a TUI log sink for streaming entries to
the Bubble Tea frontend. The interface allows swapping the backend
without touching callers.

## Exported API

<!-- BEGIN:generated:exported-api -->

```
package logging // import "github.com/mab-go/golem/internal/logging"

Package logging provides a logging system for the application.

func Debug(args ...any)
func Error(args ...any)
func Fatal(args ...any)
func Info(args ...any)
func NewContext(ctx context.Context, logger Logger) context.Context
func Panic(args ...any)
func SetDefaultConfig(config Config)
func Warn(args ...any)
type Config struct{ ... }
type Event string
type Fields map[string]any
type Level uint8
    const DebugLevel Level = iota ...
type LogEntry struct{ ... }
type Logger interface{ ... }
    func FromContext(ctx context.Context) (logger Logger, created bool)
    func NewDefaultLogger() Logger
    func NewLogger(config Config) Logger
    func NewTUILogger(sink TUILogSink) Logger
    func WithError(err error) Logger
    func WithField(key string, value any) Logger
    func WithFields(fields Fields) Logger
type TUILogSink func(LogEntry)
```

<!-- END:generated:exported-api -->

## Dependencies

<!-- BEGIN:generated:dependencies -->

_No internal dependencies._

<!-- END:generated:dependencies -->

## Used By

<!-- BEGIN:generated:used-by -->

- [`golem`](../../cmd/golem/)
- [`golem-tui`](../../cmd/golem-tui/)
- [`agent`](../agent/)
- [`claude`](../claude/)
- [`game`](../game/)
- [`task`](../task/)

<!-- END:generated:used-by -->

## Key Files

| File | Purpose |
|------|---------|
| interface.go | Logger interface definition (Debug through Fatal, WithField/WithFields/WithError, Copy) |
| logging.go | logrus-backed implementation of Logger |
| config.go | Config struct for log level and output settings |
| level.go | Level type and constants (DebugLevel through FatalLevel) |
| default.go | Package-level default logger and configuration management |
| context.go | Context utilities for logger injection and extraction |
| event.go | Event type for structured event identification |
| tui.go | LogEntry struct and TUILogSink callback for the TUI pane |
