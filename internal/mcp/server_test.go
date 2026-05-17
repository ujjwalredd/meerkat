package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ujjwalredd/meerkat/internal/config"
)

func TestExplainMethod(t *testing.T) {
	p := config.Default()
	in := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"meerkat.explain","params":{"command":"git push origin main"}}` + "\n",
	)
	var out bytes.Buffer
	if err := Serve(in, &out, p); err != nil {
		t.Fatal(err)
	}
	var resp struct {
		ID     int `json:"id"`
		Result struct {
			Action    string `json:"decision"`
			RiskLevel string `json:"risk_level"`
		} `json:"result"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("invalid response: %v\n%s", err, out.String())
	}
	if resp.Result.Action != "BLOCK" {
		t.Errorf("want BLOCK got %s", resp.Result.Action)
	}
}

func TestUnknownMethod(t *testing.T) {
	p := config.Default()
	in := strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"nope"}` + "\n")
	var out bytes.Buffer
	_ = Serve(in, &out, p)
	if !strings.Contains(out.String(), "method not found") {
		t.Errorf("want method-not-found error, got %s", out.String())
	}
}
