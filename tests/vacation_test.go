package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/foxcpp/go-sieve"
	"github.com/foxcpp/go-sieve/interp"
)

func TestVacation(t *testing.T) {
	// Test basic vacation functionality using inline script
	scriptText := `
	require ["vacation"];
	
	# Test basic vacation command
	test "Basic vacation" {
		vacation "I'm on vacation.";
		
		if not exists "vacation-response" {
			test_fail "No vacation response was added";
		}
		
		if not header :contains "vacation-response.subject" "Automated reply" {
			test_fail "Unexpected subject in vacation response";
		}
		
		if not header :contains "vacation-response.body" "I'm on vacation." {
			test_fail "Unexpected body in vacation response";
		}
	}
	
	# Test vacation command with all parameters
	test "Vacation with parameters" {
		vacation :days 14 :subject "Out of Office" :from "me@example.com" 
			:addresses ["me@example.com", "me2@example.com"] 
			:mime :handle "vacation-001" 
			"I'm on vacation until next week.";
		
		if not exists "vacation-response" {
			test_fail "No vacation response was added";
		}
		
		if not header :contains "vacation-response.subject" "Out of Office" {
			test_fail "Unexpected subject in vacation response";
		}
		
		if not header :contains "vacation-response.body" "I'm on vacation until next week." {
			test_fail "Unexpected body in vacation response";
		}
		
		if not header :contains "vacation-response.from" "me@example.com" {
			test_fail "Unexpected from in vacation response";
		}
	}
	
	# Test that no vacation response is sent to our own addresses
	test "No vacation response to own addresses" {
		# Set envelope.from to one of our addresses
		test_set "envelope.from" "me@example.com";
		
		vacation :addresses ["me@example.com"] "I'm on vacation.";
		
		if exists "vacation-response" {
			test_fail "Vacation response was added for our own address";
		}
	}
	`

	RunDovecotTestInline(t, "", scriptText)
}

// TestVacationDirectly tests the vacation functionality directly using the Go API
func TestVacationDirectly(t *testing.T) {
	ctx := context.Background()

	// Test basic vacation command
	script := `require ["vacation"];
	
	vacation "I'm on vacation.";
	`

	opts := sieve.DefaultOptions()
	opts.Lexer.Filename = "inline"

	parsedScript, err := sieve.Load(strings.NewReader(script), opts)
	if err != nil {
		t.Fatal(err)
	}

	// Create runtime data with a static envelope
	env := interp.EnvelopeStatic{
		From: "sender@example.com",
		To:   "recipient@example.com",
	}

	data := sieve.NewRuntimeData(parsedScript, interp.DummyPolicy{}, env, interp.MessageStatic{})

	// Execute the script
	err = parsedScript.Execute(ctx, data)
	if err != nil {
		t.Fatal(err)
	}

	// Check that a vacation response was added
	if len(data.VacationResponses) != 1 {
		t.Fatalf("Expected 1 vacation response, got %d", len(data.VacationResponses))
	}

	// Check the response details
	resp, ok := data.VacationResponses["sender@example.com"]
	if !ok {
		t.Fatalf("No vacation response for sender@example.com")
	}
	if resp.Body != "I'm on vacation." {
		t.Errorf("Expected body 'I'm on vacation.', got '%s'", resp.Body)
	}
	if resp.Subject != "Automated reply" {
		t.Errorf("Expected subject 'Automated reply', got '%s'", resp.Subject)
	}
	if resp.Days != 7 {
		t.Errorf("Expected days 7, got %d", resp.Days)
	}
}
