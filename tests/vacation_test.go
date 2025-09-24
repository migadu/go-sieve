package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/foxcpp/go-sieve"
	"github.com/foxcpp/go-sieve/interp"
)

func TestVacation(t *testing.T) {
	testCases := []struct {
		name              string
		script            string
		envFrom           string
		expectResponse    bool
		expectedSubject   string
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
		},
		{
			name:           "NoVacationResponseToOwnAddresses",
			script:         `require ["vacation"]; vacation :addresses ["sender@example.com", "other@example.com"] "Away.";`,
			envFrom:        "sender@example.com",
			expectResponse: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			opts := sieve.DefaultOptions()
			opts.EnabledExtensions = []string{"vacation"}

			parsedScript, err := sieve.Load(strings.NewReader(tc.script), opts)
			if err != nil {
				t.Fatalf("Failed to load script: %v", err)
			}

			env := interp.EnvelopeStatic{
				From: tc.envFrom,
				To:   "recipient@example.com",
			}

			data := sieve.NewRuntimeData(parsedScript, interp.DummyPolicy{}, env, interp.MessageStatic{})

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
		})
	}
}
