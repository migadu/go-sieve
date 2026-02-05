package interp

import (
	"context"
	"fmt"
	"regexp"
	"time"
)

// RegexLimits defines safety limits for regex execution
type RegexLimits struct {
	// MaxExecTime is the maximum time allowed for regex execution
	MaxExecTime time.Duration
	// MaxPatternLength is the maximum allowed regex pattern length
	MaxPatternLength int
	// MaxInputLength is the maximum input text length for regex matching
	MaxInputLength int
}

// DefaultRegexLimits provides safe default limits for regex execution
var DefaultRegexLimits = RegexLimits{
	MaxExecTime:      100 * time.Millisecond,
	MaxPatternLength: 1000,
	MaxInputLength:   10000,
}

// SafeRegexMatcher provides a safe regex matcher with execution limits
type SafeRegexMatcher struct {
	pattern *regexp.Regexp
	limits  RegexLimits
}

// CompileSafeRegex compiles a regex pattern with safety checks
func CompileSafeRegex(pattern string, limits RegexLimits) (*SafeRegexMatcher, error) {
	// Check pattern length
	if len(pattern) > limits.MaxPatternLength {
		return nil, fmt.Errorf("regex pattern too long: %d > %d", len(pattern), limits.MaxPatternLength)
	}

	// Compile with timeout
	ctx, cancel := context.WithTimeout(context.Background(), limits.MaxExecTime)
	defer cancel()

	compiled := make(chan *regexp.Regexp, 1)
	compileErr := make(chan error, 1)

	go func() {
		re, err := regexp.Compile(pattern)
		if err != nil {
			compileErr <- err
		} else {
			compiled <- re
		}
	}()

	select {
	case re := <-compiled:
		return &SafeRegexMatcher{
			pattern: re,
			limits:  limits,
		}, nil
	case err := <-compileErr:
		return nil, fmt.Errorf("regex compile error: %w", err)
	case <-ctx.Done():
		return nil, fmt.Errorf("regex compilation timeout")
	}
}

// Match performs safe regex matching with timeout and input size limits
func (m *SafeRegexMatcher) Match(input string) (bool, error) {
	// Check input length
	if len(input) > m.limits.MaxInputLength {
		return false, fmt.Errorf("input too long for regex: %d > %d", len(input), m.limits.MaxInputLength)
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(context.Background(), m.limits.MaxExecTime)
	defer cancel()

	result := make(chan bool, 1)
	matchErr := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				matchErr <- fmt.Errorf("regex panic: %v", r)
			}
		}()
		result <- m.pattern.MatchString(input)
	}()

	select {
	case matched := <-result:
		return matched, nil
	case err := <-matchErr:
		return false, err
	case <-ctx.Done():
		return false, fmt.Errorf("regex execution timeout")
	}
}

// FindSubmatch performs safe regex matching with capture groups
func (m *SafeRegexMatcher) FindSubmatch(input string) ([]string, error) {
	// Check input length
	if len(input) > m.limits.MaxInputLength {
		return nil, fmt.Errorf("input too long for regex: %d > %d", len(input), m.limits.MaxInputLength)
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(context.Background(), m.limits.MaxExecTime)
	defer cancel()

	result := make(chan []string, 1)
	matchErr := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				matchErr <- fmt.Errorf("regex panic: %v", r)
			}
		}()
		matches := m.pattern.FindStringSubmatch(input)
		result <- matches
	}()

	select {
	case matches := <-result:
		return matches, nil
	case err := <-matchErr:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("regex execution timeout")
	}
}

// Pattern returns the underlying regex pattern string
func (m *SafeRegexMatcher) Pattern() string {
	return m.pattern.String()
}
