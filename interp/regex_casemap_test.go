package interp

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// TestRegexComparatorCase checks :regex under each comparator that may carry it.
// RFC 4790 §9.2.1 defines i;ascii-casemap as i;octet after "the lower-case
// letters (octet values 97-122) in each input string are changed to upper case
// (octet values 65-90)", and "letters outside ASCII are not treated
// case-insensitively", so under it a pattern matches whatever the case of an
// ASCII letter in the pattern or the value, and only of an ASCII letter.
// Pigeonhole compiles the pattern with REG_ICASE (mcht-regex.c) and matches it
// against the value as it is, so the match variables hold the value's own
// text, and a negated bracket expression [^...] excludes both cases of each
// letter it lists.
func TestRegexComparatorCase(t *testing.T) {
	tests := []struct {
		comparator Comparator
		pattern    string
		value      string
		want       []string // nil: no match
	}{
		// An upper-case letter in the pattern.
		{ComparatorASCIICaseMap, "^URGENT", "URGENT Stuff", []string{"URGENT"}},
		{ComparatorASCIICaseMap, "^URGENT", "urgent stuff", []string{"urgent"}},
		{ComparatorASCIICaseMap, "^urgent", "URGENT Stuff", []string{"URGENT"}},
		{ComparatorASCIICaseMap, "^Urgent stuff$", "URGENT Stuff", []string{"URGENT Stuff"}},
		// The match variables hold the value as it is.
		{ComparatorASCIICaseMap, "^(u.*)$", "URGENT Stuff", []string{"URGENT Stuff", "URGENT Stuff"}},
		{ComparatorASCIICaseMap, "^(U)rgent (S)", "urgent stuff", []string{"urgent s", "u", "s"}},
		// A letter written as an escape or quoted is a letter too.
		{ComparatorASCIICaseMap, `^\x55`, "urgent", []string{"u"}},
		{ComparatorASCIICaseMap, `^\QU[R\E`, "u[rgent", []string{"u[r"}},
		{ComparatorASCIICaseMap, `^\Q[^U]\E`, "[^u]", []string{"[^u]"}},
		// Letters merged into one class by an alternation.
		{ComparatorASCIICaseMap, "^(A|B)$", "b", []string{"b", "b"}},
		// A bracket expression and a character class.
		{ComparatorASCIICaseMap, "^[A-Z]+$", "urgent", []string{"urgent"}},
		{ComparatorASCIICaseMap, "^[[:upper:]]+$", "urgent", []string{"urgent"}},
		{ComparatorASCIICaseMap, `^\p{Lu}+$`, "urgent", []string{"urgent"}},
		// A negated bracket expression excludes both cases of a letter it lists.
		{ComparatorASCIICaseMap, "^[^A-Z]", "urgent", nil},
		{ComparatorASCIICaseMap, "^[^a-z]", "URGENT", nil},
		{ComparatorASCIICaseMap, "^[^A-Z]", "1", []string{"1"}},
		{ComparatorASCIICaseMap, "^[^[:upper:]]+$", "urgent", nil},
		{ComparatorASCIICaseMap, "^[^[:lower:]]+$", "URGENT", nil},
		{ComparatorASCIICaseMap, "^[^[:lower:]]+$", "123", []string{"123"}},
		// ... with "]", "-" or "^" as its first character.
		{ComparatorASCIICaseMap, "^[^]a]", "A", nil},
		{ComparatorASCIICaseMap, "^[^]a]", "]", nil},
		{ComparatorASCIICaseMap, "^[^]a]", "b", []string{"b"}},
		{ComparatorASCIICaseMap, "^[^-a]", "A", nil},
		{ComparatorASCIICaseMap, "^[^-a]", "-", nil},
		{ComparatorASCIICaseMap, "^[^-a]", "b", []string{"b"}},
		{ComparatorASCIICaseMap, "^[^^a]", "A", nil},
		{ComparatorASCIICaseMap, "^[^^a]", "^", nil},
		{ComparatorASCIICaseMap, "^[^^a]", "b", []string{"b"}},
		// ... with "[" ending a range, not opening a named class, and with
		// escaped characters as the ends of a range.
		{ComparatorASCIICaseMap, "^[^A-[:]$", "z", nil},
		{ComparatorASCIICaseMap, "^[^A-[:]$", "1", []string{"1"}},
		{ComparatorASCIICaseMap, `^[^\x41-\x5a-[:digit:]]$`, "z", nil},
		{ComparatorASCIICaseMap, `^[^\x41-\x5a-[:digit:]]$`, "_", []string{"_"}},
		{ComparatorASCIICaseMap, `^[^\x{41}-\x{5a}\d]$`, "z", nil},
		// An escape keeps its meaning.
		{ComparatorASCIICaseMap, `^\D+$`, "URGENT", []string{"URGENT"}},
		{ComparatorASCIICaseMap, `^\W`, "URGENT", nil},
		// Only ASCII letters fold: not É, nor the Kelvin sign or the long s that
		// Unicode folds with k and s.
		{ComparatorASCIICaseMap, "é", "É", nil},
		{ComparatorASCIICaseMap, "^k$", "K", nil},
		{ComparatorASCIICaseMap, "^K$", "K", nil},
		{ComparatorASCIICaseMap, "^[k]$", "K", nil},
		{ComparatorASCIICaseMap, "^[^k]$", "K", []string{"K"}},
		{ComparatorASCIICaseMap, "^s$", "ſ", nil},
		// Unless the pattern asks for it.
		{ComparatorASCIICaseMap, "(?i)é", "É", []string{"É"}},
		// i;octet is case-sensitive.
		{ComparatorOctet, "^urgent", "URGENT", nil},
		{ComparatorOctet, "^[^[:lower:]]+$", "URGENT", []string{"URGENT"}},
		// i;unicode-casemap folds every letter.
		{ComparatorUnicodeCaseMap, "^ÜBER (s.*)$", "Über Stuff", []string{"Über Stuff", "Stuff"}},
		{ComparatorUnicodeCaseMap, "^[^ü]", "Ü", nil},
	}
	for _, tc := range tests {
		ok, matches, err := testString(context.Background(), tc.comparator, MatchRegex, "", tc.value, tc.pattern)
		if err != nil {
			t.Errorf("%s %q against %q: %v", tc.comparator, tc.pattern, tc.value, err)
			continue
		}
		if ok != (tc.want != nil) || !reflect.DeepEqual(matches, tc.want) {
			t.Errorf("%s %q against %q: match %v %q, want %v %q", tc.comparator, tc.pattern, tc.value, ok, matches, tc.want != nil, tc.want)
		}
	}
}

// TestRegexASCIICaseMapPatternLimits: folding the case of a pattern makes it
// longer, so the pattern-length limit is applied to the key as written, and a
// key that is not a regular expression is still an error.
func TestRegexASCIICaseMapPatternLimits(t *testing.T) {
	key := strings.Repeat("a", DefaultRegexLimits.MaxPatternLength)
	ok, _, err := testString(context.Background(), ComparatorASCIICaseMap, MatchRegex, "", strings.Repeat("A", len(key)), key)
	if err != nil || !ok {
		t.Errorf("key of the maximum length: match %v, err %v; want a match", ok, err)
	}
	if _, _, err := testString(context.Background(), ComparatorASCIICaseMap, MatchRegex, "", "a", key+"a"); err == nil {
		t.Error("key over the maximum length: no error")
	}
	for _, key := range []string{"(", "[^a", "[^[:nope:]]", `[^\p{Nope}]`} {
		if _, _, err := testString(context.Background(), ComparatorASCIICaseMap, MatchRegex, "", "a", key); err == nil {
			t.Errorf("key %q: no error", key)
		}
	}
}
