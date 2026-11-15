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

// checkRegex refuses a key at load exactly when CompileSafeRegex would refuse
// it at every run, with the same error.
func TestCheckRegexAgreesWithCompileSafeRegex(t *testing.T) {
	patterns := []string{
		"", "^a(b)$", `\d+`, "(?i)abc", "(?P<n>a)", "[[:alpha:]]+", `\<`,
		"(?:" + strings.Repeat("a", 990) + "){1000}",
		strings.Repeat("a", DefaultRegexLimits.MaxPatternLength),
		strings.Repeat("a", DefaultRegexLimits.MaxPatternLength+1),
		"(", "[z-a]", "a{2,1}", "a**", "a{1001}", "(a{1000}){2}", `\8`, `x\Ky`,
		"[[:foo:]]", "\xff", strings.Repeat("(", 500),
		"sirius(+\\+.*)?@friep\\.example\\.com",
		"sirius(\\+.*)?@friep\\.ex[]ample.com",
	}
	for _, pattern := range patterns {
		checkErr := checkRegex(pattern, DefaultRegexLimits)
		_, compileErr := CompileSafeRegex(pattern, DefaultRegexLimits)
		if (checkErr == nil) != (compileErr == nil) || (checkErr != nil && checkErr.Error() != compileErr.Error()) {
			t.Errorf("%.40q: checkRegex: %v, CompileSafeRegex: %v", pattern, checkErr, compileErr)
		}
	}
}
