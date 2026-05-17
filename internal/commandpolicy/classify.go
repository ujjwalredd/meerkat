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
	reCurl    = regexp.MustCompile(`\b(curl|wget|scp|ssh|nc|netcat|httpie|http)\b`)
	reInstall = regexp.MustCompile(`\b(npm|pnpm|yarn|pip|pip3|poetry|cargo|go|gem|brew|apt|apt-get|yum|dnf|pacman)\b\s+(install|add|get|i)\b`)
	reSudo    = regexp.MustCompile(`\b(sudo|doas|su)\b`)
	reRm      = regexp.MustCompile(`\brm\b\s+.*-[a-zA-Z]*r[a-zA-Z]*f`)
	rePush    = regexp.MustCompile(`\bgit\s+push\b`)
	reForce   = regexp.MustCompile(`--force(-with-lease)?\b|(\s|^)-f(\s|$)`)
	reCommit  = regexp.MustCompile(`\bgit\s+commit\b`)
	reChmod   = regexp.MustCompile(`\bchmod\b\s+.*-R`)
	reChown   = regexp.MustCompile(`\bchown\b\s+.*-R`)
)

// Classify a command line into risk + reasons, applying explicit policy lists.
func Classify(cmd string, p *config.Policy) Classification {
	c := Classification{Risk: RiskUnknown}
	lc := strings.ToLower(strings.TrimSpace(cmd))

	// Explicit block list wins
	for _, b := range p.Cmds.Block {
		if matchPattern(lc, strings.ToLower(b)) {
			c.Risk = RiskHigh
			c.MatchedPattern = b
			c.MatchedList = "block"
			c.Reasons = append(c.Reasons, "Command matches blocked pattern: "+b)
			return c
		}
	}
	// Heuristics for high risk
	if reSudo.MatchString(lc) {
		c.Risk = RiskHigh
		c.Reasons = append(c.Reasons, "Privilege escalation (sudo/su) is blocked by default")
		return c
	}
	if reRm.MatchString(lc) {
		c.Risk = RiskHigh
		c.Reasons = append(c.Reasons, "Recursive force-remove is high-risk")
		return c
	}
	if reChmod.MatchString(lc) || reChown.MatchString(lc) {
		c.Risk = RiskHigh
		c.Reasons = append(c.Reasons, "Recursive permission/ownership change is high-risk")
		return c
	}
	if rePush.MatchString(lc) {
		c.GitOp = "push"
		if reForce.MatchString(lc) {
			c.GitOp = "force-push"
			c.Risk = RiskHigh
			c.Reasons = append(c.Reasons, "git force push is blocked by default")
			return c
		}
		c.Branch = extractPushBranch(cmd)
		if isProtected(c.Branch, p.Git.ProtectedBranches) && p.Git.BlockPushToProtected {
			c.Risk = RiskHigh
			c.Reasons = append(c.Reasons, "Push to protected branch '"+c.Branch+"' is blocked by default")
			return c
		}
		c.Risk = RiskHigh
		c.NetworkLikely = true
		c.Reasons = append(c.Reasons, "git push touches remote repository")
	}
	if reCommit.MatchString(lc) {
		c.GitOp = "commit"
		c.Risk = RiskMedium
		c.Reasons = append(c.Reasons, "git commit requires secret scan by default")
	}
	if reCurl.MatchString(lc) {
		c.Risk = RiskHigh
		c.NetworkLikely = true
		c.Reasons = append(c.Reasons, "Network egress tool detected (curl/wget/ssh/scp/nc)")
	}
	if reInstall.MatchString(lc) {
		if c.Risk < RiskMedium {
			c.Risk = RiskMedium
		}
		c.NetworkLikely = true
		c.Reasons = append(c.Reasons, "Package install can run lifecycle scripts and requires network")
	}

	// Explicit auto-approve list
	for _, a := range p.Cmds.AutoApprove {
		if matchPattern(lc, strings.ToLower(a)) {
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
		if matchPattern(lc, strings.ToLower(r)) {
			if c.Risk < RiskMedium {
				c.Risk = RiskMedium
			}
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
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(pat) + `\b`)
		return re.MatchString(cmd)
	}
	return false
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
	fields := strings.Fields(cmd)
	idx := -1
	for i, f := range fields {
		if f == "push" {
			idx = i
			break
		}
	}
	if idx < 0 || idx+2 >= len(fields) {
		return ""
	}
	b := fields[idx+2]
	if strings.HasPrefix(b, "-") {
		return ""
	}
	// strip refspec source:dest
	if i := strings.Index(b, ":"); i >= 0 {
		b = b[i+1:]
	}
	return b
}
