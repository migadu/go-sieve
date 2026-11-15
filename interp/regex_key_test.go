package interp

import "testing"

// A :regex key without variables is compiled once, when the script loads, and
// the test runs that matcher; a key with a variable is compiled when the test
// runs, once it is known.
func TestRegexKeyCompiledOnceAtLoad(t *testing.T) {
	s := &Script{
		extensions: map[string]struct{}{"regex": {}, "variables": {}},
		opts:       &Options{MaxVariableNameLen: 32},
	}
	m := newMatcherTest()
	m.match = MatchRegex
	if err := m.setKey(s, []string{"^a(b)", "${x}"}); err != nil {
		t.Fatal(err)
	}
	if len(m.keyCompiled) != 2 || m.keyCompiled[0] == nil {
		t.Fatal("the key without variables is not compiled at load")
	}
	if m.keyCompiled[1] != nil {
		t.Fatal("the key with a variable is compiled at load")
	}
}
