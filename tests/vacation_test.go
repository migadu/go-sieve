package tests

import (
	"context"
	"net/textproto"
	"slices"
	"strings"
	"testing"

	"github.com/migadu/go-sieve"
	"github.com/migadu/go-sieve/interp"
)

func TestVacation(t *testing.T) {
	testCases := []struct {
		name              string
		script            string
		envFrom           string
		subject           string // the original message's Subject field, "" for none
		expectResponse    bool
		expectedSubject   string
		expectedAddresses []string
		expectedBody      string
		expectedFrom      string
		expectedHandle    string
		expectedDays      int
		expectedRecipient string
	}{
		{
			name:              "BasicVacation",
			script:            `require ["vacation"]; vacation "I'm on vacation.";`,
			envFrom:           "sender@example.com",
			expectResponse:    true,
			expectedSubject:   "Automated reply",
			expectedBody:      "I'm on vacation.",
			expectedDays:      7,
			expectedRecipient: "sender@example.com",
		},
		{
			name: "VacationWithParameters",
			script: `require ["vacation"];
				vacation :days 14 :subject "Out of Office" :from "me@example.com" 
				:addresses ["me@example.com", "me2@example.com"] 
				:handle "vacation-001" 
				"I'm on vacation until next week.";`,
			envFrom:           "sender@example.com",
			expectResponse:    true,
			expectedSubject:   "Out of Office",
			expectedBody:      "I'm on vacation until next week.",
			expectedFrom:      "me@example.com",
			expectedHandle:    "vacation-001",
			expectedDays:      14,
			expectedRecipient: "sender@example.com",
			expectedAddresses: []string{"me@example.com", "me2@example.com"},
		},
		{
			// RFC 5230 §5.3: without :subject, "The subject is set to the characters
			// "Auto: " followed by the original subject."
			name:              "DefaultSubjectFollowsTheOriginal",
			script:            `require ["vacation"]; vacation "Away.";`,
			envFrom:           "sender@example.com",
			subject:           "Project update",
			expectResponse:    true,
			expectedSubject:   "Auto: Project update",
			expectedBody:      "Away.",
			expectedDays:      7,
			expectedRecipient: "sender@example.com",
		},
		{
			// The original subject is decoded (RFC 2047) before it is reused.
			name:              "DefaultSubjectDecodesTheOriginal",
			script:            `require ["vacation"]; vacation "Away.";`,
			envFrom:           "sender@example.com",
			subject:           "=?utf-8?q?R=C3=A9union?=",
			expectResponse:    true,
			expectedSubject:   "Auto: Réunion",
			expectedBody:      "Away.",
			expectedDays:      7,
			expectedRecipient: "sender@example.com",
		},
		{
			// An explicit :subject is used as given.
			name:              "ExplicitSubjectIgnoresTheOriginal",
			script:            `require ["vacation"]; vacation :subject "Away" "Away.";`,
			envFrom:           "sender@example.com",
			subject:           "Project update",
			expectResponse:    true,
			expectedSubject:   "Away",
			expectedBody:      "Away.",
			expectedDays:      7,
			expectedRecipient: "sender@example.com",
		},
		{
			// RFC 5230 §5.3 generates a subject only "If the :subject parameter is
			// not supplied"; an explicit empty one is supplied, so it is kept.
			name:              "ExplicitEmptySubjectIsKept",
			script:            `require ["vacation"]; vacation :subject "" "Away.";`,
			envFrom:           "sender@example.com",
			subject:           "Original subject",
			expectResponse:    true,
			expectedSubject:   "",
			expectedBody:      "Away.",
			expectedDays:      7,
			expectedRecipient: "sender@example.com",
		},
		{
			// The fixed subject is for a reply without :subject to a message without
			// one, not for an explicit empty :subject.
			name:              "ExplicitEmptySubjectWithoutAnOriginalIsKept",
			script:            `require ["vacation"]; vacation :subject "" "Away.";`,
			envFrom:           "sender@example.com",
			expectResponse:    true,
			expectedSubject:   "",
			expectedBody:      "Away.",
			expectedDays:      7,
			expectedRecipient: "sender@example.com",
		},
		{
			// A :subject that expands to nothing was supplied all the same.
			name: "SubjectExpandingToEmptyIsKept",
			script: `require ["vacation", "variables"]; set "s" "";
				vacation :subject "${s}" "Away.";`,
			envFrom:           "sender@example.com",
			subject:           "Original subject",
			expectResponse:    true,
			expectedSubject:   "",
			expectedBody:      "Away.",
			expectedDays:      7,
			expectedRecipient: "sender@example.com",
		},
		{
			// RFC 5230 §4.5: the :addresses belong to the recipient for the personal
			// message check, which the implementation makes, so it needs them expanded.
			name: "AddressesAreExpandedForTheImplementation",
			script: `require ["vacation", "variables"]; set "alt" "user@example.net";
				vacation :addresses ["${alt}", "other@example.org"] "Away.";`,
			envFrom:           "sender@example.com",
			expectResponse:    true,
			expectedSubject:   "Automated reply",
			expectedBody:      "Away.",
			expectedDays:      7,
			expectedRecipient: "sender@example.com",
			expectedAddresses: []string{"user@example.net", "other@example.org"},
		},
		{
			name:           "NoVacationResponseToOwnAddresses",
			script:         `require ["vacation"]; vacation :addresses ["sender@example.com", "other@example.com"] "Away.";`,
			envFrom:        "sender@example.com",
			expectResponse: false,
		},
		{
			// A null reverse path gets no response (RFC 3834 §2), and the script still
			// runs, as in Pigeonhole's vacation/smtp.svtest "Null Sender".
			name:           "NoVacationResponseToNullSender",
			script:         `require ["vacation"]; vacation "Away.";`,
			envFrom:        "",
			expectResponse: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			opts := sieve.DefaultOptions()
			opts.EnabledExtensions = []string{"vacation", "variables"}

			parsedScript, err := sieve.Load(strings.NewReader(tc.script), opts)
			if err != nil {
				t.Fatalf("Failed to load script: %v", err)
			}

			env := interp.EnvelopeStatic{
				From: tc.envFrom,
				To:   "recipient@example.com",
			}

			header := textproto.MIMEHeader{}
			if tc.subject != "" {
				header.Set("Subject", tc.subject)
			}
			data := sieve.NewRuntimeData(parsedScript, interp.DummyPolicy{}, env, interp.MessageStatic{Header: header})

			err = parsedScript.Execute(ctx, data)
			if err != nil {
				t.Fatalf("Script execution failed: %v", err)
			}

			if !tc.expectResponse {
				if len(data.VacationResponses) != 0 {
					t.Fatalf("Expected no vacation responses, got %d", len(data.VacationResponses))
				}
				return
			}

			if len(data.VacationResponses) != 1 {
				t.Fatalf("Expected 1 vacation response, got %d", len(data.VacationResponses))
			}

			resp, ok := data.VacationResponses[tc.expectedRecipient]
			if !ok {
				t.Fatalf("Expected vacation response for %s", tc.expectedRecipient)
			}

			if resp.Subject != tc.expectedSubject {
				t.Errorf("Expected subject %q, got %q", tc.expectedSubject, resp.Subject)
			}
			if resp.Body != tc.expectedBody {
				t.Errorf("Expected body %q, got %q", tc.expectedBody, resp.Body)
			}
			if resp.From != tc.expectedFrom {
				t.Errorf("Expected from %q, got %q", tc.expectedFrom, resp.From)
			}
			if resp.Handle != tc.expectedHandle {
				t.Errorf("Expected handle %q, got %q", tc.expectedHandle, resp.Handle)
			}
			if resp.Days != tc.expectedDays {
				t.Errorf("Expected days %d, got %d", tc.expectedDays, resp.Days)
			}
			if !slices.Equal(resp.Addresses, tc.expectedAddresses) {
				t.Errorf("Expected addresses %q, got %q", tc.expectedAddresses, resp.Addresses)
			}
		})
	}
}

// TestVacationCommandSubjectWithoutSubjectSet covers a CmdVacation built in code
// rather than parsed: a nonempty Subject is the subject even without SubjectSet,
// as a nonempty Handle is the handle without HandleSet.
func TestVacationCommandSubjectWithoutSubjectSet(t *testing.T) {
	opts := sieve.DefaultOptions()
	opts.EnabledExtensions = []string{"vacation"}
	script, err := sieve.Load(strings.NewReader(`require "vacation";`), opts)
	if err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}
	header := make(textproto.MIMEHeader)
	header.Set("Subject", "Original subject")
	env := interp.EnvelopeStatic{From: "sender@example.com", To: "recipient@example.com"}
	data := sieve.NewRuntimeData(script, interp.DummyPolicy{}, env, interp.MessageStatic{Header: header})
	cmd := interp.CmdVacation{Days: 7, Subject: "Out of office", Reason: "Away."}
	if err := cmd.Execute(context.Background(), data); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := data.VacationResponses["sender@example.com"].Subject; got != "Out of office" {
		t.Errorf("Expected subject %q, got %q", "Out of office", got)
	}
}

// TestVacationNullSenderRunsTheRestOfTheScript covers a message with a null envelope
// sender (a bounce, DSN or MDN): vacation does nothing and the actions next to it still
// happen. It used to fail the whole script, so such a message skipped every filter in a
// script that also held a vacation.
func TestVacationNullSenderRunsTheRestOfTheScript(t *testing.T) {
	opts := sieve.DefaultOptions()
	opts.EnabledExtensions = []string{"vacation", "fileinto"}
	script, err := sieve.Load(strings.NewReader(`require ["vacation", "fileinto"]; vacation "Away."; fileinto "Invoices";`), opts)
	if err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}

	env := interp.EnvelopeStatic{From: "", To: "recipient@example.com"}
	data := sieve.NewRuntimeData(script, interp.DummyPolicy{}, env, interp.MessageStatic{})
	if err := script.Execute(context.Background(), data); err != nil {
		t.Fatalf("Script execution failed: %v", err)
	}

	if len(data.VacationResponses) != 0 {
		t.Errorf("Expected no vacation responses, got %d", len(data.VacationResponses))
	}
	if len(data.Mailboxes) != 1 || data.Mailboxes[0] != "Invoices" {
		t.Errorf("Expected the message filed into Invoices, got mailboxes %v", data.Mailboxes)
	}
}
