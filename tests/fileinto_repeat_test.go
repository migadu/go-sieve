package tests

import (
	"context"
	"net/textproto"
	"reflect"
	"strings"
	"testing"

	"github.com/migadu/go-sieve"
	"github.com/migadu/go-sieve/interp"
)

// TestFileintoRepeatedTarget covers a script that files into the same mailbox
// twice. RFC 5228 §2.10.3: "Implementations SHOULD NOT deliver a message to the
// same mailbox more than once, even if a script explicitly asks for a message to
// be written to a mailbox twice." The repeat is still performed, though, so it
// cancels the implicit keep unless it has :copy (RFC 5228 §2.10.2, RFC 3894 §3),
// and its :create asks for the mailbox to be created (RFC 5490 §3.2), whichever
// of the two comes first.
func TestFileintoRepeatedTarget(t *testing.T) {
	cases := []struct {
		name         string
		script       string
		mailboxes    []string
		create       []string
		implicitKeep bool
	}{
		{
			name:      ":copy then without",
			script:    `require ["fileinto", "copy"]; fileinto :copy "Archive"; fileinto "Archive";`,
			mailboxes: []string{"Archive"},
		},
		{
			name:      "without then :copy",
			script:    `require ["fileinto", "copy"]; fileinto "Archive"; fileinto :copy "Archive";`,
			mailboxes: []string{"Archive"},
		},
		{
			name:         ":copy twice",
			script:       `require ["fileinto", "copy"]; fileinto :copy "Archive"; fileinto :copy "Archive";`,
			mailboxes:    []string{"Archive"},
			implicitKeep: true,
		},
		{
			name:      ":copy then without, named through a variable",
			script:    `require ["fileinto", "copy", "variables"]; set "box" "Archive"; fileinto :copy "Archive"; fileinto "${box}";`,
			mailboxes: []string{"Archive"},
		},
		{
			name:      "without then :create",
			script:    `require ["fileinto", "mailbox"]; fileinto "Projects"; fileinto :create "Projects";`,
			mailboxes: []string{"Projects"},
			create:    []string{"Projects"},
		},
		{
			name:      ":create then without",
			script:    `require ["fileinto", "mailbox"]; fileinto :create "Projects"; fileinto "Projects";`,
			mailboxes: []string{"Projects"},
			create:    []string{"Projects"},
		},
		{
			name:      ":create twice",
			script:    `require ["fileinto", "mailbox"]; fileinto :create "Projects"; fileinto :create "Projects";`,
			mailboxes: []string{"Projects"},
			create:    []string{"Projects"},
		},
		{
			name:      ":copy then :create",
			script:    `require ["fileinto", "mailbox", "copy"]; fileinto :copy "Projects"; fileinto :create "Projects";`,
			mailboxes: []string{"Projects"},
			create:    []string{"Projects"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := sieve.DefaultOptions()
			opts.EnabledExtensions = []string{"fileinto", "copy", "mailbox", "variables"}
			loaded, err := sieve.Load(strings.NewReader(tc.script), opts)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			data := sieve.NewRuntimeData(loaded, interp.DummyPolicy{},
				interp.EnvelopeStatic{From: "sender@example.org", To: "rcpt@example.com"},
				interp.MessageStatic{Header: textproto.MIMEHeader{}, Size: 100})
			if err := loaded.Execute(context.Background(), data); err != nil {
				t.Fatalf("execute: %v", err)
			}
			if !reflect.DeepEqual(data.Mailboxes, tc.mailboxes) {
				t.Errorf("Mailboxes = %q, want %q", data.Mailboxes, tc.mailboxes)
			}
			if len(data.MailboxesCreate) != 0 || len(tc.create) != 0 {
				if !reflect.DeepEqual(data.MailboxesCreate, tc.create) {
					t.Errorf("MailboxesCreate = %q, want %q", data.MailboxesCreate, tc.create)
				}
			}
			if data.ImplicitKeep != tc.implicitKeep {
				t.Errorf("ImplicitKeep = %v, want %v", data.ImplicitKeep, tc.implicitKeep)
			}
		})
	}
}
