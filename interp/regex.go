package interp

import (
	"context"
	"fmt"
	"regexp"
	"regexp/syntax"
	"time"

	"rsc.io/binaryregexp"
)

// RegexLimits defines safety limits for regex/pattern execution.
type RegexLimits struct {
	// MaxExecTime is the maximum time the caller waits for a single match
	// before giving up (soft timeout). Go's regexp engine cannot be
	// interrupted mid-call, so this bounds the caller's wait, not the CPU;
	// MaxInputLength is what actually bounds the work.
	MaxExecTime time.Duration
	// MaxPatternLength is the maximum length of a pattern, in bytes, as
	// written.
	MaxPatternLength int
	// MaxProgramSize bounds a :regex pattern in weighted program-cost units:
	// instructions plus a character-class range allowance, estimated from the parsed pattern
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
// here so a single match's input truncation (MaxInputLength) and soft execution wait
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

// syncMatchInputThreshold is the input size below which a match runs
// synchronously (no goroutine/timer). Header, address, and short-string tests
// are always well under this, so they avoid the soft-timeout overhead; only
// large inputs (e.g. message bodies via the body extension) take the guarded
// path.
const syncMatchInputThreshold = 1024

// findSubmatchFunc runs a compiled matcher against a value and returns the
// submatches (nil if there is no match). It abstracts over the stdlib regexp
// and binaryregexp engines so the bounded executor stays engine-agnostic.
type findSubmatchFunc func(value string) []string

// SafeRegexMatcher wraps a compiled matcher with execution limits. It is
// backend-agnostic: the underlying engine may be stdlib regexp (Unicode) or
// binaryregexp (octet / byte-oriented), preserving each engine's match
// semantics.
type SafeRegexMatcher struct {
	find    findSubmatchFunc
	pattern string
	limits  RegexLimits
	// size is the estimated program cost from programSize, including character
	// classes; 0 for the binaryregexp :matches path.
	size int
}

// CompileSafeRegex compiles a pattern with the stdlib regexp engine
// (Unicode-aware) and applies the supplied safety limits. Used for the :regex
// match type and the Unicode :matches path.
func CompileSafeRegex(pattern string, limits RegexLimits) (*SafeRegexMatcher, error) {
	_, size, err := parseSafeRegex(pattern, limits)
	if err != nil {
		return nil, err
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("regex compile error: %w", err)
	}
	return &SafeRegexMatcher{find: re.FindStringSubmatch, pattern: pattern, limits: limits, size: size}, nil
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
	return &SafeRegexMatcher{find: re.FindStringSubmatch, pattern: pattern, limits: limits}, nil
}

// FindSubmatch runs the matcher against input with input truncation and a
// ctx-aware soft timeout. Input longer than MaxInputLength is truncated; the
// supplied ctx (e.g. the script's execution deadline) bounds the match in
// addition to MaxExecTime, whichever fires first.
func (m *SafeRegexMatcher) FindSubmatch(ctx context.Context, input string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// MaxInputLength (truncation) and MaxExecTime (soft wait) are runtime concerns and
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
	// well under a millisecond, so run synchronously and skip the
	// goroutine/timer overhead.
	if len(input) <= syncMatchInputThreshold {
		return m.find(input), nil
	}

	// Large inputs get a ctx-aware soft timeout so a single match can't
	// outrun the script budget. The match goroutine runs on the truncated
	// (bounded) input, so even if we stop waiting it completes promptly and
	// does not leak; the buffered channels keep its send non-blocking.
	matchCtx, cancel := context.WithTimeout(ctx, maxExec)
	defer cancel()

	result := make(chan []string, 1)
	matchErr := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				matchErr <- fmt.Errorf("regex panic: %v", r)
			}
		}()
		result <- m.find(input)
	}()

	select {
	case matches := <-result:
		return matches, nil
	case err := <-matchErr:
		return nil, err
	case <-matchCtx.Done():
		return nil, fmt.Errorf("regex execution timeout: %w", matchCtx.Err())
	}
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
