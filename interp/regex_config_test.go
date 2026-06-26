package interp

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestFindSubmatch_ContextOverridesExecTime verifies the per-match soft execution
// wait (MaxExecTime) is taken from the context when present, so callers can configure
// the regex "worktime" per execution instead of being fixed at the package default.
func TestFindSubmatch_ContextOverridesExecTime(t *testing.T) {
	m, err := CompileSafeRegex("(?s).*NEEDLE.*", DefaultRegexLimits)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	// Larger than syncMatchInputThreshold (so the guarded/timed path runs) but within
	// MaxInputLength (so no truncation interferes).
	big := strings.Repeat("x", 200*1024)

	// An impossibly small budget from the context must force the soft timeout.
	ctx := ContextWithRegexLimits(context.Background(), RegexLimits{MaxExecTime: time.Nanosecond})
	if _, err := m.FindSubmatch(ctx, big); err == nil {
		t.Fatal("expected timeout with 1ns MaxExecTime context override, got nil error")
	}

	// A generous budget from the context lets the bounded match complete.
	ctx = ContextWithRegexLimits(context.Background(), RegexLimits{MaxExecTime: 5 * time.Second})
	if _, err := m.FindSubmatch(ctx, big); err != nil {
		t.Fatalf("expected success with generous MaxExecTime override, got %v", err)
	}
}

// TestFindSubmatch_ContextOverridesInputLength verifies MaxInputLength truncation is
// also taken from the context.
func TestFindSubmatch_ContextOverridesInputLength(t *testing.T) {
	m, err := CompileSafeRegex("NEEDLE", DefaultRegexLimits)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	// Needle sits beyond the context's tiny input cap, so it is truncated away.
	body := strings.Repeat("x", 4096) + "NEEDLE"
	ctx := ContextWithRegexLimits(context.Background(), RegexLimits{MaxInputLength: 1024, MaxExecTime: time.Second})
	matches, err := m.FindSubmatch(ctx, body)
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	if matches != nil {
		t.Fatalf("expected no match after context MaxInputLength truncation, got %v", matches)
	}
}

// TestEffectiveRegexLimits_FillsZeroFields verifies partial overrides inherit defaults.
func TestEffectiveRegexLimits_FillsZeroFields(t *testing.T) {
	got := EffectiveRegexLimits(RegexLimits{MaxExecTime: 2 * time.Second})
	if got.MaxExecTime != 2*time.Second {
		t.Errorf("MaxExecTime = %v, want 2s (override preserved)", got.MaxExecTime)
	}
	if got.MaxInputLength != DefaultRegexLimits.MaxInputLength {
		t.Errorf("MaxInputLength = %d, want default %d", got.MaxInputLength, DefaultRegexLimits.MaxInputLength)
	}
	if got.MaxPatternLength != DefaultRegexLimits.MaxPatternLength {
		t.Errorf("MaxPatternLength = %d, want default %d", got.MaxPatternLength, DefaultRegexLimits.MaxPatternLength)
	}
}
