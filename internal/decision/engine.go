package decision

import (
	"github.com/ujjwalredd/meerkat/internal/commandpolicy"
	"github.com/ujjwalredd/meerkat/internal/config"
)

type Action string

const (
	Allow Action = "ALLOW"
	Ask   Action = "ASK"
	Block Action = "BLOCK"
)

type Decision struct {
	Action    Action   `json:"decision"`
	Reasons   []string `json:"reasons"`
	RiskLevel string   `json:"risk_level"`
}

// Decide combines classification + policy mode + git/net rules.
func Decide(cmd string, p *config.Policy) (Decision, commandpolicy.Classification) {
	c := commandpolicy.Classify(cmd, p)
	d := Decision{Reasons: c.Reasons, RiskLevel: c.Risk.String()}

	switch c.MatchedList {
	case "block":
		d.Action = Block
		return d, c
	case "auto_approve":
		if p.Mode.AutoApproveSafe && c.Risk == commandpolicy.RiskLow {
			d.Action = Allow
			return d, c
		}
		d.Action = Ask
		return d, c
	case "require_approval":
		d.Action = Ask
		return d, c
	}

	switch c.Risk {
	case commandpolicy.RiskHigh:
		d.Action = Block
	case commandpolicy.RiskMedium:
		d.Action = Ask
	case commandpolicy.RiskLow:
		if p.Mode.AutoApproveSafe {
			d.Action = Allow
		} else {
			d.Action = Ask
		}
	case commandpolicy.RiskUnknown:
		switch p.Mode.DefaultAction {
		case "allow":
			d.Action = Allow
		case "block":
			d.Action = Block
		default:
			d.Action = Ask
		}
	}
	return d, c
}
