package tests

import (
	"context"
	"net/textproto"
	"strings"
	"testing"

	"github.com/migadu/go-sieve"
	"github.com/migadu/go-sieve/interp"
)

// TestVacationSubjectSeesHeaderEdits covers that the default vacation subject,
// "Auto: " followed by the original subject (RFC 5230 §5.3), quotes the Subject
// as addheader and deleteheader left it. RFC 5293 §7: "Tests and actions such
// as "exists", "header", or "vacation" [VACATION] that examine header fields
// MUST examine the current state of a header as modified by any actions that
// have taken place so far."
//
// The vacation action read the message's original Subject, so a Subject that
// deleteheader removed or addheader replaced was still quoted.
func TestVacationSubjectSeesHeaderEdits(t *testing.T) {
	for _, tc := range []struct {
		name   string
		script string
		want   string
	}{
		{"original", `vacation "Away.";`, "Auto: hello"},
		{"replaced", `deleteheader "Subject"; addheader "Subject" "[list] hello"; vacation "Away.";`, "Auto: [list] hello"},
		{"deleted", `deleteheader "Subject"; vacation "Away.";`, "Automated reply"},
		// "so far": an edit after the action does not change what it saw.
		{"deleted after the action", `vacation "Away."; deleteheader "Subject";`, "Auto: hello"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := sieve.DefaultOptions()
			opts.EnabledExtensions = []string{"vacation", "editheader"}
			script, err := sieve.Load(strings.NewReader(`require ["vacation", "editheader"]; `+tc.script), opts)
			if err != nil {
				t.Fatalf("Failed to load script: %v", err)
			}
			header := make(textproto.MIMEHeader)
			header.Set("Subject", "hello")
			env := interp.EnvelopeStatic{From: "sender@example.com", To: "recipient@example.com"}
			data := sieve.NewRuntimeData(script, interp.DummyPolicy{}, env, interp.MessageStatic{Header: header})
			if err := script.Execute(context.Background(), data); err != nil {
				t.Fatalf("Execute: %v", err)
			}
			resp, ok := data.VacationResponses["sender@example.com"]
			if !ok {
				t.Fatalf("%s: no vacation response", tc.script)
			}
			if resp.Subject != tc.want {
				t.Errorf("%s: subject = %q, want %q", tc.script, resp.Subject, tc.want)
			}
		})
	}
}
