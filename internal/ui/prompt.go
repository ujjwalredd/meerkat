package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ujjwalredd/meerkat/internal/commandpolicy"
	"github.com/ujjwalredd/meerkat/internal/decision"
)

type Choice int

const (
	ChoiceDeny Choice = iota
	ChoiceApproveOnce
	ChoiceApproveSession
	ChoiceAlwaysDeny
)

type PromptCtx struct {
	Command   string
	Cwd       string
	Branch    string
	Decision  decision.Decision
	Class     commandpolicy.Classification
	TimeoutS  int
	OnTimeout string // "deny"|"allow"
}

func Render(ctx PromptCtx, w io.Writer) {
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "MeerKat requires approval")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Command:")
	fmt.Fprintln(w, "  "+ctx.Command)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Working dir:")
	fmt.Fprintln(w, "  "+ctx.Cwd)
	if ctx.Branch != "" {
		fmt.Fprintln(w, "Git branch: "+ctx.Branch)
	}
	fmt.Fprintln(w, "Risk: "+strings.ToUpper(ctx.Decision.RiskLevel))
	if ctx.Class.NetworkLikely {
		fmt.Fprintln(w, "Network: likely required")
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Reasons:")
	for _, r := range ctx.Decision.Reasons {
		fmt.Fprintln(w, "  - "+r)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Allow?")
	fmt.Fprintln(w, "  [y] approve once")
	fmt.Fprintln(w, "  [s] approve for session")
	fmt.Fprintln(w, "  [n] deny")
	fmt.Fprintln(w, "  [N] always deny this pattern")
	fmt.Fprintf(w, "Default on %ds timeout: %s\n", ctx.TimeoutS, ctx.OnTimeout)
}

// Ask reads a choice from stdin with timeout. Returns ChoiceDeny on timeout/EOF
// unless OnTimeout=="allow".
func Ask(ctx PromptCtx) Choice {
	Render(ctx, os.Stderr)
	ch := make(chan string, 1)
	go func() {
		s := bufio.NewScanner(os.Stdin)
		if s.Scan() {
			ch <- strings.TrimSpace(s.Text())
		} else {
			ch <- ""
		}
	}()
	timeout := time.Duration(ctx.TimeoutS) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	select {
	case ans := <-ch:
		switch ans {
		case "y", "Y":
			return ChoiceApproveOnce
		case "s", "S":
			return ChoiceApproveSession
		case "N":
			return ChoiceAlwaysDeny
		default:
			return ChoiceDeny
		}
	case <-time.After(timeout):
		fmt.Fprintln(os.Stderr, "\n[meerkat] approval timeout")
		if ctx.OnTimeout == "allow" {
			return ChoiceApproveOnce
		}
		return ChoiceDeny
	}
}
