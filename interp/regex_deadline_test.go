package interp

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"rsc.io/binaryregexp"
)

// A match costs about the size of its program times the length of its input.
// x.{0,1000}y compiles to some 2000 instructions, and every 'x' of the input
// starts a thread that stays alive for the next 1000 characters, so against
// MaxInputLength bytes of 'x' the match takes seconds. Its deadline
// (MaxExecTime, or the context's) must stop the match itself, not only end the
// wait for it: no match may go on using CPU after FindSubmatch has returned.
const slowKey = `x.{0,1000}y`

var slowInput = strings.Repeat("x", DefaultRegexLimits.MaxInputLength)

// findSubmatchStopped runs m.FindSubmatch(ctx, slowInput), which ctx must stop
// within 50ms, and returns its error after checking that it returned close to
// that and that no goroutine goes on running the match.
func findSubmatchStopped(t *testing.T, ctx context.Context, m *SafeRegexMatcher) error {
	t.Helper()
	start := time.Now()
	matches, err := m.FindSubmatch(ctx, slowInput)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("FindSubmatch returned after %v, want close to its 50ms deadline", elapsed)
	}
	if err == nil {
		t.Fatalf("FindSubmatch = %q, want an error", matches)
	}
	for wait := time.Now().Add(500 * time.Millisecond); matchRunning(); {
		if time.Now().After(wait) {
			t.Fatalf("a goroutine is still running the match 500ms after FindSubmatch returned (%v)", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return err
}

// matchRunning reports whether a goroutine is running a regexp or binaryregexp
// match.
func matchRunning() bool {
	buf := make([]byte, 1<<20)
	return bytes.Contains(buf[:runtime.Stack(buf, true)], []byte(".(*Regexp).doExecute("))
}

func TestFindSubmatchStopsAtDeadline(t *testing.T) {
	for _, tc := range []struct {
		name    string
		compile func(string, RegexLimits) (*SafeRegexMatcher, error)
	}{
		{"regexp", CompileSafeRegex},
		{"binaryregexp", compileSafeBinaryRegex},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := tc.compile(slowKey, DefaultRegexLimits)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			ctx := ContextWithRegexLimits(context.Background(), RegexLimits{MaxExecTime: 50 * time.Millisecond})
			if err := findSubmatchStopped(t, ctx, m); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("FindSubmatch: %v, want context.DeadlineExceeded", err)
			}
		})
	}
}

// The context's own deadline or cancellation (the script's) stops it as well.
func TestFindSubmatchStopsWithContext(t *testing.T) {
	m, err := CompileSafeRegex(slowKey, DefaultRegexLimits)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	t.Run("deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		if err := findSubmatchStopped(t, ctx, m); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("FindSubmatch: %v, want context.DeadlineExceeded", err)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		timer := time.AfterFunc(50*time.Millisecond, cancel)
		defer timer.Stop()
		if err := findSubmatchStopped(t, ctx, m); !errors.Is(err, context.Canceled) {
			t.Fatalf("FindSubmatch: %v, want context.Canceled", err)
		}
	})
}

// A large input is matched with the deadline; that must not change what it
// matches: the submatches are those of the engine's own string match, for both
// engines, with groups that take no part, a match at the end of the input,
// and non-ASCII and invalid UTF-8 input.
func TestFindSubmatchLargeInputSameAsEngine(t *testing.T) {
	pad := strings.Repeat("abc déf\n", syncMatchInputThreshold/8+1)
	inputs := []string{
		pad + "needle 42 end",
		pad + "needle",
		"needle 7 " + pad,
		pad + "\xff\xfe needle \xc3 9",
		pad,
	}
	keys := []string{
		`needle`,
		`needle (\d+)?`,
		`(?:(needle)|(haystack)) ?(\d*)`,
		`needle$`,
		`(?s)^(.*)needle(.*)$`,
		`(?i)NEEDLE`,
		`\bdéf\b`,
		`(?m)^needle`,
		`[^a-z\s]+`,
	}
	ctx := context.Background()
	for _, key := range keys {
		std := regexp.MustCompile(key)
		bin := binaryregexp.MustCompile(key)
		stdSafe, err := CompileSafeRegex(key, DefaultRegexLimits)
		if err != nil {
			t.Fatalf("CompileSafeRegex(%q): %v", key, err)
		}
		binSafe, err := compileSafeBinaryRegex(key, DefaultRegexLimits)
		if err != nil {
			t.Fatalf("compileSafeBinaryRegex(%q): %v", key, err)
		}
		for i, input := range inputs {
			got, err := stdSafe.FindSubmatch(ctx, input)
			if err != nil {
				t.Fatalf("regexp %q, input %d: %v", key, i, err)
			}
			if want := std.FindStringSubmatch(input); !reflect.DeepEqual(got, want) {
				t.Errorf("regexp %q, input %d: FindSubmatch = %q, want %q", key, i, got, want)
			}
			got, err = binSafe.FindSubmatch(ctx, input)
			if err != nil {
				t.Fatalf("binaryregexp %q, input %d: %v", key, i, err)
			}
			if want := bin.FindStringSubmatch(input); !reflect.DeepEqual(got, want) {
				t.Errorf("binaryregexp %q, input %d: FindSubmatch = %q, want %q", key, i, got, want)
			}
		}
	}
}
