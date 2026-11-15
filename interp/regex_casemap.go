package interp

import (
	"fmt"
	"regexp"
	"regexp/syntax"
	"sort"
	"strings"
	"unicode/utf8"
)

// compileSafeRegexFor compiles the key of a :regex match under comparator. The
// pattern is matched against the value as it is, so the match variables hold
// the value's own text, and the casemap comparators make the pattern itself
// case-insensitive, as Pigeonhole compiles it with REG_ICASE (mcht-regex.c):
// i;ascii-casemap for ASCII letters only (RFC 4790 §9.2.1), i;unicode-casemap
// for every letter. The limits and any error are about the key as written:
// folding the case of a letter does not change the size of the program.
func compileSafeRegexFor(pattern string, comparator Comparator, limits RegexLimits) (*SafeRegexMatcher, error) {
	if comparator != ComparatorASCIICaseMap && comparator != ComparatorUnicodeCaseMap {
		return CompileSafeRegex(pattern, limits)
	}
	parsed, size, err := parseSafeRegex(pattern, limits)
	if err != nil {
		return nil, err
	}
	expr := "(?i)" + pattern
	if comparator == ComparatorASCIICaseMap {
		if expr, err = foldASCIICase(pattern, parsed); err != nil {
			return nil, fmt.Errorf("regex compile error: %w", err)
		}
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("regex compile error: %w", err)
	}
	return &SafeRegexMatcher{find: re.FindStringSubmatch, pattern: pattern, limits: limits, size: size}, nil
}

// foldASCIICase rewrites pattern, parsed as parsed, so that each ASCII letter
// in it also matches its other case, and only ASCII letters do: Go's (?i)
// folds every letter, and k and s with the Kelvin sign and the long s.
//
// A negated bracket expression is folded in the pattern text, before the
// parser negates it: as with REG_ICASE, [^a] then matches neither a nor A,
// where folding the parsed class, which holds A, would make it match both.
// Everything else is folded in the parsed expression: a literal letter becomes
// a class of its two cases, and a class gets the other case of each letter.
// That includes a class an escape negates itself, such as \P{Lu} or
// [[:^upper:]]: Go extensions to the POSIX ERE syntax the regex draft
// specifies, folded after their negation.
func foldASCIICase(pattern string, parsed *syntax.Regexp) (string, error) {
	folded := foldNegatedBrackets(pattern)
	if folded != pattern {
		var err error
		if parsed, err = syntax.Parse(folded, syntax.Perl); err != nil {
			return "", err
		}
	}
	if !foldASCIILetters(parsed) {
		return folded, nil
	}
	return parsed.String(), nil
}

// foldNegatedBrackets adds to each negated bracket expression in pattern the
// other case of each ASCII letter it lists, or returns pattern if none lacks
// one. It finds the brackets as regexp/syntax does with syntax.Perl: outside a
// bracket, nothing after a backslash or between \Q and \E opens one, and
// bracketEnd finds where one closes. The caller has parsed pattern, so every
// bracket is closed.
func foldNegatedBrackets(pattern string) string {
	var b strings.Builder
	done := 0
	for i := 0; i < len(pattern); {
		switch pattern[i] {
		case '\\':
			if strings.HasPrefix(pattern[i:], `\Q`) {
				end := strings.Index(pattern[i+2:], `\E`)
				if end < 0 {
					i = len(pattern)
					continue
				}
				i += 2 + end + 2
				continue
			}
			i += 2
		case '[':
			end := bracketEnd(pattern, i)
			if end < 0 {
				i = len(pattern)
				continue
			}
			if pattern[i+1] == '^' {
				body := pattern[i+2 : end]
				if other := otherCaseLetters(body); other != "" {
					b.WriteString(pattern[done : i+2])
					b.WriteString(other)
					// The added letters come first: escape a first "]",
					// "-" or "^" that was a member only because it was first.
					if body[0] == ']' || body[0] == '-' || body[0] == '^' {
						b.WriteByte('\\')
					}
					done = i + 2
				}
			}
			i = end + 1
		default:
			i++
		}
	}
	if done == 0 {
		return pattern
	}
	b.WriteString(pattern[done:])
	return b.String()
}

// bracketEnd returns the index of the "]" that closes the bracket expression
// opening at pattern[start], or -1 if it is not closed. It reads the members
// as regexp/syntax's parseClass does: a "]" first is a member, "[:" opens a
// named class up to ":]" unless it ends a range, as in "[A-[:]", and a
// character followed by "-" and anything but "]" starts a range.
func bracketEnd(pattern string, start int) int {
	i := start + 1
	if i < len(pattern) && pattern[i] == '^' {
		i++
	}
	for first := true; i < len(pattern); first = false {
		if pattern[i] == ']' && !first {
			return i
		}
		if strings.HasPrefix(pattern[i:], "[:") {
			if end := strings.Index(pattern[i+2:], ":]"); end >= 0 {
				i += 2 + end + 2
				continue
			}
		}
		n, class := bracketMember(pattern[i:])
		i += n
		if !class && i+1 < len(pattern) && pattern[i] == '-' && pattern[i+1] != ']' {
			n, _ = bracketMember(pattern[i+1:])
			i += 1 + n
		}
	}
	return -1
}

// bracketMember returns the length of the character, escape included (\x41,
// \x{41}, \101), or of the class escape (\d, \pL, \p{Greek}) that s, inside a
// bracket expression, starts with, and whether it is a class, which cannot
// start a range. The lengths are those of regexp/syntax's parseEscape and
// parseUnicodeClass.
func bracketMember(s string) (int, bool) {
	if s[0] != '\\' || len(s) < 2 {
		_, n := utf8.DecodeRuneInString(s)
		return n, false
	}
	switch c := s[1]; {
	case c == 'd' || c == 'D' || c == 's' || c == 'S' || c == 'w' || c == 'W':
		return 2, true
	case c == 'p' || c == 'P':
		if len(s) > 2 && s[2] == '{' {
			return braced(s), true
		}
		_, n := utf8.DecodeRuneInString(s[2:])
		return 2 + n, true
	case c == 'x':
		if len(s) > 2 && s[2] == '{' {
			return braced(s), false
		}
		if len(s) < 4 {
			return len(s), false
		}
		return 4, false
	case '0' <= c && c <= '7':
		n := 2
		for n < 4 && n < len(s) && '0' <= s[n] && s[n] <= '7' {
			n++
		}
		return n, false
	}
	_, n := utf8.DecodeRuneInString(s[1:])
	return 1 + n, false
}

// braced returns the length of s up to and including its first "}".
func braced(s string) int {
	if end := strings.IndexByte(s, '}'); end >= 0 {
		return end + 1
	}
	return len(s)
}

// otherCaseLetters returns the ASCII letters that the members of a bracket
// expression, body, lack while they hold the letter's other case.
func otherCaseLetters(body string) string {
	if body[0] == '^' {
		body = `\` + body // a member here, not a negation
	}
	re, err := syntax.Parse("["+body+"]", syntax.Perl)
	if err != nil {
		return ""
	}
	// The parser makes a bracket of one member, or of the two cases of one
	// letter (FoldCase), a literal, and one of every character "." (OpAnyChar,
	// OpAnyCharNotNL), which holds both cases of every letter.
	var ranges []rune
	switch {
	case re.Op == syntax.OpCharClass:
		ranges = re.Rune
	case re.Op == syntax.OpLiteral && re.Flags&syntax.FoldCase == 0:
		ranges = []rune{re.Rune[0], re.Rune[0]}
	default:
		return ""
	}
	var other []byte
	for upper := rune('A'); upper <= 'Z'; upper++ {
		lower := upper | 0x20
		hasUpper, hasLower := inRanges(ranges, upper), inRanges(ranges, lower)
		if hasUpper && !hasLower {
			other = append(other, byte(lower))
		} else if hasLower && !hasUpper {
			other = append(other, byte(upper))
		}
	}
	return string(other)
}

// inRanges reports whether r is in the class ranges (lo-hi pairs).
func inRanges(ranges []rune, r rune) bool {
	for i := 0; i+1 < len(ranges); i += 2 {
		if ranges[i] <= r && r <= ranges[i+1] {
			return true
		}
	}
	return false
}

// foldASCIILetters makes each ASCII letter that re matches match its other case
// as well, and reports whether re changed. A literal the pattern made
// case-insensitive itself, with (?i), is left as it is.
func foldASCIILetters(re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpLiteral:
		if re.Flags&syntax.FoldCase != 0 || !hasASCIILetter(re.Rune) {
			return false
		}
		var subs []*syntax.Regexp
		for i := 0; i < len(re.Rune); {
			if r := re.Rune[i]; isASCIILetter(r) {
				upper, lower := r&^0x20, r|0x20
				subs = append(subs, &syntax.Regexp{Op: syntax.OpCharClass, Rune: []rune{upper, upper, lower, lower}})
				i++
				continue
			}
			j := i + 1
			for j < len(re.Rune) && !isASCIILetter(re.Rune[j]) {
				j++
			}
			// A copy: re.Rune may be held in re.Rune0, which *re = below clears.
			subs = append(subs, &syntax.Regexp{Op: syntax.OpLiteral, Flags: re.Flags, Rune: append([]rune(nil), re.Rune[i:j]...)})
			i = j
		}
		if len(subs) == 1 {
			*re = *subs[0]
		} else {
			*re = syntax.Regexp{Op: syntax.OpConcat, Sub: subs}
		}
		return true
	case syntax.OpCharClass:
		folded, ok := foldASCIIClass(re.Rune)
		if ok {
			re.Rune = folded
		}
		return ok
	}
	changed := false
	for _, sub := range re.Sub {
		if foldASCIILetters(sub) {
			changed = true
		}
	}
	return changed
}

// foldASCIIClass returns the ranges of a class, ranges (lo-hi pairs), with the
// other case of each ASCII letter in them, and whether that added any.
func foldASCIIClass(ranges []rune) ([]rune, bool) {
	type span struct{ lo, hi rune }
	spans := make([]span, 0, len(ranges))
	letters := false
	for i := 0; i+1 < len(ranges); i += 2 {
		lo, hi := ranges[i], ranges[i+1]
		spans = append(spans, span{lo, hi})
		for _, cased := range [...]span{{'A', 'Z'}, {'a', 'z'}} {
			l, h := lo, hi
			if l < cased.lo {
				l = cased.lo
			}
			if h > cased.hi {
				h = cased.hi
			}
			if l <= h {
				spans = append(spans, span{l ^ 0x20, h ^ 0x20}) // the other case
				letters = true
			}
		}
	}
	if !letters {
		return nil, false
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].lo < spans[j].lo })
	folded := make([]rune, 0, len(ranges)+4)
	for _, s := range spans {
		if n := len(folded); n > 0 && s.lo <= folded[n-1]+1 {
			if s.hi > folded[n-1] {
				folded[n-1] = s.hi
			}
			continue
		}
		folded = append(folded, s.lo, s.hi)
	}
	if len(folded) == len(ranges) {
		same := true
		for i := range folded {
			same = same && folded[i] == ranges[i]
		}
		if same {
			return nil, false
		}
	}
	return folded, true
}

func isASCIILetter(r rune) bool {
	return 'A' <= r && r <= 'Z' || 'a' <= r && r <= 'z'
}

func hasASCIILetter(runes []rune) bool {
	for _, r := range runes {
		if isASCIILetter(r) {
			return true
		}
	}
	return false
}
