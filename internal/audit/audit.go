package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ujjwalredd/meerkat/internal/config"
)

type Event struct {
	Timestamp        string                 `json:"timestamp"`
	Type             string                 `json:"event_type"`
	Command          string                 `json:"command,omitempty"`
	WorkingDir       string                 `json:"working_directory,omitempty"`
	Decision         string                 `json:"decision,omitempty"`
	RiskLevel        string                 `json:"risk_level,omitempty"`
	Reasons          []string               `json:"reasons,omitempty"`
	PolicyFile       string                 `json:"policy_file,omitempty"`
	GitBranch        string                 `json:"git_branch,omitempty"`
	ChangedFiles     int                    `json:"changed_files_count,omitempty"`
	SecretScanResult string                 `json:"secret_scan_result,omitempty"`
	KeepAwakeStatus  string                 `json:"keep_awake_status,omitempty"`
	ExitCode         int                    `json:"exit_code,omitempty"`
	DurationMs       int64                  `json:"duration_ms,omitempty"`
	Extra            map[string]interface{} `json:"extra,omitempty"`
}

type Logger struct {
	mu   sync.Mutex
	f    *os.File
	path string
	cfg  *config.AuditCfg
}

func New(cfg *config.AuditCfg) (*Logger, error) {
	if !cfg.Enabled {
		return &Logger{cfg: cfg}, nil
	}
	dir := cfg.LogDir
	if dir == "" {
		dir = "./.meerkat/logs"
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("audit: create log dir %s: %w", dir, err)
	}
	name := fmt.Sprintf("meerkat-%s.jsonl", time.Now().UTC().Format("20060102-150405"))
	p := filepath.Join(dir, name)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, fmt.Errorf("audit: open log %s: %w", p, err)
	}
	return &Logger{f: f, path: p, cfg: cfg}, nil
}

func (l *Logger) Path() string { return l.path }

func (l *Logger) Log(e Event) {
	if l == nil || l.f == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if e.Timestamp == "" {
		e.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	l.f.Write(b)
	l.f.Write([]byte("\n"))
}

func (l *Logger) Close() error {
	if l == nil || l.f == nil {
		return nil
	}
	return l.f.Close()
}
