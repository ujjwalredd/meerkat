package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ujjwalredd/meerkat/internal/audit"
	"github.com/ujjwalredd/meerkat/internal/awake"
	"github.com/ujjwalredd/meerkat/internal/config"
	"github.com/ujjwalredd/meerkat/internal/decision"
	"github.com/ujjwalredd/meerkat/internal/filesystem"
	"github.com/ujjwalredd/meerkat/internal/gitguard"
	"github.com/ujjwalredd/meerkat/internal/hook"
	"github.com/ujjwalredd/meerkat/internal/mcp"
	"github.com/ujjwalredd/meerkat/internal/processrunner"
	"github.com/ujjwalredd/meerkat/internal/sandbox"
	"github.com/ujjwalredd/meerkat/internal/sandbox/egress"
	"github.com/ujjwalredd/meerkat/internal/scanner"
	"github.com/ujjwalredd/meerkat/internal/ui"

	// register sandbox backends (build-tagged per OS)
	_ "github.com/ujjwalredd/meerkat/internal/sandbox/bwrap"
	_ "github.com/ujjwalredd/meerkat/internal/sandbox/jobobject"
	_ "github.com/ujjwalredd/meerkat/internal/sandbox/seatbelt"
	_ "github.com/ujjwalredd/meerkat/internal/sandbox/wsl2"
)

var Version = "0.4.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "init":
		os.Exit(cmdInit(os.Args[2:]))
	case "run":
		os.Exit(cmdRun(os.Args[2:]))
	case "scan":
		os.Exit(cmdScan(os.Args[2:]))
	case "status":
		os.Exit(cmdStatus(os.Args[2:]))
	case "doctor":
		os.Exit(cmdDoctor(os.Args[2:]))
	case "policy":
		os.Exit(cmdPolicy(os.Args[2:]))
	case "explain":
		os.Exit(cmdExplain(os.Args[2:]))
	case "sandbox":
		os.Exit(cmdSandbox(os.Args[2:]))
	case "mcp":
		os.Exit(cmdMCP(os.Args[2:]))
	case "hook":
		os.Exit(cmdHook(os.Args[2:]))
	case "claude":
		os.Exit(cmdClaude(os.Args[2:]))
	case "version", "--version", "-v":
		fmt.Println("meerkat", Version)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `MeerKat - vibe coding without blind trust.

Usage:
  meerkat init [wizard] [--profile P]       create starter meerkat.yml
                                            (profiles: basic|strict|agent|node|python)
  meerkat run [--policy F] [--keep-awake]
              [--dry-run] -- <cmd> [args]   run command under MeerKat
  meerkat scan [--policy F] [paths...]      secret + policy scan
  meerkat status [--policy F]               show runtime status
  meerkat doctor                            check system compatibility
  meerkat policy validate [--policy F]      validate meerkat.yml
  meerkat explain -- <cmd> [args]           explain decision without running
  meerkat sandbox doctor                    list available isolation backends
  meerkat sandbox profile [--policy F]      show generated sandbox profile
  meerkat mcp [start] [--policy F]          run JSON-RPC MCP server on stdio
  meerkat claude install                    install /meerkat slash cmd + hooks
                                            in ~/.claude (auto keep-awake +
                                            auto-approve via PreToolUse hook)
  meerkat hook pretooluse|sessionstart|stop run a Claude Code hook (stdin JSON)
  meerkat version                           print version`)
}

func loadPolicy(path string) *config.Policy {
	p, err := config.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	return p
}

func cmdInit(args []string) int {
	// Accept `meerkat init wizard` (interactive) and `meerkat init` (defaults).
	wizard := false
	if len(args) > 0 && args[0] == "wizard" {
		wizard = true
		args = args[1:]
	}
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	out := fs.String("o", "meerkat.yml", "output file")
	force := fs.Bool("force", false, "overwrite existing file")
	profile := fs.String("profile", "basic", "starter profile: basic|strict|agent|node|python")
	fs.Parse(args)
	if _, err := os.Stat(*out); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "%s already exists (use --force to overwrite)\n", *out)
		return 1
	}
	p := config.Default()
	if wizard {
		runWizard(p)
	} else {
		applyProfile(p, *profile)
	}
	if err := config.Save(p, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("Wrote %s policy to %s\n", *profile, *out)
	fmt.Println("Next:")
	fmt.Println("  meerkat run --keep-awake -- <your command>")
	fmt.Println("  meerkat explain -- git push origin main")
	fmt.Println("  meerkat doctor")
	return 0
}

// applyProfile tunes the default policy for a starter profile.
func applyProfile(p *config.Policy, name string) {
	switch name {
	case "strict":
		p.Mode.DefaultAction = "block"
		p.Mode.AutoApproveSafe = false
		p.Cmds.AutoApprove = nil
	case "agent":
		addUnique(&p.Cmds.AutoApprove, "claude", "codex", "aider", "goose")
	case "node":
		addUnique(&p.Cmds.AutoApprove, "node", "npx", "tsc", "vitest", "jest")
	case "python":
		addUnique(&p.Cmds.AutoApprove, "python", "python3", "pytest -q", "ruff", "mypy")
	case "basic":
	}
}

func addUnique(list *[]string, items ...string) {
	seen := map[string]bool{}
	for _, e := range *list {
		seen[e] = true
	}
	for _, it := range items {
		if !seen[it] {
			*list = append(*list, it)
			seen[it] = true
		}
	}
}

// runWizard asks a few questions on stdin to tailor the policy.
func runWizard(p *config.Policy) {
	in := bufio.NewReader(os.Stdin)
	ask := func(prompt, def string) string {
		fmt.Printf("%s [%s]: ", prompt, def)
		line, _ := in.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			return def
		}
		return line
	}
	fmt.Println("Meerkat setup wizard")
	fmt.Println("--------------------")
	p.Project.Name = ask("Project name", p.Project.Name)
	prof := ask("Profile (basic|strict|agent|node|python)", "basic")
	applyProfile(p, prof)
	agents := ask("Auto-approve AI agents? (claude,codex,aider,goose) [Y/n]", "Y")
	if strings.HasPrefix(strings.ToLower(agents), "y") {
		addUnique(&p.Cmds.AutoApprove, "claude", "codex", "aider", "goose")
	}
	if strings.HasPrefix(strings.ToLower(ask("Enable sandbox? [y/N]", "N")), "y") {
		p.Sandbox.Enabled = true
		p.Sandbox.Backend = "auto"
		p.Sandbox.FailClosed = false
	}
	if strings.HasPrefix(strings.ToLower(ask("Enable egress proxy? [y/N]", "N")), "y") {
		p.Sandbox.Egress.Mode = "proxy"
	}
}

// argsAfterDoubleDash extracts argv after `--`.
func argsAfterDoubleDash(args []string) (before, after []string) {
	for i, a := range args {
		if a == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

func cmdRun(args []string) int {
	before, after := argsAfterDoubleDash(args)
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	policyPath := fs.String("policy", "meerkat.yml", "policy file")
	keepAwake := fs.Bool("keep-awake", false, "force keep-awake on")
	dryRun := fs.Bool("dry-run", false, "evaluate decision, do not execute")
	sandboxFlag := fs.String("sandbox", "", "sandbox backend: auto|off|seatbelt|bwrap|jobobject|wsl2")
	fs.Parse(before)
	if len(after) == 0 {
		fmt.Fprintln(os.Stderr, "missing command after --")
		return 2
	}
	p := loadPolicy(*policyPath)
	if *dryRun {
		p.Mode.DryRun = true
	}
	if *keepAwake {
		p.Awake.Enabled = true
	}

	log, err := audit.New(&p.Audit)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer log.Close()

	cwd, _ := os.Getwd()
	cmdLine := strings.Join(after, " ")
	branch := ""
	if gitguard.IsRepo() {
		branch = gitguard.CurrentBranch()
	}

	log.Log(audit.Event{Type: "run_started", Command: cmdLine, WorkingDir: cwd,
		PolicyFile: p.Path, GitBranch: branch})

	d, class := decision.Decide(cmdLine, p)
	log.Log(audit.Event{Type: "command_classified", Command: cmdLine,
		Decision: string(d.Action), RiskLevel: d.RiskLevel, Reasons: d.Reasons})

	// Pre-flight: git commit/push secret scan
	if class.GitOp == "commit" && p.Git.RequireSecretScanBeforeCommit && p.Secrets.Enabled {
		if blocked := preScan(p, log, "commit"); blocked {
			return 3
		}
	}
	if class.GitOp == "push" && p.Git.RequireSecretScanBeforePush && p.Secrets.Enabled {
		if blocked := preScan(p, log, "push"); blocked {
			return 3
		}
	}

	if p.Mode.DryRun {
		printDecision(cmdLine, d)
		log.Log(audit.Event{Type: "run_finished", Command: cmdLine, Decision: "DRY_RUN",
			RiskLevel: d.RiskLevel, ExitCode: 0})
		return 0
	}

	switch d.Action {
	case decision.Block:
		fmt.Fprintln(os.Stderr, "BLOCKED:", cmdLine)
		for _, r := range d.Reasons {
			fmt.Fprintln(os.Stderr, "  - "+r)
		}
		log.Log(audit.Event{Type: "policy_violation", Command: cmdLine,
			Decision: "BLOCK", RiskLevel: d.RiskLevel, Reasons: d.Reasons})
		return 4
	case decision.Ask:
		log.Log(audit.Event{Type: "approval_requested", Command: cmdLine,
			RiskLevel: d.RiskLevel, Reasons: d.Reasons})
		choice := ui.Ask(ui.PromptCtx{
			Command: cmdLine, Cwd: cwd, Branch: branch,
			Decision: d, Class: class,
			TimeoutS: p.Approve.TimeoutSeconds, OnTimeout: p.Approve.DefaultOnTimeout,
		})
		if choice == ui.ChoiceDeny || choice == ui.ChoiceAlwaysDeny {
			log.Log(audit.Event{Type: "approval_denied", Command: cmdLine})
			fmt.Fprintln(os.Stderr, "denied")
			return 5
		}
		log.Log(audit.Event{Type: "approval_granted", Command: cmdLine})
	}

	// Start keep-awake
	var keeper *awake.Keeper
	if p.Awake.Enabled && p.Awake.Mode != "disabled" {
		dur := time.Duration(p.Awake.MaxDurationMinutes) * time.Minute
		k, err := awake.Start(dur)
		if err != nil {
			fmt.Fprintln(os.Stderr, "[meerkat] keep-awake unavailable:", err)
			log.Log(audit.Event{Type: "keep_awake_started", KeepAwakeStatus: "unavailable",
				Reasons: []string{err.Error()}})
		} else {
			keeper = k
			log.Log(audit.Event{Type: "keep_awake_started", KeepAwakeStatus: k.Backend()})
			fmt.Fprintf(os.Stderr, "[meerkat] keep-awake: %s\n", k.Backend())
		}
	}
	defer func() {
		if keeper != nil {
			keeper.Stop()
			log.Log(audit.Event{Type: "keep_awake_stopped", KeepAwakeStatus: keeper.Backend()})
		}
	}()

	// Resolve sandbox backend: CLI flag overrides policy.
	sbName := *sandboxFlag
	if sbName == "" {
		if p.Sandbox.Enabled {
			sbName = p.Sandbox.Backend
			if sbName == "" {
				sbName = "auto"
			}
		} else {
			sbName = "off"
		}
	}
	sb, sbErr := sandbox.Select(sbName, p.Sandbox.FailClosed)
	if sbErr != nil {
		fmt.Fprintln(os.Stderr, "[meerkat] sandbox:", sbErr)
		log.Log(audit.Event{Type: "policy_violation", Reasons: []string{sbErr.Error()}})
		return 4
	}
	wrapped := after
	var sbCleanup sandbox.Cleanup
	if sb != nil {
		w, cl, err := sb.Wrap(after, p)
		if err != nil {
			fmt.Fprintln(os.Stderr, "[meerkat] sandbox wrap:", err)
			log.Log(audit.Event{Type: "policy_violation", Reasons: []string{err.Error()}})
			return 4
		}
		wrapped = w
		sbCleanup = cl
		log.Log(audit.Event{Type: "sandbox_started",
			Extra: map[string]any{"backend": sb.Name(), "wrapped_argv0": w[0]}})
		fmt.Fprintf(os.Stderr, "[meerkat] sandbox: %s\n", sb.Name())
	}
	defer func() {
		if sbCleanup != nil {
			sbCleanup()
		}
	}()

	// Optional egress proxy.
	var prox *egress.Proxy
	if p.Sandbox.Egress.Mode == "proxy" {
		pr, err := egress.Start(&p.Net, log)
		if err != nil {
			fmt.Fprintln(os.Stderr, "[meerkat] egress proxy:", err)
		} else {
			prox = pr
			os.Setenv("HTTP_PROXY", "http://"+pr.Addr())
			os.Setenv("HTTPS_PROXY", "http://"+pr.Addr())
			fmt.Fprintf(os.Stderr, "[meerkat] egress proxy: %s\n", pr.Addr())
		}
	}
	defer func() {
		if prox != nil {
			prox.Close()
		}
	}()

	log.Log(audit.Event{Type: "command_started", Command: cmdLine})
	fmt.Fprintf(os.Stderr, "[meerkat] running: %s\n\n", cmdLine)
	res := processrunner.Run(wrapped)
	log.Log(audit.Event{Type: "command_finished", Command: cmdLine,
		ExitCode: res.ExitCode, DurationMs: res.DurationMs})

	// Post-run scope check
	if gitguard.IsRepo() {
		if changed, err := gitguard.ChangedFiles(); err == nil {
			oos := outOfScope(changed, p)
			if len(oos) > 0 {
				fmt.Fprintln(os.Stderr, "[meerkat] WARNING: out-of-scope file changes:")
				for _, f := range oos {
					fmt.Fprintln(os.Stderr, "  - "+f)
				}
				log.Log(audit.Event{Type: "policy_violation", Reasons: append([]string{"out-of-scope writes"}, oos...)})
				if p.Mode.DenyOutOfScope {
					res.ExitCode = 4
				}
			}
		}
	}

	log.Log(audit.Event{Type: "run_finished", Command: cmdLine,
		ExitCode: res.ExitCode, DurationMs: res.DurationMs})
	fmt.Fprintf(os.Stderr, "[meerkat] exit=%d duration=%dms log=%s\n",
		res.ExitCode, res.DurationMs, log.Path())
	return res.ExitCode
}

func preScan(p *config.Policy, log *audit.Logger, op string) bool {
	log.Log(audit.Event{Type: "secret_scan_started", Extra: map[string]any{"op": op}})
	files, _ := gitguard.StagedFiles()
	if op == "push" {
		// scan all changed too
		if extra, err := gitguard.ChangedFiles(); err == nil {
			files = append(files, extra...)
		}
	}
	if len(files) == 0 {
		files = []string{"."}
	}
	findings, _ := scanner.Scan(files, &p.Secrets, p.Project.Root)
	if len(findings) == 0 {
		log.Log(audit.Event{Type: "secret_scan_started", SecretScanResult: "clean"})
		return false
	}
	fmt.Fprintln(os.Stderr, "BLOCKED: Secret-like values detected")
	fmt.Fprintln(os.Stderr, "")
	for _, f := range findings {
		fmt.Fprintf(os.Stderr, "File: %s\nLine: %d\nType: %s\nAction: remove the secret and use environment variables instead\n\n",
			f.File, f.Line, f.Type)
		log.Log(audit.Event{Type: "secret_detected",
			Extra: map[string]any{"file": f.File, "line": f.Line, "type": f.Type, "redacted": f.Redact}})
	}
	log.Log(audit.Event{Type: "git_guard_triggered",
		Reasons: []string{"secret scan failed before " + op}})
	return true
}

func outOfScope(files []string, p *config.Policy) []string {
	root, _ := filesystem.Resolve(p.Project.Root)
	if root == "" || p.Project.Root == "" {
		root, _ = filesystem.Resolve(".")
	}
	base := root
	if repoRoot := gitguard.RepoRoot(); repoRoot != "" {
		if resolved, err := filesystem.Resolve(repoRoot); err == nil {
			base = resolved
		}
	}
	var oos []string
	for _, f := range files {
		target := filesystem.ExpandTilde(f)
		if !filepath.IsAbs(target) {
			target = filepath.Join(base, target)
		}
		abs, _ := filesystem.Resolve(target)
		ok := false
		for _, w := range p.FS.AllowedWritePaths {
			we := filesystem.ExpandTilde(w)
			if !filepath.IsAbs(we) {
				we = filepath.Join(root, we)
			}
			we, _ = filesystem.Resolve(we)
			if filesystem.Inside(we, abs) {
				ok = true
				break
			}
		}
		if !ok {
			oos = append(oos, f)
		}
	}
	return oos
}

func printDecision(cmd string, d decision.Decision) {
	fmt.Printf("Command: %s\nDecision: %s\nRisk: %s\nReasons:\n", cmd, d.Action, d.RiskLevel)
	for _, r := range d.Reasons {
		fmt.Println("  - " + r)
	}
}

func cmdScan(args []string) int {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	policyPath := fs.String("policy", "meerkat.yml", "policy file")
	jsonOut := fs.Bool("json", false, "JSON output")
	fs.Parse(args)
	p := loadPolicy(*policyPath)
	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	findings, err := scanner.Scan(paths, &p.Secrets, p.Project.Root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *jsonOut {
		b, _ := json.MarshalIndent(findings, "", "  ")
		os.Stdout.Write(b)
		os.Stdout.Write([]byte("\n"))
	} else {
		if len(findings) == 0 {
			fmt.Println("No secrets detected.")
			return 0
		}
		fmt.Printf("Found %d secret-like values:\n\n", len(findings))
		for _, f := range findings {
			fmt.Printf("  %s:%d  %s  (%s)\n", f.File, f.Line, f.Type, f.Redact)
		}
	}
	if len(findings) > 0 {
		return 3
	}
	return 0
}

func cmdStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	policyPath := fs.String("policy", "meerkat.yml", "policy file")
	fs.Parse(args)
	p, err := config.Load(*policyPath)
	cwd, _ := os.Getwd()
	fmt.Println("MeerKat status")
	fmt.Println("  workspace:    ", cwd)
	if err != nil {
		fmt.Println("  policy:        (missing)", err)
	} else {
		fmt.Println("  policy:       ", p.Path)
		backend, ok := awake.BackendAvailable()
		if ok && p.Awake.Enabled {
			fmt.Println("  keep-awake:    available (" + backend + ")")
		} else {
			fmt.Println("  keep-awake:    disabled / unavailable")
		}
	}
	if gitguard.IsRepo() {
		b := gitguard.CurrentBranch()
		fmt.Println("  git branch:   ", b)
		if p != nil {
			protected := false
			for _, pb := range p.Git.ProtectedBranches {
				if pb == b {
					protected = true
					break
				}
			}
			fmt.Println("  protected:    ", protected)
		}
		if changed, err := gitguard.ChangedFiles(); err == nil {
			fmt.Println("  changed files:", len(changed))
		}
	} else {
		fmt.Println("  git:           not a repo")
	}
	return 0
}

func cmdDoctor(args []string) int {
	fmt.Println("MeerKat doctor")
	fmt.Println("  OS:           ", runtime.GOOS, "/", runtime.GOARCH)
	check("git in PATH", gitguard.IsRepo() || hasBin("git"))
	backend, ok := awake.BackendAvailable()
	if ok {
		fmt.Printf("  [OK]    keep-awake backend: %s\n", backend)
	} else {
		fmt.Println("  [WARN]  no keep-awake backend on this platform")
	}
	fmt.Println("  [OK]    built-in secret scanner")
	if _, err := config.Load("meerkat.yml"); err == nil {
		fmt.Println("  [OK]    meerkat.yml present and valid")
	} else {
		fmt.Println("  [WARN]  meerkat.yml missing or invalid (run: meerkat init)")
	}
	return 0
}

func check(label string, ok bool) {
	if ok {
		fmt.Println("  [OK]   ", label)
	} else {
		fmt.Println("  [FAIL] ", label)
	}
}

func hasBin(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func cmdPolicy(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: meerkat policy validate [--policy F]")
		return 2
	}
	switch args[0] {
	case "validate":
		fs := flag.NewFlagSet("validate", flag.ExitOnError)
		path := fs.String("policy", "meerkat.yml", "policy file")
		fs.Parse(args[1:])
		_, err := config.Load(*path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("OK")
		return 0
	}
	return 2
}

func cmdExplain(args []string) int {
	_, after := argsAfterDoubleDash(args)
	if len(after) == 0 {
		// allow `explain "git push origin main"`
		after = args
	}
	if len(after) == 0 {
		fmt.Fprintln(os.Stderr, "usage: meerkat explain -- <command>")
		return 2
	}
	p, err := config.Load("meerkat.yml")
	if err != nil {
		p = config.Default()
	}
	cmdLine := strings.Join(after, " ")
	// If single quoted arg given, use as-is
	if len(after) == 1 {
		cmdLine = after[0]
	}
	d, _ := decision.Decide(cmdLine, p)
	printDecision(cmdLine, d)
	switch d.Action {
	case decision.Block:
		return 4
	case decision.Ask:
		return 6
	}
	return 0
}

func cmdSandbox(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: meerkat sandbox doctor|profile [--policy F] [--backend NAME]")
		return 2
	}
	switch args[0] {
	case "doctor":
		fmt.Println("Sandbox backends")
		fmt.Println("  OS:", runtime.GOOS)
		for _, name := range sandbox.List() {
			b, _ := sandbox.Get(name)
			status := "unavailable"
			if b.Available() {
				status = "available"
			}
			fmt.Printf("  %-13s %s\n", name, status)
		}
		auto := sandbox.Auto()
		if auto != nil {
			fmt.Println("  auto picks:", auto.Name())
		} else {
			fmt.Println("  auto picks: (none available)")
		}
		return 0
	case "profile":
		fs := flag.NewFlagSet("profile", flag.ExitOnError)
		policyPath := fs.String("policy", "meerkat.yml", "policy file")
		backend := fs.String("backend", "auto", "backend name")
		fs.Parse(args[1:])
		p := loadPolicy(*policyPath)
		b, err := sandbox.Select(*backend, false)
		if err != nil || b == nil {
			fmt.Fprintln(os.Stderr, "no backend available:", err)
			return 1
		}
		// Show the wrapped argv for a representative command.
		w, cl, err := b.Wrap([]string{"echo", "hello"}, p)
		if cl != nil {
			defer cl()
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("backend:", b.Name())
		fmt.Println("wrapped argv:")
		for _, a := range w {
			fmt.Println("  ", a)
		}
		return 0
	}
	fmt.Fprintln(os.Stderr, "unknown sandbox subcommand:", args[0])
	return 2
}

func cmdMCP(args []string) int {
	// Accept canonical `meerkat mcp start` and bare `meerkat mcp`.
	if len(args) > 0 && args[0] == "start" {
		args = args[1:]
	}
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	policyPath := fs.String("policy", "meerkat.yml", "policy file")
	fs.Parse(args)
	p, err := config.Load(*policyPath)
	if err != nil {
		p = config.Default()
	}
	if err := mcp.Run(p); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func cmdHook(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: meerkat hook pretooluse|sessionstart|stop [--policy F]")
		return 2
	}
	event := args[0]
	fs := flag.NewFlagSet("hook", flag.ExitOnError)
	policyPath := fs.String("policy", "meerkat.yml", "policy file (relative to cwd; falls back to ~/.meerkat/meerkat.yml then defaults)")
	fs.Parse(args[1:])

	p, err := config.Load(*policyPath)
	if err != nil {
		home, _ := os.UserHomeDir()
		p2, err2 := config.Load(filepath.Join(home, ".meerkat", "meerkat.yml"))
		if err2 == nil {
			p = p2
		} else {
			p = config.Default()
		}
	}

	switch event {
	case "pretooluse":
		if err := hook.PreToolUse(os.Stdin, os.Stdout, p); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	case "sessionstart":
		if err := hook.SessionStart(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	case "stop", "sessionend":
		if err := hook.Stop(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown hook event:", event)
		return 2
	}
	return 0
}

func cmdClaude(args []string) int {
	if len(args) == 0 || (args[0] != "install" && args[0] != "uninstall") {
		fmt.Fprintln(os.Stderr, "usage: meerkat claude install|uninstall")
		return 2
	}
	if args[0] == "uninstall" {
		return claudeUninstall()
	}
	return claudeInstall()
}

func claudeInstall() int {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	binPath, _ := os.Executable()
	if binPath == "" {
		binPath = "meerkat"
	}
	// 1) /meerkat slash command
	cmdDir := filepath.Join(home, ".claude", "commands")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	slashPath := filepath.Join(cmdDir, "meerkat.md")
	slashBody := `---
description: Run a Claude task under Meerkat policy (auto keep-awake + safe auto-approval).
argument-hint: <your prompt>
---

Run the following task. Meerkat hooks (PreToolUse, SessionStart, Stop) are
active in this session; safe shell commands are auto-approved per the
project's ` + "`meerkat.yml`" + ` policy, dangerous commands are blocked, and the
machine is kept awake while you work. Treat the policy as authoritative —
do not attempt to bypass it.

Task: $ARGUMENTS
`
	if err := os.WriteFile(slashPath, []byte(slashBody), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("wrote", slashPath)

	// 2) merge hooks into ~/.claude/settings.json
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	settings := map[string]any{}
	if b, err := os.ReadFile(settingsPath); err == nil {
		_ = json.Unmarshal(b, &settings)
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	cmdStr := func(event string) string {
		return fmt.Sprintf("%q hook %s", binPath, event)
	}
	installClaudeHook(hooks, claudeHookSpec{
		EventKey: "PreToolUse",
		HookArg:  "pretooluse",
		// Match Bash AND file-mutating tools so Meerkat governs both
		// shell commands and edits to disk.
		Matcher: "Bash|Write|Edit|MultiEdit|NotebookEdit|Read",
		Command: cmdStr("pretooluse"),
	})
	installClaudeHook(hooks, claudeHookSpec{
		EventKey: "SessionStart",
		HookArg:  "sessionstart",
		Command:  cmdStr("sessionstart"),
	})
	installClaudeHook(hooks, claudeHookSpec{
		EventKey: "Stop",
		HookArg:  "stop",
		Command:  cmdStr("stop"),
	})
	settings["hooks"] = hooks
	out, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsPath, out, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("wired hooks in", settingsPath)

	// 3) suggest MCP wiring
	fmt.Println()
	fmt.Println("Next:")
	fmt.Println("  claude mcp add meerkat -- meerkat mcp start    # optional MCP server")
	fmt.Println("  (cd into a project, run: meerkat init)")
	fmt.Println("  Then open Claude Code and type:  /meerkat fix the auth bug")
	return 0
}

func claudeUninstall() int {
	home, _ := os.UserHomeDir()
	_ = os.Remove(filepath.Join(home, ".claude", "commands", "meerkat.md"))
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	b, err := os.ReadFile(settingsPath)
	if err == nil {
		var s map[string]any
		if json.Unmarshal(b, &s) == nil {
			if hooks, ok := s["hooks"].(map[string]any); ok {
				removeClaudeHook(hooks, "PreToolUse", "pretooluse")
				removeClaudeHook(hooks, "SessionStart", "sessionstart")
				removeClaudeHook(hooks, "Stop", "stop")
				if len(hooks) == 0 {
					delete(s, "hooks")
				} else {
					s["hooks"] = hooks
				}
			}
			out, _ := json.MarshalIndent(s, "", "  ")
			_ = os.WriteFile(settingsPath, out, 0o644)
		}
	}
	fmt.Println("removed /meerkat slash command + Meerkat hooks from ~/.claude")
	return 0
}

type claudeHookSpec struct {
	EventKey string
	HookArg  string
	Matcher  string
	Command  string
}

func installClaudeHook(hooks map[string]any, spec claudeHookSpec) {
	entry := map[string]any{
		"hooks": []any{map[string]any{
			"type":    "command",
			"command": spec.Command,
			"timeout": 10,
		}},
	}
	if spec.Matcher != "" {
		entry["matcher"] = spec.Matcher
	}
	existing := hookEntries(hooks[spec.EventKey])
	hooks[spec.EventKey] = append(removeMeerkatHookEntries(existing, spec.HookArg), entry)
}

func removeClaudeHook(hooks map[string]any, eventKey, hookArg string) {
	filtered := removeMeerkatHookEntries(hookEntries(hooks[eventKey]), hookArg)
	if len(filtered) == 0 {
		delete(hooks, eventKey)
		return
	}
	hooks[eventKey] = filtered
}

func hookEntries(v any) []any {
	if v == nil {
		return nil
	}
	if entries, ok := v.([]any); ok {
		return entries
	}
	return []any{v}
}

func removeMeerkatHookEntries(entries []any, hookArg string) []any {
	filtered := make([]any, 0, len(entries))
	for _, entry := range entries {
		if !isMeerkatHookEntry(entry, hookArg) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func isMeerkatHookEntry(entry any, hookArg string) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	hooks, ok := m["hooks"].([]any)
	if !ok {
		return false
	}
	for _, h := range hooks {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		cmd, _ := hm["command"].(string)
		if isMeerkatHookCommand(cmd, hookArg) {
			return true
		}
	}
	return false
}

func isMeerkatHookCommand(cmd, hookArg string) bool {
	cmd = strings.ToLower(cmd)
	hookArg = strings.ToLower(hookArg)
	return strings.Contains(cmd, "meerkat") && strings.Contains(cmd, " hook "+hookArg)
}
