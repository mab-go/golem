package logging

import (
	"fmt"
	"maps"
	"os"
	"time"
)

// LogEntry is the structured payload sent to a TUILogSink.
type LogEntry struct {
	Time    time.Time
	Level   Level
	Event   string
	Message string
	Fields  Fields
}

// TUILogSink is a callback that receives structured log entries.
// It must be safe for concurrent calls.
type TUILogSink func(LogEntry)

type tuiLogger struct {
	sink   TUILogSink
	fields Fields
}

// NewTUILogger returns a Logger that sends every log entry to sink.
// The sink is called synchronously from the logging goroutine; it
// should be non-blocking (e.g. tea.Program.Send).
func NewTUILogger(sink TUILogSink) Logger {
	return &tuiLogger{
		sink:   sink,
		fields: make(Fields),
	}
}

var _ Logger = (*tuiLogger)(nil)

func (log *tuiLogger) Debug(args ...any) { log.emit(DebugLevel, args) }
func (log *tuiLogger) Info(args ...any)  { log.emit(InfoLevel, args) }
func (log *tuiLogger) Warn(args ...any)  { log.emit(WarnLevel, args) }
func (log *tuiLogger) Error(args ...any) { log.emit(ErrorLevel, args) }

func (log *tuiLogger) Fatal(args ...any) {
	log.emit(FatalLevel, args)
	os.Exit(1)
}

func (log *tuiLogger) Panic(args ...any) {
	log.emit(PanicLevel, args)
	panic(formatArgs(args))
}

func (log *tuiLogger) WithField(key string, value any) Logger {
	return &tuiLogger{
		sink:   log.sink,
		fields: log.copyFieldsWith(key, value),
	}
}

func (log *tuiLogger) WithFields(fields Fields) Logger {
	merged := log.copyFields()
	maps.Copy(merged, fields)
	return &tuiLogger{
		sink:   log.sink,
		fields: merged,
	}
}

func (log *tuiLogger) WithError(err error) Logger {
	return log.WithField("error", err.Error())
}

func (log *tuiLogger) Copy() Logger {
	return &tuiLogger{
		sink:   log.sink,
		fields: log.copyFields(),
	}
}

func (log *tuiLogger) emit(level Level, args []any) {
	event, msg := log.extractEventAndFormat(args)
	log.sink(LogEntry{
		Time:    time.Now(),
		Level:   level,
		Event:   event,
		Message: msg,
		Fields:  log.copyFields(),
	})
}

func (log *tuiLogger) extractEventAndFormat(args []any) (event, message string) {
	if len(args) > 0 {
		if e, ok := args[0].(Event); ok {
			event = string(e)
			args = args[1:]
			if len(args) == 0 {
				message = event
				return
			}
		}
	}
	message = formatArgs(args)
	return
}

func formatArgs(args []any) string {
	if len(args) == 0 {
		return ""
	}
	if len(args) == 1 {
		return fmt.Sprint(args[0])
	}
	if fmtStr, ok := args[0].(string); ok {
		return fmt.Sprintf(fmtStr, args[1:]...)
	}
	return fmt.Sprint(args...)
}

func (log *tuiLogger) copyFields() Fields {
	cp := make(Fields, len(log.fields))
	maps.Copy(cp, log.fields)
	return cp
}

func (log *tuiLogger) copyFieldsWith(key string, value any) Fields {
	cp := make(Fields, len(log.fields)+1)
	maps.Copy(cp, log.fields)
	cp[key] = value
	return cp
}
