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

func runFlags(t *testing.T, script string) *interp.RuntimeData {
	t.Helper()
	opts := sieve.DefaultOptions()
	opts.EnabledExtensions = []string{"fileinto", "imap4flags", "variables", "copy"}
	loaded, err := sieve.Load(strings.NewReader(script), opts)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	data := sieve.NewRuntimeData(loaded, interp.DummyPolicy{},
		interp.EnvelopeStatic{From: "sender@example.org", To: "rcpt@example.net"},
		interp.MessageStatic{Header: textproto.MIMEHeader{}, Size: 100})
	if err := loaded.Execute(context.Background(), data); err != nil {
		t.Fatalf("execute: %v", err)
	}
	return data
}

// TestFlagsBelongToTheirCopy checks that keep and fileinto each record the flags
// of the copy they store, and that :flags leaves the internal variable alone.
// RFC 5232 §5: "The copy of the message filed into the mailbox will have only
// flags listed after the :flags tagged argument." §3: "If the :flags tagged
// argument is not specified with those two actions, the current value of the
// internal variable is used instead. The value of the internal variable also
// applies to the implicit keep."
func TestFlagsBelongToTheirCopy(t *testing.T) {
	cases := []struct {
		name         string
		script       string
		wantMailbox  map[string][]string
		wantKeep     []string
		wantInternal []string
	}{
		{
			name:        "fileinto :flags then keep",
			script:      `require ["fileinto", "imap4flags"]; fileinto :flags "\\Flagged" "Archive"; keep;`,
			wantMailbox: map[string][]string{"Archive": {"\\flagged"}},
		},
		{
			name:        "keep :flags then fileinto",
			script:      `require ["fileinto", "imap4flags"]; keep :flags "\\Flagged"; fileinto "Archive";`,
			wantMailbox: map[string][]string{"Archive": nil},
			wantKeep:    []string{"\\flagged"},
		},
		{
			name: "each fileinto its own flags",
			script: `require ["fileinto", "imap4flags"]; setflag "Work";
				fileinto :flags "\\Flagged" "Archive"; fileinto "Projects";`,
			wantMailbox:  map[string][]string{"Archive": {"\\flagged"}, "Projects": {"work"}},
			wantInternal: []string{"work"},
		},
		{
			// "the current value of the internal variable": the value when the
			// action runs, not when the script ends.
			name: "internal variable taken when the action runs",
			script: `require ["fileinto", "imap4flags", "copy"]; addflag "Work";
				fileinto :copy "Archive"; removeflag "Work"; addflag "\\Seen"; keep;
				addflag "Later";`,
			wantMailbox:  map[string][]string{"Archive": {"work"}},
			wantKeep:     []string{"\\seen"},
			wantInternal: []string{"\\seen", "later"},
		},
		{
			// discard cancels the implicit keep; it does not touch the internal
			// variable, which a later keep still uses.
			name:         "discard leaves the internal variable",
			script:       `require "imap4flags"; setflag "Work"; discard; keep;`,
			wantKeep:     []string{"work"},
			wantInternal: []string{"work"},
		},
		{
			name: "variables in :flags",
			script: `require ["fileinto", "imap4flags", "variables"]; set "f" "$Label1 Work";
				fileinto :flags "${f}" "Archive";`,
			wantMailbox: map[string][]string{"Archive": {"$label1", "work"}},
		},
		{
			// Pigeonhole's testsuite, imap4flags/execute/flags-side-effect.sieve:
			// "When keep/fileinto is used multiple times in a script and duplicate
			// message elimination is performed, the last flag list value MUST win."
			name: "last flag list wins",
			script: `require ["imap4flags", "fileinto"];
				setflag "IMPLICIT";
				fileinto :flags "\\Seen \\Draft" "INBOX.Junk";
				fileinto :flags "NONSENSE" "INBOX.Junk";
				keep;
				keep :flags "\\Seen";
				fileinto :flags "\\Seen" "Inbox.Nonsense";
				fileinto "Inbox.Nonsense";`,
			wantMailbox:  map[string][]string{"INBOX.Junk": {"nonsense"}, "Inbox.Nonsense": {"implicit"}},
			wantKeep:     []string{"\\seen"},
			wantInternal: []string{"implicit"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := runFlags(t, tc.script)
			if len(d.MailboxFlags) != len(tc.wantMailbox) {
				t.Errorf("MailboxFlags = %v, want %v", d.MailboxFlags, tc.wantMailbox)
			}
			for mailbox, want := range tc.wantMailbox {
				got, ok := d.MailboxFlags[mailbox]
				if !ok || strings.Join(got, " ") != strings.Join(want, " ") {
					t.Errorf("flags of the copy in %q = %v, want %v", mailbox, got, want)
				}
			}
			if strings.Join(d.KeepFlags, " ") != strings.Join(tc.wantKeep, " ") {
				t.Errorf("KeepFlags = %v, want %v", d.KeepFlags, tc.wantKeep)
			}
			if strings.Join(d.Flags, " ") != strings.Join(tc.wantInternal, " ") {
				t.Errorf("internal variable = %v, want %v", d.Flags, tc.wantInternal)
			}
		})
	}
}

// TestFlagsCopy checks that RuntimeData.Copy does not share the per-copy flags
// with the original.
func TestFlagsCopy(t *testing.T) {
	d := runFlags(t, `require ["fileinto", "imap4flags"]; fileinto :flags "Work" "Archive"; keep :flags "\\Seen";`)
	c := d.Copy()
	if !reflect.DeepEqual(c.MailboxFlags, d.MailboxFlags) || !reflect.DeepEqual(c.KeepFlags, d.KeepFlags) {
		t.Fatalf("copy has MailboxFlags %v KeepFlags %v, want %v %v", c.MailboxFlags, c.KeepFlags, d.MailboxFlags, d.KeepFlags)
	}
	c.MailboxFlags["Archive"] = []string{"changed"}
	c.KeepFlags[0] = "changed"
	if d.MailboxFlags["Archive"][0] != "work" || d.KeepFlags[0] != "\\seen" {
		t.Fatalf("changing the copy changed the original: MailboxFlags %v KeepFlags %v", d.MailboxFlags, d.KeepFlags)
	}
}
