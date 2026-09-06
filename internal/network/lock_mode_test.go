package network

import "testing"

func TestShouldChmodLock(t *testing.T) {
	cases := []struct {
		name       string
		owner, uid uint32
		want       bool
	}{
		{"owner matches", 1000, 1000, true},
		{"root always", 0, 0, true},
		{"root overrides", 1000, 0, true},
		{"foreign owner skipped", 0, 1000, false},
	}
	for _, c := range cases {
		if got := shouldChmodLock(c.owner, c.uid); got != c.want {
			t.Fatalf("%s: shouldChmodLock(%d,%d)=%v want %v", c.name, c.owner, c.uid, got, c.want)
		}
	}
}
