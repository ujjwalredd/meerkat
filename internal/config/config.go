package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Policy struct {
	Version int           `yaml:"version"`
	Project ProjectCfg    `yaml:"project"`
	Mode    ModeCfg       `yaml:"mode"`
	Awake   AwakeCfg      `yaml:"awake"`
	FS      FilesystemCfg `yaml:"filesystem"`
	Cmds    CommandsCfg   `yaml:"commands"`
	Net     NetworkCfg    `yaml:"network"`
	Git     GitCfg        `yaml:"git"`
	Secrets SecretsCfg    `yaml:"secrets"`
	Audit   AuditCfg      `yaml:"audit"`
	Approve ApprovalCfg   `yaml:"approval"`

	// Path is the file this was loaded from (not serialized).
	Path string `yaml:"-"`
}

type ProjectCfg struct {
	Name string `yaml:"name"`
	Root string `yaml:"root"`
}

type ModeCfg struct {
	DefaultAction   string `yaml:"default_action"` // ask|allow|block
	AutoApproveSafe bool   `yaml:"auto_approve_safe_actions"`
	DenyOutOfScope  bool   `yaml:"deny_out_of_scope"`
	DryRun          bool   `yaml:"dry_run"`
}

type AwakeCfg struct {
	Enabled            bool   `yaml:"enabled"`
	Mode               string `yaml:"mode"` // disabled|while_command_running|duration
	MaxDurationMinutes int    `yaml:"max_duration_minutes"`
}

type FilesystemCfg struct {
	AllowedReadPaths    []string `yaml:"allowed_read_paths"`
	AllowedWritePaths   []string `yaml:"allowed_write_paths"`
	BlockedPaths        []string `yaml:"blocked_paths"`
	BlockOutsideProject bool     `yaml:"block_outside_project"`
	AllowExternalPaths  []string `yaml:"allow_external_paths"`
}

type CommandsCfg struct {
	AutoApprove     []string `yaml:"auto_approve"`
	RequireApproval []string `yaml:"require_approval"`
	Block           []string `yaml:"block"`
}

type NetworkCfg struct {
	Default                   string   `yaml:"default"` // block|allow|ask
	AllowDomains              []string `yaml:"allow_domains"`
	BlockDomains              []string `yaml:"block_domains"`
	RequireApprovalForUnknown bool     `yaml:"require_approval_for_unknown_domains"`
}

type GitCfg struct {
	ProtectedBranches             []string `yaml:"protected_branches"`
	BlockPushToProtected          bool     `yaml:"block_push_to_protected_branches"`
	RequireCleanTestsBeforeCommit bool     `yaml:"require_clean_tests_before_commit"`
	RequireSecretScanBeforeCommit bool     `yaml:"require_secret_scan_before_commit"`
	RequireSecretScanBeforePush   bool     `yaml:"require_secret_scan_before_push"`
	BlockForcePush                bool     `yaml:"block_force_push"`
}

type SecretsCfg struct {
	Enabled          bool     `yaml:"enabled"`
	ScanBeforeWrite  bool     `yaml:"scan_before_write"`
	ScanBeforeCommit bool     `yaml:"scan_before_commit"`
	ScanBeforePush   bool     `yaml:"scan_before_push"`
	ScanPatterns     []string `yaml:"scan_patterns"`
	IgnorePaths      []string `yaml:"ignore_paths"`
	MaxFileBytes     int64    `yaml:"max_file_bytes"`
}

type AuditCfg struct {
	Enabled              bool   `yaml:"enabled"`
	LogDir               string `yaml:"log_dir"`
	Format               string `yaml:"format"`
	IncludeCommandOutput bool   `yaml:"include_command_output"`
	RedactSecrets        bool   `yaml:"redact_secrets"`
}

type ApprovalCfg struct {
	PromptStyle          string `yaml:"prompt_style"`
	TimeoutSeconds       int    `yaml:"timeout_seconds"`
	DefaultOnTimeout     string `yaml:"default_on_timeout"`
	AllowSessionApproval bool   `yaml:"allow_session_approval"`
	AllowOneTimeApproval bool   `yaml:"allow_one_time_approval"`
}

// Default returns the strict default policy.
func Default() *Policy {
	cwd, _ := os.Getwd()
	name := filepath.Base(cwd)
	return &Policy{
		Version: 1,
		Project: ProjectCfg{Name: name, Root: "."},
		Mode: ModeCfg{
			DefaultAction:   "ask",
			AutoApproveSafe: true,
			DenyOutOfScope:  true,
			DryRun:          false,
		},
		Awake: AwakeCfg{Enabled: true, Mode: "while_command_running", MaxDurationMinutes: 180},
		FS: FilesystemCfg{
			AllowedReadPaths:  []string{"."},
			AllowedWritePaths: []string{"./src", "./tests", "./docs", "./package.json", "./README.md"},
			BlockedPaths: []string{
				"./.env", "./.env.*", "./secrets", "./.git/config",
				"~/.ssh", "~/.aws", "~/.config", "~/Library/Application Support",
			},
			BlockOutsideProject: true,
		},
		Cmds: CommandsCfg{
			AutoApprove: []string{
				"npm test", "npm run test", "npm run build", "npm run lint",
				"pnpm test", "pnpm build", "yarn test",
				"go test ./...", "cargo test", "pytest",
				"git diff", "git status",
			},
			RequireApproval: []string{
				"npm install", "pnpm install", "yarn add", "pip install", "poetry add",
				"cargo add", "go get", "git commit", "git push",
				"docker build", "docker compose up",
			},
			Block: []string{
				"sudo", "su", "rm -rf /", "rm -rf ~", "chmod -R 777", "chown -R",
				"curl", "wget", "scp", "ssh", "nc", "netcat",
				"powershell Invoke-WebRequest", "Invoke-RestMethod",
			},
		},
		Net: NetworkCfg{
			Default:                   "block",
			AllowDomains:              []string{"github.com", "api.github.com", "registry.npmjs.org", "pypi.org", "files.pythonhosted.org"},
			BlockDomains:              []string{"pastebin.com", "webhook.site", "requestbin.com"},
			RequireApprovalForUnknown: true,
		},
		Git: GitCfg{
			ProtectedBranches:             []string{"main", "master", "production"},
			BlockPushToProtected:          true,
			RequireCleanTestsBeforeCommit: false,
			RequireSecretScanBeforeCommit: true,
			RequireSecretScanBeforePush:   true,
			BlockForcePush:                true,
		},
		Secrets: SecretsCfg{
			Enabled:          true,
			ScanBeforeWrite:  false,
			ScanBeforeCommit: true,
			ScanBeforePush:   true,
			ScanPatterns: []string{
				"aws_access_key", "private_key", "github_token", "openai_api_key",
				"anthropic_api_key", "database_url", "jwt", "generic_api_key",
				"slack_token", "stripe_key",
			},
			IgnorePaths:  []string{"./node_modules", "./.git", "./dist", "./build", "./vendor"},
			MaxFileBytes: 1024 * 1024,
		},
		Audit: AuditCfg{
			Enabled: true, LogDir: "./.meerkat/logs", Format: "jsonl",
			IncludeCommandOutput: false, RedactSecrets: true,
		},
		Approve: ApprovalCfg{
			PromptStyle: "compact", TimeoutSeconds: 120, DefaultOnTimeout: "deny",
			AllowSessionApproval: true, AllowOneTimeApproval: true,
		},
	}
}

// Load reads policy from path (or meerkat.yml in cwd).
func Load(path string) (*Policy, error) {
	if path == "" {
		path = "meerkat.yml"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read policy %s: %w", path, err)
	}
	p := &Policy{}
	if err := yaml.Unmarshal(b, p); err != nil {
		return nil, fmt.Errorf("parse policy %s: %w", path, err)
	}
	abs, _ := filepath.Abs(path)
	p.Path = abs
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return p, nil
}

// Save writes policy to path.
func Save(p *Policy, path string) error {
	b, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Validate enforces semantic rules.
func (p *Policy) Validate() error {
	if p.Version != 1 {
		return fmt.Errorf("policy error: version must be 1 (got %d)", p.Version)
	}
	root := p.Project.Root
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("policy error: invalid project.root: %v", err)
	}
	if !p.FS.BlockOutsideProject {
		return nil
	}
	for i, w := range p.FS.AllowedWritePaths {
		exp := expandTilde(w)
		ap, err := filepath.Abs(filepath.Join(root, exp))
		if strings.HasPrefix(exp, string(filepath.Separator)) || strings.HasPrefix(exp, "~") {
			ap, err = filepath.Abs(exp)
		}
		if err != nil {
			return fmt.Errorf("policy error: filesystem.allowed_write_paths[%d] invalid: %v", i, err)
		}
		rel, err := filepath.Rel(absRoot, ap)
		if err != nil || strings.HasPrefix(rel, "..") {
			if !isExplicitlyAllowed(ap, p.FS.AllowExternalPaths) {
				return fmt.Errorf(`policy error in %s:

filesystem.allowed_write_paths[%d] points outside the project:
  %s

By default, MeerKat blocks paths outside the project root.
To allow this intentionally, set filesystem.allow_external_paths with explicit paths.`, p.Path, i, w)
			}
		}
	}
	switch p.Mode.DefaultAction {
	case "", "ask", "allow", "block":
	default:
		return fmt.Errorf("policy error: mode.default_action must be ask|allow|block")
	}
	return nil
}

func isExplicitlyAllowed(ap string, allow []string) bool {
	for _, a := range allow {
		aa, _ := filepath.Abs(expandTilde(a))
		if aa == ap {
			return true
		}
	}
	return false
}

func expandTilde(p string) string {
	if strings.HasPrefix(p, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}
