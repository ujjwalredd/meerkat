package plugins

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/ujjwalredd/meerkat/internal/scanner"
)

type Trufflehog struct{}

func (Trufflehog) Name() string    { return "trufflehog" }
func (Trufflehog) Available() bool { return HasBinary("trufflehog") }

type thResult struct {
	DetectorName string `json:"DetectorName"`
	Raw          string `json:"Raw"`
	Verified     bool   `json:"Verified"`
	SourceMeta   struct {
		Data struct {
			Filesystem struct {
				File string `json:"file"`
				Line int    `json:"line"`
			} `json:"Filesystem"`
		} `json:"Data"`
	} `json:"SourceMetadata"`
}

func (Trufflehog) Scan(ctx context.Context, paths []string) ([]scanner.Finding, error) {
	if !HasBinary("trufflehog") {
		return nil, fmt.Errorf("trufflehog: not installed")
	}
	out := []scanner.Finding{}
	for _, p := range paths {
		var buf bytes.Buffer
		cmd := exec.CommandContext(ctx, "trufflehog", "filesystem", p, "--json", "--no-update")
		cmd.Stdout = &buf
		_ = cmd.Run()
		sc := bufio.NewScanner(&buf)
		sc.Buffer(make([]byte, 1<<20), 4<<20)
		for sc.Scan() {
			var r thResult
			if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
				continue
			}
			out = append(out, scanner.Finding{
				File:   r.SourceMeta.Data.Filesystem.File,
				Line:   r.SourceMeta.Data.Filesystem.Line,
				Type:   "trufflehog:" + r.DetectorName,
				Redact: scanner.Redact(r.Raw),
			})
		}
	}
	return out, nil
}
