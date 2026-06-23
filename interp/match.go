package interp

import (
	"context"
	"strings"
)

func foldASCII(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}

func patternToRegex(pattern string, caseFold bool) string {
	result := strings.Builder{}
	result.WriteString(`(?s)`)
	if caseFold {
		result.WriteString(`(?i)`)
	}
	result.WriteRune('^')
	escaped := false
	for _, chr := range pattern {
		if !escaped {
			switch chr {
			case '\\':
				escaped = true
			case '?':
				result.WriteString(`(.)`)
			case '*':
				result.WriteString(`(.*?)`)
			case '.', '+', '(', ')', '|', '[', ']', '{', '}', '^', '$':
				result.WriteRune('\\')
				fallthrough
			default:
				result.WriteRune(chr)
			}
		} else {
			switch chr {
			case '\\', '?', '*', '.', '+', '(', ')', '|', '[', ']', '{', '}', '^', '$':
				result.WriteRune('\\')
				fallthrough
			default:
				result.WriteRune(chr)
			}

			escaped = false
		}
	}

	// Such regex won't compile.
	if escaped {
		return result.String()
	}

	result.WriteRune('$')

	return result.String()
}

type CompiledMatcher func(ctx context.Context, value string) (bool, []string, error)

// compileMatcher returns a function that will check whether pre-defined pattern matches the passed
// value. It is preferable to use compileMatcher over matchOctet, matchUnicode if
// pattern does not change often (e.g. does not depend on any variables).
//
// The wildcard pattern is compiled once through the bounded executor
// (SafeRegexMatcher), so the per-match execution is pattern/input/time bounded
// and honours the caller's context.
func compileMatcher(pattern string, octet bool, caseFold bool) (CompiledMatcher, error) {
	matcher, err := compileBoundedMatcher(pattern, octet, caseFold)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context, value string) (bool, []string, error) {
		matches, err := matcher.FindSubmatch(ctx, value)
		if err != nil {
			return false, nil, err
		}
		return len(matches) != 0, matches, nil
	}, nil
}

// compileBoundedMatcher converts a Sieve wildcard pattern to a regex and wraps
// it in a bounded executor, using the byte-oriented binaryregexp engine for
// octet comparators and the Unicode stdlib regexp engine otherwise.
func compileBoundedMatcher(pattern string, octet bool, caseFold bool) (*SafeRegexMatcher, error) {
	regexStr := patternToRegex(pattern, caseFold)
	if octet {
		return compileSafeBinaryRegex(regexStr, DefaultRegexLimits)
	}
	return CompileSafeRegex(regexStr, DefaultRegexLimits)
}

func matchOctet(ctx context.Context, pattern, value string, caseFold bool) (bool, []string, error) {
	matcher, err := compileSafeBinaryRegex(patternToRegex(pattern, caseFold), DefaultRegexLimits)
	if err != nil {
		return false, nil, err
	}

	matches, err := matcher.FindSubmatch(ctx, value)
	if err != nil {
		return false, nil, err
	}
	return len(matches) != 0, matches, nil
}

func matchUnicode(ctx context.Context, pattern, value string, caseFold bool) (bool, []string, error) {
	matcher, err := CompileSafeRegex(patternToRegex(pattern, caseFold), DefaultRegexLimits)
	if err != nil {
		return false, nil, err
	}

	matches, err := matcher.FindSubmatch(ctx, value)
	if err != nil {
		return false, nil, err
	}
	return len(matches) != 0, matches, nil
}
