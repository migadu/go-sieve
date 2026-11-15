package interp

import (
	"strings"
	"testing"
)

// A :regex key without variables is checked when the script loads, and one
// that does not compile refuses the script; its compiled program is not kept,
// so the test compiles each key when it runs, as it does a key with a
// variable.
func TestRegexKeyCheckedAtLoadNotKept(t *testing.T) {
	s := &Script{
		extensions: map[string]struct{}{"regex": {}, "variables": {}},
		opts:       &Options{MaxVariableNameLen: 32},
	}
	m := newMatcherTest()
	m.match = MatchRegex
	if err := m.setKey(s, []string{"^a(b)", "${x}"}); err != nil {
		t.Fatal(err)
	}
	for i, compiled := range m.keyCompiled {
		if compiled != nil {
			t.Fatalf("the compiled program of key %d is kept", i)
		}
	}

	m = newMatcherTest()
	m.match = MatchRegex
	if err := m.setKey(s, []string{"^a", "("}); err == nil || !strings.Contains(err.Error(), "malformed pattern") {
		t.Fatalf("setKey: %v, want a malformed pattern error", err)
	}
}
