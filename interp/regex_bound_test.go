package interp

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestSafeRegexMatcher_TruncatesOversizedInput proves that input longer than
// MaxInputLength is truncated (safe degradation) rather than matched in full or
// rejected — this is what actually bounds the CPU cost of a single match.
func TestSafeRegexMatcher_TruncatesOversizedInput(t *testing.T) {
	limits := RegexLimits{MaxExecTime: 100 * time.Millisecond, MaxPatternLength: 100, MaxInputLength: 8}
	m, err := CompileSafeRegex("b$", limits)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	// 8 'a's + 'b' = 9 bytes; truncated to 8 ("aaaaaaaa") drops the trailing 'b'.
	got, err := m.FindSubmatch(context.Background(), strings.Repeat("a", 8)+"b")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != nil {
		t.Errorf("expected no match after truncation, got %v", got)
	}

	// 7 'a's + 'b' = 8 bytes, within the cap, so the 'b' survives and matches.
	got, err = m.FindSubmatch(context.Background(), strings.Repeat("a", 7)+"b")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got == nil {
		t.Errorf("expected match within cap, got none")
	}
}

// TestCompileSafeRegex_RejectsTooLongPattern proves the pattern-length cap is
// enforced at compile time for the :regex path.
func TestCompileSafeRegex_RejectsTooLongPattern(t *testing.T) {
	limits := RegexLimits{MaxExecTime: 100 * time.Millisecond, MaxPatternLength: 10, MaxInputLength: 100}
	if _, err := CompileSafeRegex(strings.Repeat("a", 11), limits); err == nil {
		t.Fatal("expected error for too-long pattern")
	}
}

// TestCompileMatcher_RejectsOversizedPattern proves the same cap protects the
// :matches wildcard path: a glob whose expanded regex exceeds MaxPatternLength
// fails to compile (surfaced as a malformed pattern at setKey time).
func TestCompileMatcher_RejectsOversizedPattern(t *testing.T) {
	// Each '*' expands to "(.*?)" (5 chars), so 300 stars > the 1000-char cap.
	if _, err := compileMatcher(strings.Repeat("*", 300), false, false); err == nil {
		t.Fatal("expected compile error for oversized :matches pattern")
	}
}

// TestSafeRegexMatcher_RespectsCancelledContext proves the script's execution
// deadline is honoured: a cancelled context aborts the match promptly instead
// of running unbounded.
func TestSafeRegexMatcher_RespectsCancelledContext(t *testing.T) {
	m, err := CompileSafeRegex("^(.*)$", DefaultRegexLimits)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Use an input above the sync threshold so the guarded path is selected.
	if _, err := m.FindSubmatch(ctx, strings.Repeat("a", syncMatchInputThreshold+1)); err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

// TestSafeRegexMatcher_GuardedPathMatches proves the goroutine-guarded path
// (large inputs) still returns correct results when the deadline is not hit.
func TestSafeRegexMatcher_GuardedPathMatches(t *testing.T) {
	m, err := CompileSafeRegex("needle", DefaultRegexLimits)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	input := strings.Repeat("x", syncMatchInputThreshold*2) + "needle"
	got, err := m.FindSubmatch(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got == nil {
		t.Error("expected match on large input via guarded path")
	}
}

// TestMatch_CaptureGroupsPreserved is a regression guard: bounding :matches must
// not change wildcard capture-group semantics, for both the Unicode (regexp) and
// octet (binaryregexp) engines.
func TestMatch_CaptureGroupsPreserved(t *testing.T) {
	for _, tc := range []struct {
		name  string
		octet bool
	}{
		{"unicode", false},
		{"octet", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var (
				ok      bool
				matches []string
				err     error
			)
			if tc.octet {
				ok, matches, err = matchOctet(context.Background(), "*-*", "foo-bar", false)
			} else {
				ok, matches, err = matchUnicode(context.Background(), "*-*", "foo-bar", false)
			}
			if err != nil {
				t.Fatalf("match: %v", err)
			}
			if !ok {
				t.Fatal("expected match")
			}
			if len(matches) != 3 || matches[1] != "foo" || matches[2] != "bar" {
				t.Errorf("unexpected capture groups: %#v", matches)
			}
		})
	}
}
