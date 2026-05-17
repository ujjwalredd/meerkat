package decision

import (
	"testing"

	"github.com/ujjwalredd/meerkat/internal/config"
)

func TestDecide(t *testing.T) {
	p := config.Default()
	cases := []struct {
		cmd  string
		want Action
	}{
		{"npm test", Allow},
		{"git status", Allow},
		{"git push origin main", Block},
		{"git push --force origin x", Block},
		{"sudo rm anything", Block},
		{"curl https://x", Block},
		{"npm install x", Ask},
		{"weird-tool", Ask}, // default_action=ask
	}
	for _, c := range cases {
		d, _ := Decide(c.cmd, p)
		if d.Action != c.want {
			t.Errorf("%q: want %s got %s reasons=%v", c.cmd, c.want, d.Action, d.Reasons)
		}
	}
}

func TestDefaultBlockUnknown(t *testing.T) {
	p := config.Default()
	p.Mode.DefaultAction = "block"
	d, _ := Decide("weird-tool", p)
	if d.Action != Block {
		t.Errorf("unknown w/ default=block want BLOCK got %s", d.Action)
	}
}
