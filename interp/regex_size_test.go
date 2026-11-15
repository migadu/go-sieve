package interp

import (
	"context"
	"fmt"
	"regexp/syntax"
	"strconv"
	"strings"
	"testing"
	"time"
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

// programSize estimates the program regexp compiles a pattern to: never less
// than the program (which adds a Fail and a Match instruction to what the
// pattern compiles to), and not much more, as written and once its ASCII
// letters are folded for i;ascii-casemap.
func TestRegexProgramSizeEstimate(t *testing.T) {
	patterns := []string{
		"", "a", "^a(b)$", "a{0}", "a{2,5}", "a{3,}", "a*", "a+?b??c*?", "(a|b|cd)*",
		`\bfoo\b`, "(?i)abc", "[^a-z]+", "x.{0,1000}y", "[a-z]{1000}",
		"(?:abcdefghi){1000}", "(?:abc|def|ghi){100}", `^(?:[^,]*,){999}`,
		"(?:" + strings.Repeat("a", 990) + "){1000}",
	}
	for _, pattern := range patterns {
		re, err := syntax.Parse(pattern, syntax.Perl)
		if err != nil {
			t.Fatal(err)
		}
		estimate := programSize(re)
		folded, err := foldASCIICase(pattern, re)
		if err != nil {
			t.Fatal(err)
		}
		for _, expr := range []string{pattern, folded} {
			re, err := syntax.Parse(expr, syntax.Perl)
			if err != nil {
				t.Fatal(err)
			}
			prog, err := syntax.Compile(re.Simplify())
			if err != nil {
				t.Fatal(err)
			}
			if actual := len(prog.Inst) - 2; estimate < actual || estimate > 2*actual+1 {
				t.Errorf("%.40q compiled as %.40q: estimated %d instructions, compiles to %d", pattern, expr, estimate, actual)
			}
		}
	}
}

// regexCache compiles a key once per comparator, and keeps at most
// regexCacheMaxKeys keys whose programs come to at most regexCacheMaxSize
// instructions together; a key past either bound is compiled each time it is
// matched, and matches all the same.
func TestRegexCacheBounded(t *testing.T) {
	var c regexCache
	first, err := c.compile("^a(b)", ComparatorOctet)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := c.compile("^a(b)", ComparatorOctet); again != first {
		t.Error("the same key was compiled again")
	}
	if other, _ := c.compile("^a(b)", ComparatorASCIICaseMap); other == first {
		t.Error("the key under another comparator is the same matcher")
	}

	// Each key compiles to 9000 instructions, so eleven fit.
	var last string
	for i := 0; i < 20; i++ {
		last = fmt.Sprintf("(?:k%03dabcde){1000}", i)
		if _, err := c.compile(last, ComparatorOctet); err != nil {
			t.Fatal(err)
		}
	}
	if c.size > regexCacheMaxSize || len(c.matchers) != 2+regexCacheMaxSize/9000 {
		t.Errorf("keeps %d keys of %d instructions", len(c.matchers), c.size)
	}
	a, _ := c.compile(last, ComparatorOctet)
	b, _ := c.compile(last, ComparatorOctet)
	if a == b {
		t.Error("a key past regexCacheMaxSize is kept")
	}
	// The value is over syncMatchInputThreshold: give the match time under -race.
	ctx := ContextWithRegexLimits(context.Background(), RegexLimits{MaxExecTime: 10 * time.Second})
	if ok, err := a.Match(ctx, strings.Repeat(last[3:12], 1000)); !ok || err != nil {
		t.Errorf("a key past regexCacheMaxSize: match %v, %v", ok, err)
	}

	var many regexCache
	for i := 0; i < regexCacheMaxKeys+10; i++ {
		if _, err := many.compile(strconv.Itoa(i), ComparatorOctet); err != nil {
			t.Fatal(err)
		}
	}
	if len(many.matchers) != regexCacheMaxKeys {
		t.Errorf("keeps %d keys, want %d", len(many.matchers), regexCacheMaxKeys)
	}
	if ok, _, err := matchRegex(context.Background(), &many, ComparatorOctet, strconv.Itoa(regexCacheMaxKeys+9), "x"+strconv.Itoa(regexCacheMaxKeys+9)); !ok || err != nil {
		t.Errorf("a key past regexCacheMaxKeys: match %v, %v", ok, err)
	}
}
