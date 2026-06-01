package webbou

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

type LogEntry struct {
	Timestamp  string                 `json:"timestamp"`
	Level      string                 `json:"level"`
	Message    string                 `json:"message"`
	Component  string                 `json:"component,omitempty"`
	Fields     map[string]interface{} `json:"fields,omitempty"`
	StackTrace string                 `json:"stack_trace,omitempty"`
}

type Logger struct {
	mu         sync.Mutex
	level      LogLevel
	output     io.Writer
	formatter  string
	component  string
	jsonOutput bool
}

var (
	defaultLogger = &Logger{
		level:      INFO,
		output:     os.Stdout,
		formatter:  "json",
		jsonOutput: true,
	}
	loggerMutex sync.RWMutex
)

func NewLogger(component string, level LogLevel, jsonOutput bool) *Logger {
	return &Logger{
		level:      level,
		output:     os.Stdout,
		formatter:  "json",
		component:  component,
		jsonOutput: jsonOutput,
	}
}

func GetLogger() *Logger {
	loggerMutex.RLock()
	defer loggerMutex.RUnlock()
	return defaultLogger
}

func SetLogger(l *Logger) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	defaultLogger = l
}

func SetLogLevel(level LogLevel) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	defaultLogger.level = level
}

func (l *Logger) log(level LogLevel, message string, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level.String(),
		Message:   message,
		Component: l.component,
		Fields:    fields,
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.jsonOutput {
		data, err := json.Marshal(entry)
		if err == nil {
			l.output.Write(append(data, '\n'))
		}
	} else {
		fmt.Fprintf(l.output, "[%s] %s %s: %s\n",
			entry.Timestamp, entry.Level, l.component, message)
	}
}

func (l *Logger) Debug(message string, fields ...interface{}) {
	l.log(DEBUG, message, pairsToMap(fields...))
}

func (l *Logger) Info(message string, fields ...interface{}) {
	l.log(INFO, message, pairsToMap(fields...))
}

func (l *Logger) Warn(message string, fields ...interface{}) {
	l.log(WARN, message, pairsToMap(fields...))
}

func (l *Logger) Error(message string, fields ...interface{}) {
	l.log(ERROR, message, pairsToMap(fields...))
}

func (l *Logger) Fatal(message string, fields ...interface{}) {
	l.log(FATAL, message, pairsToMap(fields...))
	os.Exit(1)
}

func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	return &Logger{
		level:      l.level,
		output:     l.output,
		formatter:  l.formatter,
		component:  l.component,
		jsonOutput: l.jsonOutput,
	}
}

func pairsToMap(pairs ...interface{}) map[string]interface{} {
	if len(pairs)%2 != 0 {
		pairs = pairs[:len(pairs)-1]
	}

	result := make(map[string]interface{})
	for i := 0; i < len(pairs); i += 2 {
		if key, ok := pairs[i].(string); ok {
			result[key] = pairs[i+1]
		}
	}
	return result
}

func Debug(msg string, fields ...interface{}) {
	GetLogger().Debug(msg, fields...)
}

func Info(msg string, fields ...interface{}) {
	GetLogger().Info(msg, fields...)
}

func Warn(msg string, fields ...interface{}) {
	GetLogger().Warn(msg, fields...)
}

func Error(msg string, fields ...interface{}) {
	GetLogger().Error(msg, fields...)
}

func Fatal(msg string, fields ...interface{}) {
	GetLogger().Fatal(msg, fields...)
}