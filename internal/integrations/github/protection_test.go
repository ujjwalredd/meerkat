package github

import "testing"

func TestParseRemote(t *testing.T) {
	cases := map[string]struct {
		owner, repo string
		ok          bool
	}{
		"https://github.com/foo/bar":     {"foo", "bar", true},
		"https://github.com/foo/bar.git": {"foo", "bar", true},
		"git@github.com:foo/bar.git":     {"foo", "bar", true},
		"git@github.com:foo/bar":         {"foo", "bar", true},
		"https://gitlab.com/foo/bar":     {"", "", false},
		"":                               {"", "", false},
	}
	for url, want := range cases {
		o, r, ok := ParseRemote(url)
		if o != want.owner || r != want.repo || ok != want.ok {
			t.Errorf("%q: want (%s,%s,%v) got (%s,%s,%v)", url, want.owner, want.repo, want.ok, o, r, ok)
		}
	}
}
