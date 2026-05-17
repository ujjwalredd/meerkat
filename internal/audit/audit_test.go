package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/ujjwalredd/meerkat/internal/config"
)

func TestLoggerJSONL(t *testing.T) {
	d := t.TempDir()
	cfg := &config.AuditCfg{Enabled: true, LogDir: d, Format: "jsonl", RedactSecrets: true}
	l, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	l.Log(Event{Type: "run_started", Command: "npm test"})
	l.Log(Event{Type: "run_finished", ExitCode: 0})
	l.Close()
	f, err := os.Open(l.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		n++
		var e Event
		if err := json.Unmarshal([]byte(sc.Text()), &e); err != nil {
			t.Errorf("invalid jsonl: %v", err)
		}
		if strings.Contains(sc.Text(), "AKIA") {
			t.Errorf("secret leaked: %s", sc.Text())
		}
	}
	if n != 2 {
		t.Errorf("want 2 lines got %d", n)
	}
}
