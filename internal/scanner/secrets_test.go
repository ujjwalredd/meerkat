package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ujjwalredd/meerkat/internal/config"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestScanDetects(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "a.go", `var k = "AKIAABCDEFGHIJKLMNOP"`)
	writeFile(t, d, "b.env", "OPENAI_API_KEY=sk-abcdefghijklmnopqrstuvwx")
	writeFile(t, d, "c.txt", "no secrets here")
	cfg := config.Default().Secrets
	findings, _ := Scan([]string{d}, &cfg, d)
	if len(findings) < 2 {
		t.Fatalf("want >=2 findings got %d (%v)", len(findings), findings)
	}
	for _, f := range findings {
		if strings.Contains(f.Redact, "AKIAABCDEFGHIJKL") {
			t.Errorf("redact leaked full secret: %s", f.Redact)
		}
	}
}

func TestScanIgnoresBlocked(t *testing.T) {
	d := t.TempDir()
	sub := filepath.Join(d, "node_modules")
	os.MkdirAll(sub, 0o755)
	writeFile(t, sub, "x.js", `key="sk-ABCDEFGHIJKLMNOPQRSTUVWX"`)
	cfg := config.Default().Secrets
	cfg.IgnorePaths = []string{"./node_modules"}
	findings, _ := Scan([]string{d}, &cfg, d)
	if len(findings) != 0 {
		t.Errorf("want 0 findings, got %d", len(findings))
	}
}

func TestScanPatternsLimitRules(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "a.env", "OPENAI_API_KEY=sk-abcdefghijklmnopqrstuvwx")
	cfg := config.Default().Secrets
	cfg.ScanPatterns = []string{"aws_access_key"}
	findings, _ := Scan([]string{d}, &cfg, d)
	if len(findings) != 0 {
		t.Fatalf("scan_patterns should limit enabled rules, got %#v", findings)
	}
}

func TestScanDisabled(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "a.env", "OPENAI_API_KEY=sk-abcdefghijklmnopqrstuvwx")
	cfg := config.Default().Secrets
	cfg.Enabled = false
	findings, _ := Scan([]string{d}, &cfg, d)
	if len(findings) != 0 {
		t.Fatalf("disabled scanner should return no findings, got %#v", findings)
	}
}

func TestRedact(t *testing.T) {
	if r := Redact("AKIA1234567890ABCDEF"); strings.Contains(r, "1234567890") {
		t.Errorf("redact leaked middle: %s", r)
	}
}
