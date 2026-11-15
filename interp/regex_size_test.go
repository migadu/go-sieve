package interp

import (
	"strings"
	"testing"
)

// Counted repetition is expanded when a pattern is compiled, so a key within
// MaxPatternLength can compile to a program of about a million instructions
// (some 45 MB, allocating some 220 MB, in some 40 ms). Such a key is refused,
// with the same error at load (checkRegex) as when the test runs, under every
// comparator :regex takes; a key whose program stays within the bound, counted
// repetition included, still compiles.
func TestRegexProgramSizeBounded(t *testing.T) {
	comparators := []Comparator{ComparatorOctet, ComparatorASCIICaseMap, ComparatorUnicodeCaseMap}

	refused := []string{
		"(?:" + strings.Repeat("a", 990) + "){1000}",
		"(?:abcdefghijk){1000}",
		strings.Repeat("[a-z]{1000}", 11),
		"(?:(a)(b)(c)(d)){1000}",
		"(?:abc|def|ghi){1000}",
	}
	for _, pattern := range refused {
		if err := checkRegex(pattern, DefaultRegexLimits); err == nil || !strings.Contains(err.Error(), "too complex") {
			t.Errorf("checkRegex(%.40q): %v, want a too complex error", pattern, err)
		}
		if _, err := CompileSafeRegex(pattern, DefaultRegexLimits); err == nil || !strings.Contains(err.Error(), "too complex") {
			t.Errorf("CompileSafeRegex(%.40q): %v, want a too complex error", pattern, err)
		}
		for _, comparator := range comparators {
			if _, err := compileSafeRegexFor(pattern, comparator, DefaultRegexLimits); err == nil || !strings.Contains(err.Error(), "too complex") {
				t.Errorf("compileSafeRegexFor(%.40q, %s): %v, want a too complex error", pattern, comparator, err)
			}
		}
	}

	accepted := []string{
		strings.Repeat("a", DefaultRegexLimits.MaxPatternLength),
		strings.Repeat("(a)|", DefaultRegexLimits.MaxPatternLength/4-1) + "b",
		"x.{0,1000}y",
		"[a-z]{1000}",
		"(?:abcdefghi){1000}",
		`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`,
		`^(?:[^,]*,){999}`,
	}
	for _, pattern := range accepted {
		if err := checkRegex(pattern, DefaultRegexLimits); err != nil {
			t.Errorf("checkRegex(%.40q): %v", pattern, err)
		}
		for _, comparator := range comparators {
			if _, err := compileSafeRegexFor(pattern, comparator, DefaultRegexLimits); err != nil {
				t.Errorf("compileSafeRegexFor(%.40q, %s): %v", pattern, comparator, err)
			}
		}
	}
}
