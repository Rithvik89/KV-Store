package logger

import (
	"fmt"
	"io"
	"os"
	"time"
)

// LogLevel represents the severity of a log message
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

// Logger handles logging with different levels and components
type Logger struct {
	component string
	level     LogLevel
	output    io.Writer
}

// defaultLogger is the global logger instance
var (
	defaultLogger *Logger
)

// init initializes the default logger
func init() {
	defaultLogger = &Logger{
		component: "default",
		level:     INFO,
		output:    os.Stdout,
	}
}

// New creates a new logger for a specific component
func New(component string) *Logger {
	return &Logger{
		component: component,
		level:     INFO,
		output:    os.Stdout,
	}
}

// SetLevel sets the log level for this logger
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// SetOutput sets the output destination for this logger
func (l *Logger) SetOutput(w io.Writer) {
	l.output = w
}

// log formats and prints a message if its level is >= the current log level
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {

	if level < l.level {
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05.000")
	message := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] %s [%s] %s\n", now, logLevelNames[level], l.component, message)

	l.output.Write([]byte(logLine))

	// Exit on fatal
	if level == FATAL {
		os.Exit(1)
	}
}

// Debug logs a debug-level message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs an info-level message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn logs a warning-level message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error logs an error-level message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Fatal logs a fatal-level message and exits
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FATAL, format, args...)
}

// Package-level functions using the default logger

// SetDefaultLevel sets the log level for the default logger
func SetDefaultLevel(level LogLevel) {
	defaultLogger.SetLevel(level)
}

// SetDefaultOutput sets the output for the default logger
func SetDefaultOutput(w io.Writer) {
	defaultLogger.SetOutput(w)
}

// Debug logs using the default logger
func Debug(format string, args ...interface{}) {
	defaultLogger.log(DEBUG, format, args...)
}

// Info logs using the default logger
func Info(format string, args ...interface{}) {
	defaultLogger.log(INFO, format, args...)
}

// Warn logs using the default logger
func Warn(format string, args ...interface{}) {
	defaultLogger.log(WARN, format, args...)
}

// Error logs using the default logger
func Error(format string, args ...interface{}) {
	defaultLogger.log(ERROR, format, args...)
}

// Fatal logs using the default logger and exits
func Fatal(format string, args ...interface{}) {
	defaultLogger.log(FATAL, format, args...)
}
