package interp

import (
	"context"
	"fmt"
	"strconv"
)

// matcherTest contains code shared between tests
// such as 'header', 'address', 'envelope', 'string' -
// all tests that compare some values from message
// with pre-defined "key"
type matcherTest struct {
	comparator Comparator
	match      Match
	relational Relational
	key        []string

	// Used for keys without variables.
	keyCompiled []CompiledMatcher

	matchCnt int
}

func newMatcherTest() matcherTest {
	return matcherTest{
		comparator: DefaultComparator,
		match:      MatchIs,
	}
}

func (t *matcherTest) addSpecTags(s *Spec) *Spec {
	if s.Tags == nil {
		s.Tags = make(map[string]SpecTag, 4)
	}
	s.Tags["comparator"] = SpecTag{
		NeedsValue:  true,
		MinStrCount: 1,
		MaxStrCount: 1,
		MatchStr: func(val []string) {
			t.comparator = Comparator(val[0])
		},
		NoVariables: true,
	}
	s.Tags["is"] = SpecTag{
		MatchBool: func() {
			t.match = MatchIs
			t.matchCnt++
		},
	}
	s.Tags["contains"] = SpecTag{
		MatchBool: func() {
			t.match = MatchContains
			t.matchCnt++
		},
	}
	s.Tags["matches"] = SpecTag{
		MatchBool: func() {
			t.match = MatchMatches
			t.matchCnt++
		},
	}
	s.Tags["regex"] = SpecTag{
		MatchBool: func() {
			t.match = MatchRegex
			t.matchCnt++
		},
	}
	s.Tags["value"] = SpecTag{
		NeedsValue:  true,
		MinStrCount: 1,
		MaxStrCount: 1,
		NoVariables: true,
		MatchStr: func(val []string) {
			t.match = MatchValue
			t.matchCnt++
			t.relational = Relational(val[0])
		},
	}
	s.Tags["count"] = SpecTag{
		NeedsValue:  true,
		MinStrCount: 1,
		MaxStrCount: 1,
		NoVariables: true,
		MatchStr: func(val []string) {
			t.match = MatchCount
			t.matchCnt++
			t.relational = Relational(val[0])
		},
	}
	return s
}

// checkOptions refuses comparator and match-type options that are wrong in
// themselves, whatever they are matched against: more than one match type,
// :regex without regex, :count or :value without relational or with an
// unknown operator, and a comparator that is unknown or that the script does
// not require.
func (t *matcherTest) checkOptions(s *Script) error {
	if t.matchCnt > 1 {
		return fmt.Errorf("multiple match-types are not allowed")
	}

	// RFC 5228 §2.10.5: "If an extension is not enabled with "require",
	// implementations MUST treat it as if they did not support it at all."
	if t.match == MatchRegex && !s.RequiresExtension("regex") {
		return fmt.Errorf("missing require 'regex'")
	}

	if t.match == MatchCount || t.match == MatchValue {
		if !s.RequiresExtension("relational") {
			return fmt.Errorf("missing require 'relational'")
		}
		switch t.relational {
		case RelGreaterThan, RelGreaterOrEqual,
			RelLessThan, RelLessOrEqual, RelEqual,
			RelNotEqual:
		default:
			return fmt.Errorf("unknown relational operator: %v", t.relational)
		}
	}

	switch t.comparator {
	case ComparatorOctet, ComparatorASCIICaseMap:
	case ComparatorASCIINumeric, ComparatorUnicodeCaseMap:
		// RFC 5228 §2.7.3: "Comparators other than "i;octet" and
		// "i;ascii-casemap" must be declared with require [...] If the
		// comparator is not declared with require, it is also an error, even
		// if the comparator is supported."
		if !s.RequiresExtension("comparator-" + string(t.comparator)) {
			return fmt.Errorf("missing require 'comparator-%s'", t.comparator)
		}
	default:
		return fmt.Errorf("unsupported comparator: %v", t.comparator)
	}
	return nil
}

func (t *matcherTest) setKey(s *Script, k []string) error {
	t.key = k

	if err := t.checkOptions(s); err != nil {
		return err
	}

	caseFold := false
	octet := false
	switch t.comparator {
	case ComparatorOctet:
		octet = true
	case ComparatorUnicodeCaseMap:
		caseFold = true
	case ComparatorASCIICaseMap:
		octet = true
		caseFold = true
	}

	// A :matches pattern without variables is compiled once, here, so one
	// that does not compile refuses the script instead of failing its test at
	// every message.
	if t.match == MatchMatches {
		t.keyCompiled = make([]CompiledMatcher, len(t.key))
		for i := range t.key {
			if len(usedVars(s, t.key[i])) > 0 {
				continue
			}

			var err error
			t.keyCompiled[i], err = compileMatcher(t.key[i], octet, caseFold)
			if err != nil {
				return fmt.Errorf("malformed pattern (%v): %v", t.key[i], err)
			}
		}
	}

	// A :regex key without variables is checked here, so one that does not
	// compile refuses the script instead of failing its test at every
	// message. draft-murchison-sieve-regex §3: "Implementations SHOULD reject
	// regular expressions that are unsupported by this specification as a
	// syntax error." The key is only checked, as Pigeonhole does, and the
	// test compiles it when it runs: counted repetition makes a key of
	// MaxPatternLength bytes compile to tens of megabytes, so compiling and
	// keeping every key would let one script hold hundreds.
	if t.match == MatchRegex {
		for _, key := range t.key {
			if len(usedVars(s, key)) > 0 {
				continue
			}
			if err := checkRegex(key, DefaultRegexLimits); err != nil {
				return fmt.Errorf("malformed pattern (%v): %v", key, err)
			}
		}
	}

	// Note: :count always performs numeric comparison internally via countMatches(),
	// regardless of the comparator setting. The comparator is not used for :count.

	// RFC 5228 §2.7.1: "It is an error to use a comparator with ":contains"
	// or ":matches" that is not compatible with it." The regex draft (§3)
	// puts ":regex" under "the same rules and restrictions", and
	// i;ascii-numeric "does not support the substring operation" (RFC 4790
	// §9.1.1).
	if t.comparator == ComparatorASCIINumeric {
		switch t.match {
		case MatchContains, MatchMatches, MatchRegex:
			return fmt.Errorf("numeric comparator cannot be used with :%s", t.match)
		}
	}

	return nil
}

func (t *matcherTest) isCount() bool {
	return t.match == MatchCount
}

func (t *matcherTest) countMatches(d *RuntimeData, value uint64) bool {
	if !t.isCount() {
		panic("countMatches can be called only with MatchCount matcher")
	}

	for _, k := range t.key {
		kNum, err := strconv.ParseUint(expandVars(d, k), 10, 64)
		if err != nil {
			continue
		}

		if t.relational.CompareUint64(value, kNum) {
			return true
		}
	}

	return false
}

func (t *matcherTest) tryMatch(ctx context.Context, d *RuntimeData, source string) (bool, error) {
	for i, key := range t.key {
		// Honour the script execution deadline between keys so a test with
		// many keys/values can't run past the budget.
		if err := ctx.Err(); err != nil {
			return false, err
		}

		var (
			ok      bool
			matches []string
			err     error
		)
		if t.keyCompiled != nil && t.keyCompiled[i] != nil {
			ok, matches, err = t.keyCompiled[i](ctx, source)
		} else {
			// RFC 5229 §3: "Only one pass through the string SHALL be
			// done." The expanded key is not expanded again: a value
			// holding "${...}" text is that text.
			key = expandVars(d, key)
			ok, matches, err = testString(ctx, t.comparator, t.match, t.relational, source, key)

			// RFC 5231, Section 5.4:
			// With the "i;ascii-numeric" comparator, a numeric comparison is
			// performed.
			if t.match == MatchValue && t.comparator == ComparatorASCIINumeric {
				var lhs, rhs *uint64
				if l, err := strconv.ParseUint(source, 10, 64); err == nil {
					lhs = &l
				}
				if r, err := strconv.ParseUint(key, 10, 64); err == nil {
					rhs = &r
				}

				ok = t.relational.CompareNumericValue(lhs, rhs)
				// No match variables for :value
				matches = nil
				err = nil
			}

		}
		if err != nil {
			return false, err
		}
		if ok {
			if t.match == MatchMatches || t.match == MatchRegex {
				d.MatchVariables = matches
			}
			return true, nil
		}
	}
	return false, nil
}
