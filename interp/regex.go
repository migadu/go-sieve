package interp

import (
	"context"
	"fmt"
	"regexp"
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
	// MaxPatternLength is the maximum allowed compiled-pattern length.
	MaxPatternLength int
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
// default. MaxPatternLength is a compile-time bound and is not read from the context.
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
}

// CompileSafeRegex compiles a pattern with the stdlib regexp engine
// (Unicode-aware) and applies the supplied safety limits. Used for the :regex
// match type and the Unicode :matches path.
func CompileSafeRegex(pattern string, limits RegexLimits) (*SafeRegexMatcher, error) {
	if len(pattern) > limits.MaxPatternLength {
		return nil, fmt.Errorf("regex pattern too long: %d > %d", len(pattern), limits.MaxPatternLength)
	}
	// regexp.Compile is linear in the (bounded) pattern length and rejects
	// programs that would expand too large, so it is self-limiting here.
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("regex compile error: %w", err)
	}
	return &SafeRegexMatcher{find: re.FindStringSubmatch, pattern: pattern, limits: limits}, nil
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
