package observability

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Level string

const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

// Artifact is a structured trace entry for a tool call.
type Artifact struct {
	ToolName string
	Input    string
	Output   string
}

// Logger writes structured log entries to stderr and a per-run log file.
type Logger struct {
	runID   string
	logPath string
	mu      sync.Mutex
	f       *os.File
}

type logEntry struct {
	Time    string `json:"t"`
	Level   Level  `json:"level"`
	Message string `json:"msg"`
}

func NewLogger(artifactsDir, runID string) *Logger {
	dir := filepath.Join(artifactsDir, runID)
	os.MkdirAll(dir, 0o755) //nolint:errcheck
	logPath := filepath.Join(dir, "run.log")
	f, _ := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	return &Logger{runID: runID, logPath: logPath, f: f}
}

func (l *Logger) log(level Level, msg string) {
	e := logEntry{Time: time.Now().UTC().Format(time.RFC3339), Level: level, Message: msg}
	line, _ := json.Marshal(e)

	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Fprintf(os.Stderr, "[%s] %s %s\n", level, e.Time, msg)
	if l.f != nil {
		l.f.Write(append(line, '\n')) //nolint:errcheck
	}
}

func (l *Logger) Debug(msg string) { l.log(LevelDebug, msg) }
func (l *Logger) Info(msg string)  { l.log(LevelInfo, msg) }
func (l *Logger) Warn(msg string)  { l.log(LevelWarn, msg) }
func (l *Logger) Error(msg string) { l.log(LevelError, msg) }

// Trace persists a tool call artifact to the run directory.
func (l *Logger) Trace(a Artifact) {
	dir := filepath.Dir(l.logPath)
	path := filepath.Join(dir, fmt.Sprintf("trace_%s_%d.json", a.ToolName, time.Now().UnixNano()))
	data, _ := json.MarshalIndent(a, "", "  ")
	os.WriteFile(path, data, 0o644) //nolint:errcheck
}
