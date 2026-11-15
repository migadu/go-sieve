package interp

import (
	"fmt"
	"regexp"
	"regexp/syntax"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// compileSafeRegexFor compiles the key of a :regex match under comparator. The
// pattern is matched against the value as it is, so the match variables hold
// the value's own text, and the casemap comparators make the pattern itself
// case-insensitive, as Pigeonhole compiles it with REG_ICASE (mcht-regex.c):
// i;ascii-casemap for ASCII letters only (RFC 4790 §9.2.1), i;unicode-casemap
// for every letter. The limits and any error are about the key as written,
// whose size parseSafeRegex estimates with its classes folded as well, so the
// bound is the same under every comparator.
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

// foldASCIICase folds ASCII letters without changing non-ASCII membership.
// Source classes must be handled before the parser discards how each member
// was negated: fold a positive member, then negate it, union the members, and
// finally apply the bracket's own negation. Literal letters outside classes
// are folded in the parsed expression as before.
func foldASCIICase(pattern string, parsed *syntax.Regexp) (string, error) {
	folded, err := foldASCIIClassSources(pattern)
	if err != nil {
		return "", err
	}
	if folded != pattern {
		if parsed, err = syntax.Parse(folded, syntax.Perl); err != nil {
			return "", err
		}
	}
	if !foldASCIILetters(parsed) {
		return folded, nil
	}
	return parsed.String(), nil
}

// foldASCIIClassSources replaces each class with its ASCII-folded set. An
// explicit (?i) requests Go's Unicode folding, so classes in that scope are
// left to Go. A (?-i) scope still has the comparator's baseline ASCII folding.
// The input has already been parsed, so group syntax and escapes are valid.
func foldASCIIClassSources(pattern string) (string, error) {
	var b strings.Builder
	done := 0
	unicodeFold := false
	var scopes []bool
	for i := 0; i < len(pattern); {
		start, end := i, i
		switch pattern[i] {
		case '(':
			if strings.HasPrefix(pattern[i:], "(?") && !strings.HasPrefix(pattern[i:], "(?P<") && !strings.HasPrefix(pattern[i:], "(?<") {
				j, nextFold, negative := i+2, unicodeFold, false
				for pattern[j] != ':' && pattern[j] != ')' {
					switch pattern[j] {
					case '-':
						negative = true
					case 'i':
						nextFold = !negative
					}
					j++
				}
				if pattern[j] == ':' {
					scopes = append(scopes, unicodeFold)
				}
				unicodeFold = nextFold
				i = j + 1
				continue
			}
			scopes = append(scopes, unicodeFold)
			i++
		case ')':
			unicodeFold = scopes[len(scopes)-1]
			scopes = scopes[:len(scopes)-1]
			i++
		case '\\':
			if strings.HasPrefix(pattern[i:], `\Q`) {
				end := strings.Index(pattern[i+2:], `\E`)
				if end < 0 {
					i = len(pattern)
				} else {
					i += 2 + end + 2
				}
				continue
			}
			n, class := bracketMember(pattern[i:])
			i += n
			if class && !unicodeFold {
				end = i
			}
		case '[':
			i = bracketEnd(pattern, i) + 1
			if !unicodeFold {
				end = i
			}
		default:
			i++
		}
		if end > start {
			replacement, err := foldASCIIClassSource(pattern[start:end])
			if err != nil {
				return "", err
			}
			b.WriteString(pattern[done:start])
			b.WriteString(replacement)
			done = end
		}
	}
	if done == 0 {
		return pattern, nil
	}
	b.WriteString(pattern[done:])
	return b.String(), nil
}

type asciiClass [128]bool

// foldASCIIClassSource retains the original class's non-ASCII ranges and
// replaces just its ASCII membership. This avoids Unicode SimpleFold adding
// K/k from a Kelvin-sign member, or S/s from a long-s member.
func foldASCIIClassSource(source string) (string, error) {
	original, err := syntax.Parse(source, syntax.Perl)
	if err != nil {
		return "", err
	}
	var ascii asciiClass
	if source[0] != '[' {
		ascii, err = foldedASCIIMember(source)
	} else {
		i, negate := 1, false
		if source[i] == '^' {
			i, negate = i+1, true
		}
		for first := true; source[i] != ']' || first; first = false {
			start := i
			namedEnd := -1
			if strings.HasPrefix(source[i:], "[:") {
				namedEnd = strings.Index(source[i+2:], ":]")
			}
			if namedEnd >= 0 {
				i += 2 + namedEnd + 2
			} else {
				n, class := bracketMember(source[i:])
				i += n
				if !class && source[i] == '-' && source[i+1] != ']' {
					n, _ = bracketMember(source[i+1:])
					i += 1 + n
				}
			}
			member, memberErr := foldedASCIIMember(source[start:i])
			if memberErr != nil {
				return "", memberErr
			}
			for r := range ascii {
				ascii[r] = ascii[r] || member[r]
			}
		}
		if negate {
			for r := range ascii {
				ascii[r] = !ascii[r]
			}
		}
	}
	if err != nil {
		return "", err
	}
	ranges := make([]rune, 0, 2*len(ascii))
	for r := 0; r < len(ascii); {
		if !ascii[r] {
			r++
			continue
		}
		lo := r
		for r+1 < len(ascii) && ascii[r+1] {
			r++
		}
		ranges = append(ranges, rune(lo), rune(r))
		r++
	}
	for i, originalRanges := 0, classRanges(original); i+1 < len(originalRanges); i += 2 {
		lo, hi := originalRanges[i], originalRanges[i+1]
		if hi < 128 {
			continue
		}
		if lo < 128 {
			lo = 128
		}
		if n := len(ranges); n > 0 && lo <= ranges[n-1]+1 {
			if hi > ranges[n-1] {
				ranges[n-1] = hi
			}
		} else {
			ranges = append(ranges, lo, hi)
		}
	}
	return (&syntax.Regexp{Op: syntax.OpCharClass, Rune: ranges}).String(), nil
}

// foldedASCIIMember closes a positive member under ASCII case before applying
// its own inverse. A source bracket can union these sets before negating them.
func foldedASCIIMember(source string) (asciiClass, error) {
	negate := false
	switch {
	case strings.HasPrefix(source, "[:^"):
		negate, source = true, "[:"+source[3:]
	case strings.HasPrefix(source, `\p`) || strings.HasPrefix(source, `\P`):
		negate = source[1] == 'P'
		source = `\p` + source[2:]
		if strings.HasPrefix(source, `\p{^`) {
			negate, source = !negate, `\p{`+source[4:]
		}
	case source == `\D` || source == `\S` || source == `\W`:
		negate, source = true, strings.ToLower(source)
	}
	if source[0] == '^' {
		source = `\` + source
	}
	parsed, err := syntax.Parse("["+source+"]", syntax.Perl)
	if err != nil {
		return asciiClass{}, err
	}
	ranges := classRanges(parsed)
	var ascii asciiClass
	for r := range ascii {
		other := rune(r)
		if isASCIILetter(other) {
			other ^= 0x20
		}
		ascii[r] = inRanges(ranges, rune(r)) || inRanges(ranges, other)
		if negate {
			ascii[r] = !ascii[r]
		}
	}
	return ascii, nil
}

// classRanges returns the rune set of a parsed single-character class. The
// parser can simplify full/empty classes or a complete SimpleFold cycle into
// special operators; expanding that cycle here preserves its original set.
func classRanges(re *syntax.Regexp) []rune {
	switch re.Op {
	case syntax.OpCharClass:
		return re.Rune
	case syntax.OpAnyChar:
		return []rune{0, unicode.MaxRune}
	case syntax.OpAnyCharNotNL:
		return []rune{0, '\n' - 1, '\n' + 1, unicode.MaxRune}
	case syntax.OpLiteral:
		r := re.Rune[0]
		ranges := []rune{r, r}
		if re.Flags&syntax.FoldCase != 0 {
			for other := unicode.SimpleFold(r); other != r; other = unicode.SimpleFold(other) {
				ranges = append(ranges, other, other)
			}
			sort.Slice(ranges, func(i, j int) bool { return ranges[i] < ranges[j] })
		}
		return ranges
	default:
		return nil
	}
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
