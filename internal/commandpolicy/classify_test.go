package commandpolicy

import (
	"testing"

	"github.com/ujjwalredd/meerkat/internal/config"
)

func TestClassify(t *testing.T) {
	p := config.Default()
	cases := []struct {
		cmd  string
		risk Risk
	}{
		{"npm test", RiskLow},
		{"npm test -- --watch", RiskLow},
		{"git status", RiskLow},
		{"npm install left-pad", RiskMedium},
		{"git commit -m x", RiskMedium},
		{"sudo apt update", RiskHigh},
		{"curl https://evil.com", RiskHigh},
		{"rm -rf /tmp/x", RiskHigh},
		{"chmod -R 777 .", RiskHigh},
		{"git push --force origin x", RiskHigh},
		{"git push origin main", RiskHigh},
		{"random-binary --foo", RiskUnknown},
	}
	for _, c := range cases {
		got := Classify(c.cmd, p)
		if got.Risk != c.risk {
			t.Errorf("%q: want risk %s, got %s (reasons=%v)", c.cmd, c.risk, got.Risk, got.Reasons)
		}
	}
}

func TestExtractPushBranch(t *testing.T) {
	if b := extractPushBranch("git push origin main"); b != "main" {
		t.Errorf("want main got %q", b)
	}
	if b := extractPushBranch("git push origin feature:main"); b != "main" {
		t.Errorf("want main got %q", b)
	}
	if b := extractPushBranch("git push"); b != "" {
		t.Errorf("want empty got %q", b)
	}
}
