package commandpolicy

import (
	"regexp"
	"strings"

	"github.com/ujjwalredd/meerkat/internal/config"
)

type Risk int

// Order matters: numeric ordering Unknown < Low < Medium < High so
// `if risk < Medium { risk = Medium }` bumps Unknown correctly.
const (
	RiskUnknown Risk = iota
	RiskLow
	RiskMedium
	RiskHigh
)

func (r Risk) String() string {
	switch r {
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	}
	return "unknown"
}

type Classification struct {
	Risk           Risk
	Reasons        []string
	NetworkLikely  bool
	GitOp          string // "", "push", "commit", "force-push", etc.
	Branch         string
	WritesPaths    []string
	ReadsPaths     []string
	MatchedPattern string // policy pattern matched (block/auto/req)
	MatchedList    string // "block"|"auto_approve"|"require_approval"|""
}

var (
	singleTokenBoundary = regexp.MustCompile(`^[A-Za-z0-9_.:/\\-]+$`)
)

// Classify a command line into risk + reasons, applying explicit policy lists.
func Classify(cmd string, p *config.Policy) Classification {
	c := Classification{Risk: RiskUnknown}
	lc := strings.ToLower(strings.TrimSpace(cmd))
	parsed := parseCommand(cmd)

	// Explicit block list wins
	for _, b := range p.Cmds.Block {
		if matchPatternParsed(parsed, lc, strings.ToLower(b)) {
			c.Risk = RiskHigh
			c.MatchedPattern = b
			c.MatchedList = "block"
			c.Reasons = append(c.Reasons, "Command matches blocked pattern: "+b)
			return c
		}
	}

	if parsed.HasControl {
		c.Risk = bump(c.Risk, RiskMedium)
		c.Reasons = append(c.Reasons, "Shell control operator detected; chained commands require approval")
	}
	if parsed.HasRedirection {
		c.Risk = bump(c.Risk, RiskMedium)
		c.Reasons = append(c.Reasons, "Shell redirection detected; file effects require approval")
	}
	if parsed.HasShellWrapper {
		c.Risk = bump(c.Risk, RiskMedium)
		c.Reasons = append(c.Reasons, "Shell -c wrapper detected; inner command was inspected")
	}
	for _, w := range parsed.Warnings {
		c.Risk = bump(c.Risk, RiskMedium)
		c.Reasons = append(c.Reasons, w)
	}

	// Heuristics for high risk
	for _, seg := range parsed.Segments {
		name, idx := commandName(seg.Tokens)
		if name == "" {
			continue
		}
		switch name {
		case "sudo", "doas", "su":
			c.Risk = RiskHigh
			c.Reasons = append(c.Reasons, "Privilege escalation (sudo/su) is blocked by default")
			return c
		case "rm":
			if hasRecursiveForce(seg.Tokens[idx+1:]) {
				c.Risk = RiskHigh
				c.Reasons = append(c.Reasons, "Recursive force-remove is high-risk")
				return c
			}
		case "chmod", "chown":
			if hasRecursiveFlag(seg.Tokens[idx+1:]) {
				c.Risk = RiskHigh
				c.Reasons = append(c.Reasons, "Recursive permission/ownership change is high-risk")
				return c
			}
		case "git":
			sub, subIdx := gitSubcommand(seg.Tokens, idx+1)
			switch sub {
			case "push":
				c.GitOp = "push"
				if hasForceFlag(seg.Tokens[subIdx+1:]) {
					c.GitOp = "force-push"
					c.Risk = RiskHigh
					c.Reasons = append(c.Reasons, "git force push is blocked by default")
					return c
				}
				c.Branch = extractPushBranchTokens(seg.Tokens, subIdx)
				if isProtected(c.Branch, p.Git.ProtectedBranches) && p.Git.BlockPushToProtected {
					c.Risk = RiskHigh
					c.Reasons = append(c.Reasons, "Push to protected branch '"+c.Branch+"' is blocked by default")
					return c
				}
				c.Risk = RiskHigh
				c.NetworkLikely = true
				c.Reasons = append(c.Reasons, "git push touches remote repository")
			case "commit":
				c.GitOp = "commit"
				c.Risk = bump(c.Risk, RiskMedium)
				c.Reasons = append(c.Reasons, "git commit requires secret scan by default")
			}
		}
		if isNetworkTool(name) {
			c.Risk = RiskHigh
			c.NetworkLikely = true
			c.Reasons = append(c.Reasons, "Network egress tool detected (curl/wget/ssh/scp/nc)")
			return c
		}
		if isInstallCommand(name, seg.Tokens[idx+1:]) {
			c.Risk = bump(c.Risk, RiskMedium)
			c.NetworkLikely = true
			c.Reasons = append(c.Reasons, "Package install can run lifecycle scripts and requires network")
		}
	}

	// Explicit auto-approve list
	for _, a := range p.Cmds.AutoApprove {
		if matchPatternParsed(parsed, lc, strings.ToLower(a)) {
			if c.Risk == RiskUnknown {
				c.Risk = RiskLow
			}
			c.MatchedPattern = a
			c.MatchedList = "auto_approve"
			c.Reasons = append(c.Reasons, "Command matches auto-approve pattern: "+a)
			return c
		}
	}
	// Explicit require-approval list
	for _, r := range p.Cmds.RequireApproval {
		if matchPatternParsed(parsed, lc, strings.ToLower(r)) {
			c.Risk = bump(c.Risk, RiskMedium)
			c.MatchedPattern = r
			c.MatchedList = "require_approval"
			c.Reasons = append(c.Reasons, "Command matches require-approval pattern: "+r)
			return c
		}
	}

	if c.Risk == RiskUnknown {
		c.Reasons = append(c.Reasons, "Command not matched by any rule (unknown)")
	}
	return c
}

func bump(got, want Risk) Risk {
	if got < want {
		return want
	}
	return got
}

type parsedCommand struct {
	Segments        []parsedSegment
	HasControl      bool
	HasRedirection  bool
	HasShellWrapper bool
	Warnings        []string
}

type parsedSegment struct {
	Text   string
	Tokens []string
}

// matchPattern: prefix match on tokens (so "npm test" matches "npm test --verbose").
func matchPattern(cmd, pat string) bool {
	cmd = strings.TrimSpace(cmd)
	pat = strings.TrimSpace(pat)
	if pat == "" {
		return false
	}
	if cmd == pat {
		return true
	}
	if strings.HasPrefix(cmd, pat+" ") {
		return true
	}
	// loose substring match for short single-token patterns like "sudo", "curl"
	if !strings.Contains(pat, " ") {
		// boundary check
		if !singleTokenBoundary.MatchString(pat) {
			return false
		}
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(pat) + `\b`)
		return re.MatchString(cmd)
	}
	return false
}

func matchPatternParsed(parsed parsedCommand, rawLower, pat string) bool {
	pat = strings.TrimSpace(strings.ToLower(pat))
	if pat == "" {
		return false
	}
	patTokens, _ := shellFields(pat)
	if len(patTokens) == 0 {
		return false
	}
	if len(patTokens) == 1 && !strings.Contains(pat, " ") {
		want := commandBase(patTokens[0])
		for _, seg := range parsed.Segments {
			if name, _ := commandName(seg.Tokens); name == want {
				return true
			}
		}
		return len(parsed.Segments) == 0 && matchPattern(rawLower, pat)
	}
	for _, seg := range parsed.Segments {
		_, idx := commandName(seg.Tokens)
		if idx < 0 {
			continue
		}
		if tokenPrefix(seg.Tokens[idx:], patTokens) {
			return true
		}
	}
	return matchPattern(rawLower, pat)
}

func tokenPrefix(tokens, pat []string) bool {
	if len(tokens) < len(pat) {
		return false
	}
	for i := range pat {
		if strings.ToLower(tokens[i]) != strings.ToLower(pat[i]) {
			return false
		}
	}
	return true
}

func parseCommand(cmd string) parsedCommand {
	return parseCommandDepth(cmd, 0)
}

func parseCommandDepth(cmd string, depth int) parsedCommand {
	parts, hasControl, hasRedirection, warnings := splitShellSegments(cmd)
	out := parsedCommand{
		HasControl:     hasControl,
		HasRedirection: hasRedirection,
		Warnings:       warnings,
	}
	for _, part := range parts {
		tokens, ws := shellFields(part)
		out.Warnings = append(out.Warnings, ws...)
		if len(tokens) == 0 {
			continue
		}
		out.Segments = append(out.Segments, parsedSegment{Text: strings.Join(tokens, " "), Tokens: tokens})
		if depth < 3 {
			if script, ok := shellScriptArg(tokens); ok {
				out.HasShellWrapper = true
				child := parseCommandDepth(script, depth+1)
				out.Segments = append(out.Segments, child.Segments...)
				out.HasControl = out.HasControl || child.HasControl
				out.HasRedirection = out.HasRedirection || child.HasRedirection
				out.HasShellWrapper = out.HasShellWrapper || child.HasShellWrapper
				out.Warnings = append(out.Warnings, child.Warnings...)
			}
		}
	}
	if len(out.Segments) == 0 && strings.TrimSpace(cmd) != "" {
		out.Warnings = append(out.Warnings, "Command could not be tokenized safely")
	}
	return out
}

func splitShellSegments(cmd string) ([]string, bool, bool, []string) {
	var parts []string
	var b strings.Builder
	quote := rune(0)
	escaped := false
	hasControl := false
	hasRedirection := false
	var warnings []string
	flush := func() {
		if s := strings.TrimSpace(b.String()); s != "" {
			parts = append(parts, s)
		}
		b.Reset()
	}
	for i, r := range cmd {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			b.WriteRune(r)
			escaped = true
			continue
		}
		if quote != 0 {
			b.WriteRune(r)
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
			b.WriteRune(r)
		case ';', '\n':
			hasControl = true
			flush()
		case '|':
			hasControl = true
			flush()
		case '&':
			hasControl = true
			flush()
		case '>', '<':
			hasRedirection = true
			b.WriteRune(r)
		default:
			_ = i
			b.WriteRune(r)
		}
	}
	if quote != 0 {
		warnings = append(warnings, "Unbalanced shell quote detected")
	}
	flush()
	return parts, hasControl, hasRedirection, warnings
}

func shellFields(s string) ([]string, []string) {
	var fields []string
	var b strings.Builder
	quote := rune(0)
	escaped := false
	var warnings []string
	flush := func() {
		if b.Len() > 0 {
			fields = append(fields, b.String())
			b.Reset()
		}
	}
	for _, r := range s {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			b.WriteRune(r)
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case ' ', '\t', '\r', '\n':
			flush()
		case '>', '<':
			flush()
			fields = append(fields, string(r))
		default:
			b.WriteRune(r)
		}
	}
	if escaped {
		b.WriteRune('\\')
	}
	if quote != 0 {
		warnings = append(warnings, "Unbalanced shell quote detected")
	}
	flush()
	return fields, warnings
}

func commandName(tokens []string) (string, int) {
	for i := 0; i < len(tokens); i++ {
		name := commandBase(tokens[i])
		switch name {
		case "", ">", "<":
			continue
		case "command", "exec", "builtin":
			continue
		case "env":
			j := i + 1
			for j < len(tokens) {
				if isEnvAssignment(tokens[j]) {
					j++
					continue
				}
				if strings.HasPrefix(tokens[j], "-") {
					if (tokens[j] == "-u" || tokens[j] == "--unset") && j+1 < len(tokens) {
						j += 2
						continue
					}
					j++
					continue
				}
				break
			}
			i = j - 1
			continue
		}
		if isEnvAssignment(tokens[i]) {
			continue
		}
		return name, i
	}
	return "", -1
}

func commandBase(tok string) string {
	tok = strings.TrimSpace(tok)
	tok = strings.Trim(tok, `"'`)
	tok = strings.ReplaceAll(tok, "\\", "/")
	if i := strings.LastIndex(tok, "/"); i >= 0 {
		tok = tok[i+1:]
	}
	tok = strings.ToLower(tok)
	tok = strings.TrimSuffix(tok, ".exe")
	return tok
}

func isEnvAssignment(tok string) bool {
	i := strings.Index(tok, "=")
	if i <= 0 || strings.ContainsAny(tok[:i], "/\\") {
		return false
	}
	for _, r := range tok[:i] {
		if r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

func shellScriptArg(tokens []string) (string, bool) {
	name, idx := commandName(tokens)
	if name != "sh" && name != "bash" && name != "zsh" && name != "dash" {
		return "", false
	}
	for i := idx + 1; i < len(tokens); i++ {
		if tokens[i] == "-c" && i+1 < len(tokens) {
			return tokens[i+1], true
		}
	}
	return "", false
}

func hasRecursiveForce(args []string) bool {
	recursive := false
	force := false
	for _, a := range args {
		if !strings.HasPrefix(a, "-") || a == "--" {
			continue
		}
		if a == "--recursive" {
			recursive = true
		}
		if a == "--force" {
			force = true
		}
		flags := strings.TrimLeft(a, "-")
		if strings.ContainsAny(flags, "rR") {
			recursive = true
		}
		if strings.Contains(flags, "f") {
			force = true
		}
	}
	return recursive && force
}

func hasRecursiveFlag(args []string) bool {
	for _, a := range args {
		if a == "-R" || a == "--recursive" {
			return true
		}
		if strings.HasPrefix(a, "-") && strings.Contains(a, "R") {
			return true
		}
	}
	return false
}

func hasForceFlag(args []string) bool {
	for _, a := range args {
		if a == "--force" || a == "--force-with-lease" || a == "-f" {
			return true
		}
	}
	return false
}

func isNetworkTool(name string) bool {
	switch name {
	case "curl", "wget", "scp", "ssh", "nc", "netcat", "httpie", "http":
		return true
	}
	return false
}

func isInstallCommand(name string, args []string) bool {
	installers := map[string]bool{
		"npm": true, "pnpm": true, "yarn": true, "pip": true, "pip3": true,
		"poetry": true, "cargo": true, "go": true, "gem": true, "brew": true,
		"apt": true, "apt-get": true, "yum": true, "dnf": true, "pacman": true,
	}
	if !installers[name] {
		return false
	}
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		switch a {
		case "install", "add", "get", "i":
			return true
		}
		return false
	}
	return false
}

func gitSubcommand(tokens []string, start int) (string, int) {
	for i := start; i < len(tokens); i++ {
		if tokens[i] == "--" {
			continue
		}
		if strings.HasPrefix(tokens[i], "-") {
			if gitGlobalFlagTakesValue(tokens[i]) && i+1 < len(tokens) {
				i++
			}
			continue
		}
		return strings.ToLower(tokens[i]), i
	}
	return "", -1
}

func isProtected(branch string, protected []string) bool {
	for _, p := range protected {
		if p == branch {
			return true
		}
	}
	return false
}

// extractPushBranch parses `git push <remote> <branch>`; returns "" if unspecified.
func extractPushBranch(cmd string) string {
	parsed := parseCommand(cmd)
	for _, seg := range parsed.Segments {
		name, idx := commandName(seg.Tokens)
		if name != "git" {
			continue
		}
		sub, subIdx := gitSubcommand(seg.Tokens, idx+1)
		if sub == "push" {
			return extractPushBranchTokens(seg.Tokens, subIdx)
		}
	}
	return ""
}

func extractPushBranchTokens(tokens []string, pushIdx int) string {
	if pushIdx < 0 {
		return ""
	}
	args := tokens[pushIdx+1:]
	positional := make([]string, 0, 2)
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			continue
		}
		if strings.HasPrefix(a, "-") {
			if flagTakesValue(a) && i+1 < len(args) {
				i++
			}
			continue
		}
		positional = append(positional, a)
	}
	if len(positional) < 2 {
		return ""
	}
	b := positional[1]
	// strip refspec source:dest
	if i := strings.Index(b, ":"); i >= 0 {
		b = b[i+1:]
	}
	return b
}

func flagTakesValue(flag string) bool {
	switch flag {
	case "--repo", "--receive-pack", "--exec", "--push-option", "-o":
		return true
	}
	return false
}

func gitGlobalFlagTakesValue(flag string) bool {
	switch flag {
	case "-C", "-c", "--git-dir", "--work-tree", "--namespace", "--config-env":
		return true
	}
	return strings.HasPrefix(flag, "--git-dir=") ||
		strings.HasPrefix(flag, "--work-tree=") ||
		strings.HasPrefix(flag, "--namespace=") ||
		strings.HasPrefix(flag, "--config-env=")
}
