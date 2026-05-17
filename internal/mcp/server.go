// Package mcp implements a minimal Model Context Protocol server that
// exposes Meerkat's decision engine over JSON-RPC 2.0 on stdio. Agents
// that speak MCP can call:
//
//	meerkat.explain { command: string }       → decision JSON
//	meerkat.scan    { paths: [string] }       → findings (redacted)
//	meerkat.approve { command: string }       → ALLOW|DENY (interactive)
//
// Transport: line-delimited JSON-RPC 2.0, the same shape MCP uses over
// stdio. This is a pragmatic subset — the goal is interop with agents
// that already build MCP requests, not full MCP spec coverage.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/ujjwalredd/meerkat/internal/config"
	"github.com/ujjwalredd/meerkat/internal/decision"
	"github.com/ujjwalredd/meerkat/internal/scanner"
)

type rpcReq struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResp struct {
	JSONRPC string  `json:"jsonrpc"`
	ID      any     `json:"id"`
	Result  any     `json:"result,omitempty"`
	Error   *rpcErr `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Serve reads JSON-RPC requests from r, writes responses to w.
// Blocks until r is closed.
func Serve(r io.Reader, w io.Writer, p *config.Policy) error {
	enc := json.NewEncoder(w)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 8<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcReq
		if err := json.Unmarshal(line, &req); err != nil {
			_ = enc.Encode(rpcResp{JSONRPC: "2.0", Error: &rpcErr{Code: -32700, Message: "parse error"}})
			continue
		}
		resp := dispatch(req, p)
		_ = enc.Encode(resp)
	}
	return sc.Err()
}

func dispatch(req rpcReq, p *config.Policy) rpcResp {
	resp := rpcResp{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "meerkat.explain", "explain":
		var args struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(req.Params, &args); err != nil {
			resp.Error = &rpcErr{Code: -32602, Message: err.Error()}
			return resp
		}
		d, _ := decision.Decide(args.Command, p)
		resp.Result = d
	case "meerkat.scan", "scan":
		var args struct {
			Paths []string `json:"paths"`
		}
		_ = json.Unmarshal(req.Params, &args)
		if len(args.Paths) == 0 {
			args.Paths = []string{"."}
		}
		fs, _ := scanner.Scan(args.Paths, &p.Secrets, p.Project.Root)
		resp.Result = fs
	case "meerkat.approve", "approve":
		// MVP MCP: approval is delegated back to the caller via a
		// well-defined decision payload. The agent uses the returned
		// reasons to decide whether to surface a prompt to the user.
		var args struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(req.Params, &args); err != nil {
			resp.Error = &rpcErr{Code: -32602, Message: err.Error()}
			return resp
		}
		d, _ := decision.Decide(args.Command, p)
		resp.Result = map[string]any{
			"decision": d.Action,
			"risk":     d.RiskLevel,
			"reasons":  d.Reasons,
			"note":     "Agent must surface this to the user; meerkat never auto-approves through MCP.",
		}
	default:
		resp.Error = &rpcErr{Code: -32601, Message: "method not found: " + req.Method}
	}
	return resp
}

// Run convenience wraps Serve on os.Stdin/os.Stdout.
func Run(p *config.Policy) error {
	fmt.Fprintln(os.Stderr, "meerkat mcp: listening on stdio (JSON-RPC 2.0)")
	return Serve(os.Stdin, os.Stdout, p)
}
