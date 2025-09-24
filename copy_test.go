package sieve

import (
	"context"
	"testing"
)

// TestCopyExtension verifies that the :copy extension works correctly
// for both redirect and fileinto commands.
func TestCopyExtension(t *testing.T) {
	testCases := []struct {
		name       string
		script     string
		shouldFail bool
		expected   Result
	}{
		{
			name:   "redirect with :copy",
			script: `require "copy"; redirect :copy "user@example.com";`,
			expected: Result{
				Redirect:     []string{"user@example.com"},
				ImplicitKeep: true,
			},
		},
		{
			name:   "fileinto with :copy",
			script: `require ["fileinto", "copy"]; fileinto :copy "Spam";`,
			expected: Result{
				Fileinto:     []string{"Spam"},
				ImplicitKeep: true,
			},
		},
		{
			name:       "redirect :copy without require",
			script:     `redirect :copy "user@example.com";`,
			shouldFail: true,
		},
		{
			name:       "fileinto :copy without require",
			script:     `require "fileinto"; fileinto :copy "Spam";`,
			shouldFail: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			testExecute(ctx, t, tc.script, eml, tc.shouldFail, tc.expected)
		})
	}
}
