package interp

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"regexp/syntax"
	"time"
	"unicode/utf8"

	"rsc.io/binaryregexp"
)

// RegexLimits defines safety limits for regex/pattern execution.
type RegexLimits struct {
	// MaxExecTime is the maximum time a single match of an input over 1 KB
	// (syncMatchInputThreshold) may run. The match stops at it, or at the
	// context's deadline if that comes first, and returns an error, so it
	// bounds the match's CPU and not only the caller's wait (see
	// deadlineReader).
	MaxExecTime time.Duration
	// MaxPatternLength is the maximum length of a pattern, in bytes, as
	// written.
	MaxPatternLength int
	// MaxProgramSize is the maximum size of the program a :regex pattern
	// compiles to, in instructions, as estimated from the parsed pattern
	// before it is compiled. Counted repetition multiplies what it repeats,
	// so without this bound a pattern of MaxPatternLength bytes can compile
	// to about a million instructions (some 45 MB). A compiled instruction
	// keeps about 50 bytes, and a character class counts one instruction
	// more for every 8 range ends it holds (see programSize).
	MaxProgramSize int
	// MaxInputLength is the maximum input length fed to the matcher. Longer
	// input is truncated to this length before matching (safe degradation,
	// rather than failing the whole script).
	MaxInputLength int
}

// DefaultRegexLimits provides safe default limits for regex execution. These
// apply to both the :regex match type and the :matches wildcard match type.
var DefaultRegexLimits = RegexLimits{
	MaxExecTime:      100 * time.Millisecond,
	MaxPatternLength: 1000,
	MaxProgramSize:   10000,
	MaxInputLength:   256 * 1024,
}

// EffectiveRegexLimits fills any unset (zero) field of l from DefaultRegexLimits, so a
// caller can override a single limit (for example MaxExecTime) and inherit the safe
// defaults for the rest.
func EffectiveRegexLimits(l RegexLimits) RegexLimits {
	if l.MaxExecTime <= 0 {
		l.MaxExecTime = DefaultRegexLimits.MaxExecTime
	}
	if l.MaxPatternLength <= 0 {
		l.MaxPatternLength = DefaultRegexLimits.MaxPatternLength
	}
	if l.MaxProgramSize <= 0 {
		l.MaxProgramSize = DefaultRegexLimits.MaxProgramSize
	}
	if l.MaxInputLength <= 0 {
		l.MaxInputLength = DefaultRegexLimits.MaxInputLength
	}
	return l
}

type regexLimitsCtxKey struct{}

// ContextWithRegexLimits returns a context carrying the regex limits to apply to
// matches executed under it. Script.Execute installs the script's effective limits
// here so a single match's input truncation (MaxInputLength) and execution time
// (MaxExecTime) are configurable per execution rather than fixed at the package
// default. MaxPatternLength and MaxProgramSize are compile-time bounds and are not
// read from the context.
func ContextWithRegexLimits(ctx context.Context, limits RegexLimits) context.Context {
	return context.WithValue(ctx, regexLimitsCtxKey{}, limits)
}

func regexLimitsFromContext(ctx context.Context) (RegexLimits, bool) {
	l, ok := ctx.Value(regexLimitsCtxKey{}).(RegexLimits)
	return l, ok
}

// syncMatchInputThreshold is the input size up to which a match runs on the
// string itself, without a deadline. Header, address, and short-string tests
// are always well under this, so they keep the engines' fastest paths
// (backtracking, literal-prefix search) and avoid the timer; only large
// inputs (e.g. message bodies via the body extension) are read through a
// deadlineReader.
const syncMatchInputThreshold = 1024

// findSubmatchFunc runs a compiled matcher against a value and returns the
// submatches (nil if there is no match). With a nil in it matches value as a
// string; otherwise it reads value from in, which ends it at the match's
// deadline. It abstracts over the stdlib regexp and binaryregexp engines so
// the bounded executor stays engine-agnostic.
type findSubmatchFunc func(value string, in *deadlineReader) []string

func regexpFindSubmatch(re *regexp.Regexp) findSubmatchFunc {
	return func(value string, in *deadlineReader) []string {
		if in == nil {
			return re.FindStringSubmatch(value)
		}
		return submatchStrings(value, re.FindReaderSubmatchIndex(in))
	}
}

func binaryRegexpFindSubmatch(re *binaryregexp.Regexp) findSubmatchFunc {
	return func(value string, in *deadlineReader) []string {
		if in == nil {
			return re.FindStringSubmatch(value)
		}
		return submatchStrings(value, re.FindReaderSubmatchIndex(in))
	}
}

// submatchStrings returns the submatches of value that the index pairs loc
// locate, as FindStringSubmatch returns them: "" for a group that took no
// part in the match.
func submatchStrings(value string, loc []int) []string {
	if loc == nil {
		return nil
	}
	matches := make([]string, len(loc)/2)
	for i := range matches {
		if loc[2*i] >= 0 {
			matches[i] = value[loc[2*i]:loc[2*i+1]]
		}
	}
	return matches
}

// deadlineCheckBytes is how much input a match reads between two looks at its
// deadline. A look costs a few nanoseconds, as much as a step of a small
// program, so it is not taken at every character. A step costs about the size
// of the program, some 50 microseconds for 10000 instructions, so the match
// still stops within a few milliseconds of its deadline.
const deadlineCheckBytes = 64

// deadlineReader gives a regexp engine its input one character at a time: a
// rune to regexp (io.RuneReader), a byte to binaryregexp (io.ByteReader). A
// match costs about the size of its program times the length of its input,
// seconds for a large key against a large body, and Go's regexp cannot be
// interrupted, but it asks for each character as it goes. Once done is closed
// the reader ends the input there, so the match stops at its deadline instead
// of running to the end of the input. stopped records that it did: the match
// result is then meaningless.
type deadlineReader struct {
	s       string
	pos     int
	check   int // where done is next looked at
	done    <-chan struct{}
	stopped bool
}

// end reports whether the input ends at pos: at its end or at the deadline.
func (r *deadlineReader) end() bool {
	if r.stopped || r.pos >= len(r.s) {
		return true
	}
	if r.pos >= r.check {
		select {
		case <-r.done:
			r.stopped = true
			return true
		default:
		}
		r.check = r.pos + deadlineCheckBytes
	}
	return false
}

func (r *deadlineReader) ReadRune() (rune, int, error) {
	if r.end() {
		return 0, 0, io.EOF
	}
	c, n := rune(r.s[r.pos]), 1
	if c >= utf8.RuneSelf {
		c, n = utf8.DecodeRuneInString(r.s[r.pos:])
	}
	r.pos += n
	return c, n, nil
}

func (r *deadlineReader) ReadByte() (byte, error) {
	if r.end() {
		return 0, io.EOF
	}
	c := r.s[r.pos]
	r.pos++
	return c, nil
}

// SafeRegexMatcher wraps a compiled matcher with execution limits. It is
// backend-agnostic: the underlying engine may be stdlib regexp (Unicode) or
// binaryregexp (octet / byte-oriented), preserving each engine's match
// semantics.
type SafeRegexMatcher struct {
	find    findSubmatchFunc
	pattern string
	limits  RegexLimits
	// size is the estimated size of the compiled program, in instructions
	// (programSize); 0 for the binaryregexp :matches path.
	size       int
	cacheBytes int // conservative retained cache charge; no parsed tree is kept
}

// CompileSafeRegex compiles a pattern with the stdlib regexp engine
// (Unicode-aware) and applies the supplied safety limits. Used for the :regex
// match type and the Unicode :matches path.
func CompileSafeRegex(pattern string, limits RegexLimits) (*SafeRegexMatcher, error) {
	parsed, size, err := parseSafeRegex(pattern, limits)
	if err != nil {
		return nil, err
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("regex compile error: %w", err)
	}
	return &SafeRegexMatcher{find: regexpFindSubmatch(re), pattern: pattern, limits: limits, size: size, cacheBytes: regexCacheCost(pattern, pattern, parsed)}, nil
}

// checkRegex returns the error CompileSafeRegex returns for pattern under
// limits, without compiling it: regexp.Compile refuses only what
// syntax.Parse refuses with the Perl flags.
func checkRegex(pattern string, limits RegexLimits) error {
	_, _, err := parseSafeRegex(pattern, limits)
	return err
}

// parseSafeRegex parses pattern as regexp.Compile does and refuses it when it
// is longer than MaxPatternLength or when the program it compiles to would
// be larger than MaxProgramSize (0 for the default). Parsing does not expand
// counted repetition, so the refusal costs no more than the pattern's length,
// however large its program would be. It returns the parsed pattern and the
// estimated size of its program.
func parseSafeRegex(pattern string, limits RegexLimits) (*syntax.Regexp, int, error) {
	if len(pattern) > limits.MaxPatternLength {
		return nil, 0, fmt.Errorf("regex pattern too long: %d > %d", len(pattern), limits.MaxPatternLength)
	}
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil, 0, fmt.Errorf("regex compile error: %w", err)
	}
	maxSize := limits.MaxProgramSize
	if maxSize <= 0 {
		maxSize = DefaultRegexLimits.MaxProgramSize
	}
	size := programSize(re)
	if size <= maxSize {
		// The casemap comparators compile the pattern case-insensitive,
		// which adds the other case of each letter to a class: [A-Z] holds
		// 2 range ends, folded 8, and a range of Latin-1 and Latin
		// Extended-A letters folds to some 50. The bound is checked on both,
		// so that it is the same under every comparator.
		if folded, err := syntax.Parse(pattern, syntax.Perl|syntax.FoldCase); err == nil {
			if foldedSize := programSize(folded); foldedSize > size {
				size = foldedSize
			}
		}
	}
	if size > maxSize {
		return nil, 0, fmt.Errorf("regex too complex: compiles to about %d instructions > %d", size, maxSize)
	}
	return re, size, nil
}

// programSize estimates the number of instructions re compiles to, as
// regexp/syntax estimates it for its own limit of some 3 million
// (parser.calcSize): counted repetition multiplies the size of what it
// repeats. The parser refuses repetition nested to more than 1000 copies of
// what it repeats, and a tree deeper than 1000, so neither the size nor the
// recursion can grow out of bounds.
//
// It also counts what a character class keeps, which regexp/syntax does not:
// regexp compiles a program of under 1000 instructions that is anchored at
// both ends to a one-pass program as well, which copies the class into every
// instruction that matches it, some 6 bytes per range end. So a class counts
// one instruction more for every 8 range ends: \pL holds 1318, and ^\pL{990}$
// keeps some 8 MB, as much as 160000 instructions do.
func programSize(re *syntax.Regexp) int {
	size := 0
	switch re.Op {
	case syntax.OpLiteral:
		size = len(re.Rune)
	case syntax.OpCharClass:
		size = 1 + len(re.Rune)/8
	case syntax.OpCapture, syntax.OpStar:
		size = 2 + programSize(re.Sub[0])
	case syntax.OpPlus, syntax.OpQuest:
		size = 1 + programSize(re.Sub[0])
	case syntax.OpConcat:
		for _, sub := range re.Sub {
			size += programSize(sub)
		}
	case syntax.OpAlternate:
		for _, sub := range re.Sub {
			size += programSize(sub)
		}
		size += len(re.Sub) - 1
	case syntax.OpRepeat:
		sub := programSize(re.Sub[0])
		switch {
		case re.Max == -1 && re.Min == 0:
			size = 2 + sub // x*
		case re.Max == -1:
			size = 1 + re.Min*sub // xx+
		default:
			size = re.Max*sub + re.Max - re.Min // x{2,5} = xx(x(x(x)?)?)?
		}
	}
	if size < 1 {
		size = 1
	}
	return size
}

// compileSafeBinaryRegex compiles a pattern with the binaryregexp engine
// (byte-oriented), preserving octet-comparator semantics for the :matches
// path while applying the same safety limits as CompileSafeRegex.
func compileSafeBinaryRegex(pattern string, limits RegexLimits) (*SafeRegexMatcher, error) {
	if len(pattern) > limits.MaxPatternLength {
		return nil, fmt.Errorf("regex pattern too long: %d > %d", len(pattern), limits.MaxPatternLength)
	}
	re, err := binaryregexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("regex compile error: %w", err)
	}
	return &SafeRegexMatcher{find: binaryRegexpFindSubmatch(re), pattern: pattern, limits: limits}, nil
}

// FindSubmatch runs the matcher against input with input truncation and a
// ctx-aware deadline. Input longer than MaxInputLength is truncated; the
// supplied ctx (e.g. the script's execution deadline) bounds a match of input
// over syncMatchInputThreshold in addition to MaxExecTime, whichever comes
// first, and a match stopped by either returns an error.
func (m *SafeRegexMatcher) FindSubmatch(ctx context.Context, input string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// MaxInputLength (truncation) and MaxExecTime (deadline) are runtime concerns and
	// may be overridden per execution via the context (Script.Execute installs the
	// script's effective limits with ContextWithRegexLimits). Fall back to the limits
	// captured at compile time when the context carries none. MaxPatternLength was
	// already enforced at compile time and is not re-read here.
	maxInput := m.limits.MaxInputLength
	maxExec := m.limits.MaxExecTime
	if l, ok := regexLimitsFromContext(ctx); ok {
		if l.MaxInputLength > 0 {
			maxInput = l.MaxInputLength
		}
		if l.MaxExecTime > 0 {
			maxExec = l.MaxExecTime
		}
	}

	if len(input) > maxInput {
		input = input[:maxInput]
	}

	// Fast path: small inputs (headers, addresses, short strings) match in
	// well under a millisecond, so match the string itself and skip the
	// timer and reader overhead.
	if len(input) <= syncMatchInputThreshold {
		return m.find(input, nil), nil
	}

	// A large input is read through a deadlineReader, which ends it at the
	// deadline, so a single match can't outrun the script budget: it stops
	// there rather than going on using CPU after we return.
	matchCtx, cancel := context.WithTimeout(ctx, maxExec)
	defer cancel()

	in := &deadlineReader{s: input, done: matchCtx.Done()}
	matches := m.find(input, in)
	if in.stopped {
		return nil, fmt.Errorf("regex execution timeout: %w", matchCtx.Err())
	}
	return matches, nil
}

// Match reports whether input matches, applying the same bounds as
// FindSubmatch.
func (m *SafeRegexMatcher) Match(ctx context.Context, input string) (bool, error) {
	matches, err := m.FindSubmatch(ctx, input)
	if err != nil {
		return false, err
	}
	return matches != nil, nil
}

// Pattern returns the underlying compiled pattern string.
func (m *SafeRegexMatcher) Pattern() string {
	return m.pattern
}
