package networkpolicy

import (
	"regexp"
	"strings"

	"github.com/ujjwalredd/meerkat/internal/config"
)

// MVP: command/policy-based check. NOT a packet-level firewall.

var reDomain = regexp.MustCompile(`https?://([A-Za-z0-9._\-]+)`)

// ExtractDomains pulls URL hosts from a command line.
func ExtractDomains(cmd string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, m := range reDomain.FindAllStringSubmatch(cmd, -1) {
		h := strings.ToLower(m[1])
		if !seen[h] {
			seen[h] = true
			out = append(out, h)
		}
	}
	return out
}

type Verdict struct {
	Allowed bool
	Asked   bool
	Reason  string
}

// Evaluate checks domains against allow/block lists.
func Evaluate(domains []string, cfg *config.NetworkCfg) Verdict {
	if len(domains) == 0 {
		return Verdict{Allowed: true}
	}
	for _, d := range domains {
		if matchHost(d, cfg.BlockDomains) {
			return Verdict{Allowed: false, Reason: "domain in block list: " + d}
		}
	}
	allUnknown := true
	for _, d := range domains {
		if !matchHost(d, cfg.AllowDomains) {
			if cfg.RequireApprovalForUnknown {
				return Verdict{Asked: true, Reason: "unknown domain requires approval: " + d}
			}
			if strings.ToLower(cfg.Default) == "block" {
				return Verdict{Allowed: false, Reason: "domain not in allow list: " + d}
			}
		} else {
			allUnknown = false
		}
	}
	_ = allUnknown
	return Verdict{Allowed: true}
}

func matchHost(host string, list []string) bool {
	host = strings.ToLower(host)
	for _, p := range list {
		p = strings.ToLower(p)
		if host == p || strings.HasSuffix(host, "."+p) {
			return true
		}
	}
	return false
}
