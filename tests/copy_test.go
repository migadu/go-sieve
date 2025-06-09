package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/foxcpp/go-sieve"
	"github.com/foxcpp/go-sieve/interp"
)

// TestCopyExtension verifies that the :copy extension works correctly
// for both redirect and fileinto commands.
func TestCopyExtension(t *testing.T) {
	ctx := context.Background()

	// Test 1: redirect :copy should work and keep implicitKeep true
	redirectScript := `
require ["copy"];

redirect :copy "user@example.com";
`
	redirectParsedScript, err := loadTestScript(redirectScript)
	if err != nil {
		t.Fatalf("Failed to load redirect script: %v", err)
	}

	redirectData := createTestRuntimeData(redirectParsedScript)

	// Execute the script
	err = redirectParsedScript.Execute(ctx, redirectData)
	if err != nil {
		t.Fatalf("redirect :copy script failed: %v", err)
	}

	if len(redirectData.RedirectAddr) != 1 {
		t.Errorf("expected 1 redirect address, got %d", len(redirectData.RedirectAddr))
	}
	if redirectData.RedirectAddr[0] != "user@example.com" {
		t.Errorf("expected redirect to user@example.com, got %s", redirectData.RedirectAddr[0])
	}
	if !redirectData.ImplicitKeep {
		t.Errorf("redirect :copy should have kept ImplicitKeep true")
	}

	// Test 2: fileinto :copy should work and keep implicitKeep true
	fileintoScript := `
require ["fileinto", "copy"];

fileinto :copy "Spam";
`
	fileintoParsedScript, err := loadTestScript(fileintoScript)
	if err != nil {
		t.Fatalf("Failed to load fileinto script: %v", err)
	}

	fileintoData := createTestRuntimeData(fileintoParsedScript)

	// Execute the script
	err = fileintoParsedScript.Execute(ctx, fileintoData)
	if err != nil {
		t.Fatalf("fileinto :copy script failed: %v", err)
	}

	if len(fileintoData.Mailboxes) != 1 {
		t.Errorf("expected 1 mailbox, got %d", len(fileintoData.Mailboxes))
	}
	if fileintoData.Mailboxes[0] != "Spam" {
		t.Errorf("expected fileinto to Spam, got %s", fileintoData.Mailboxes[0])
	}
	if !fileintoData.ImplicitKeep {
		t.Errorf("fileinto :copy should have kept ImplicitKeep true")
	}

	// Test 3: redirect :copy without require should fail
	invalidRedirectScript := `
require ["redirect"];

redirect :copy "user@example.com";
`
	_, err = loadTestScript(invalidRedirectScript)
	if err == nil {
		t.Errorf("redirect :copy without require 'copy' should have failed")
	}

	// Test 4: fileinto :copy without require should fail
	invalidFileintoScript := `
require ["fileinto"];

fileinto :copy "Spam";
`
	_, err = loadTestScript(invalidFileintoScript)
	if err == nil {
		t.Errorf("fileinto :copy without require 'copy' should have failed")
	}
}

// Helper functions
func loadTestScript(script string) (*sieve.Script, error) {
	opts := sieve.DefaultOptions()
	opts.Lexer.Filename = "inline"
	return sieve.Load(strings.NewReader(script), opts)
}

func createTestRuntimeData(script *sieve.Script) *sieve.RuntimeData {
	// Create runtime data with static envelope and message
	env := interp.EnvelopeStatic{
		From: "sender@example.com",
		To:   "recipient@example.com",
	}

	return sieve.NewRuntimeData(script, interp.DummyPolicy{}, env, interp.MessageStatic{})
}
