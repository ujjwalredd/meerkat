package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ujjwalredd/meerkat/internal/audit"
	"github.com/ujjwalredd/meerkat/internal/awake"
	"github.com/ujjwalredd/meerkat/internal/config"
	"github.com/ujjwalredd/meerkat/internal/decision"
	"github.com/ujjwalredd/meerkat/internal/gitguard"
	"github.com/ujjwalredd/meerkat/internal/processrunner"
	"github.com/ujjwalredd/meerkat/internal/scanner"
	"github.com/ujjwalredd/meerkat/internal/ui"
)

const Version = "0.1.0"

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
  meerkat init                              create starter meerkat.yml
  meerkat run [--policy F] [--keep-awake]
              [--dry-run] -- <cmd> [args]   run command under MeerKat
  meerkat scan [--policy F] [paths...]      secret + policy scan
  meerkat status [--policy F]               show runtime status
  meerkat doctor                            check system compatibility
  meerkat policy validate [--policy F]      validate meerkat.yml
  meerkat explain -- <cmd> [args]           explain decision without running
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
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	out := fs.String("o", "meerkat.yml", "output file")
	force := fs.Bool("force", false, "overwrite existing file")
	fs.Parse(args)
	if _, err := os.Stat(*out); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "%s already exists (use --force to overwrite)\n", *out)
		return 1
	}
	p := config.Default()
	if err := config.Save(p, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("Wrote default policy to %s\n", *out)
	fmt.Println("Review it, then run:  meerkat run -- <your command>")
	return 0
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

	log.Log(audit.Event{Type: "command_started", Command: cmdLine})
	res := processrunner.Run(after)
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
	findings, _ := scanner.Scan(files, &p.Secrets, ".")
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
	root, _ := filepath.Abs(p.Project.Root)
	if root == "" {
		root, _ = os.Getwd()
	}
	var oos []string
	for _, f := range files {
		abs, _ := filepath.Abs(f)
		ok := false
		for _, w := range p.FS.AllowedWritePaths {
			we := w
			if !filepath.IsAbs(we) {
				we = filepath.Join(root, we)
			}
			we, _ = filepath.Abs(we)
			if abs == we || strings.HasPrefix(abs, we+string(filepath.Separator)) {
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
	findings, err := scanner.Scan(paths, &p.Secrets, ".")
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
	_, err := os.Stat("/usr/bin/" + name)
	if err == nil {
		return true
	}
	_, err = os.Stat("/usr/local/bin/" + name)
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
