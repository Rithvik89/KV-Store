package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the severity of a log message.
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

var logLevelNames = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
}

// Logger handles logging with different levels and components.
type Logger struct {
	component string
	level     LogLevel
	output    io.Writer
	// exit is called on Fatal; nil means os.Exit. Discard sets a no-op.
	exit func(code int)
}

var (
	defaultMu     sync.RWMutex
	defaultLogger *Logger
)

func init() {
	defaultLogger = newLogger("default", INFO, os.Stdout, nil)
}

func newLogger(component string, level LogLevel, out io.Writer, exit func(int)) *Logger {
	if out == nil {
		out = io.Discard
	}
	return &Logger{
		component: component,
		level:     level,
		output:    out,
		exit:      exit,
	}
}

// New creates a logger for a component (stdout, exits on Fatal).
func New(component string) *Logger {
	return newLogger(component, INFO, os.Stdout, nil)
}

// Discard returns a silent logger that never writes and never exits.
func Discard() *Logger {
	return newLogger("discard", LogLevel(int(FATAL)+1), io.Discard, func(int) {})
}

// Default returns the process default logger.
func Default() *Logger {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultLogger
}

// SetDefault replaces the process default logger (e.g. tests → Discard).
func SetDefault(l *Logger) {
	if l == nil {
		l = Discard()
	}
	defaultMu.Lock()
	defaultLogger = l
	defaultMu.Unlock()
}

// SetLevel sets the log level for this logger.
func (l *Logger) SetLevel(level LogLevel) {
	if l == nil {
		return
	}
	l.level = level
}

// SetOutput sets the output destination for this logger.
func (l *Logger) SetOutput(w io.Writer) {
	if l == nil {
		return
	}
	if w == nil {
		w = io.Discard
	}
	l.output = w
}

func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if l == nil {
		l = Default()
	}
	if level < l.level {
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05.000")
	message := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] %s [%s] %s\n", now, logLevelNames[level], l.component, message)
	_, _ = l.output.Write([]byte(logLine))

	if level == FATAL {
		if l.exit != nil {
			l.exit(1)
			return
		}
		os.Exit(1)
	}
}

func (l *Logger) Debug(format string, args ...interface{}) { l.log(DEBUG, format, args...) }
func (l *Logger) Info(format string, args ...interface{})  { l.log(INFO, format, args...) }
func (l *Logger) Warn(format string, args ...interface{})  { l.log(WARN, format, args...) }
func (l *Logger) Error(format string, args ...interface{}) { l.log(ERROR, format, args...) }
func (l *Logger) Fatal(format string, args ...interface{}) { l.log(FATAL, format, args...) }

// SetDefaultLevel sets the log level for the default logger.
func SetDefaultLevel(level LogLevel) {
	Default().SetLevel(level)
}

// SetDefaultOutput sets the output for the default logger.
func SetDefaultOutput(w io.Writer) {
	Default().SetOutput(w)
}

// ParseLevel maps a string to LogLevel (debug|info|warn|error).
func ParseLevel(s string) (LogLevel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return DEBUG, nil
	case "info", "":
		return INFO, nil
	case "warn", "warning":
		return WARN, nil
	case "error":
		return ERROR, nil
	default:
		return INFO, fmt.Errorf("unknown log level %q (want debug|info|warn|error)", s)
	}
}

// Package-level helpers use Default().

func Debug(format string, args ...interface{}) { Default().log(DEBUG, format, args...) }
func Info(format string, args ...interface{})  { Default().log(INFO, format, args...) }
func Warn(format string, args ...interface{})  { Default().log(WARN, format, args...) }
func Error(format string, args ...interface{}) { Default().log(ERROR, format, args...) }
func Fatal(format string, args ...interface{}) { Default().log(FATAL, format, args...) }
