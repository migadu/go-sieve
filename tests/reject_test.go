package tests

import (
	"context"
	"net/textproto"
	"strings"
	"testing"

	"github.com/migadu/go-sieve"
	"github.com/migadu/go-sieve/interp"
)

var rejectExtensions = []string{
	"fileinto", "envelope", "imap4flags", "variables", "vacation", "copy",
	"reject", "ereject",
}

func runReject(t *testing.T, script string) (*interp.RuntimeData, error) {
	t.Helper()
	opts := sieve.DefaultOptions()
	opts.EnabledExtensions = rejectExtensions
	loaded, err := sieve.Load(strings.NewReader(script), opts)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	hdr := textproto.MIMEHeader{}
	hdr.Set("To", "list@frop.example")
	hdr.Set("Subject", "Buy now")
	data := sieve.NewRuntimeData(loaded, interp.DummyPolicy{},
		interp.EnvelopeStatic{From: "sender@example.org", To: "rcpt@example.net"},
		interp.MessageStatic{Header: hdr, Size: 100})
	return data, loaded.Execute(context.Background(), data)
}

func TestRejectLoad(t *testing.T) {
	cases := []struct {
		name    string
		script  string
		enabled []string
		wantErr string
	}{
		{"reject without require", `reject "no";`, rejectExtensions, "missing require 'reject'"},
		{"ereject without require", `require "reject"; ereject "no";`, rejectExtensions, "missing require 'ereject'"},
		{"reject not enabled", `require "reject"; reject "no";`, []string{"fileinto"}, "extension 'reject' is not supported"},
		{"reject without reason", `require "reject"; reject;`, rejectExtensions, ""},
		{"reject with two reasons", `require "reject"; reject "a" "b";`, rejectExtensions, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := sieve.DefaultOptions()
			opts.EnabledExtensions = tc.enabled
			_, err := sieve.Load(strings.NewReader(tc.script), opts)
			if err == nil {
				t.Fatal("load succeeded, want an error")
			}
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestRejectExecute(t *testing.T) {
	cases := []struct {
		name         string
		script       string
		wantRejected bool
		wantReason   string
		wantExtended bool
		wantKeep     bool // implicit keep after the run
		wantFlags    []string
	}{
		{
			name:         "reject",
			script:       `require "reject"; reject "I don't want your mail";`,
			wantRejected: true, wantReason: "I don't want your mail",
		},
		{
			name:         "ereject",
			script:       `require "ereject"; ereject "go away";`,
			wantRejected: true, wantReason: "go away", wantExtended: true,
		},
		{
			name:         "reason expands variables",
			script:       `require ["reject", "variables"]; set "who" "spammers"; reject "no ${who}";`,
			wantRejected: true, wantReason: "no spammers",
		},
		{
			// Pigeonhole's extensions/reject/execute/basic.sieve: stop keeps the
			// trailing keep from running.
			name: "pigeonhole basic.sieve",
			script: `require "reject";
if address :contains "to" "frop.example" {
	reject "Don't send unrequested messages.";
	stop;
}
keep;`,
			wantRejected: true, wantReason: "Don't send unrequested messages.",
		},
		{
			name:     "reject not reached keeps implicitly",
			script:   `require "reject"; if false { reject "no"; }`,
			wantKeep: true,
		},
		{
			name:         "flags do not conflict",
			script:       `require ["reject", "imap4flags"]; addflag "\\Seen"; reject "no";`,
			wantRejected: true, wantReason: "no", wantFlags: []string{"\\seen"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := runReject(t, tc.script)
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if d.Rejected != tc.wantRejected || d.RejectReason != tc.wantReason || d.RejectExtended != tc.wantExtended {
				t.Fatalf("rejected=%v reason=%q extended=%v, want %v %q %v",
					d.Rejected, d.RejectReason, d.RejectExtended, tc.wantRejected, tc.wantReason, tc.wantExtended)
			}
			if d.ImplicitKeep != tc.wantKeep {
				t.Fatalf("implicit keep = %v, want %v", d.ImplicitKeep, tc.wantKeep)
			}
			if strings.Join(d.Flags, " ") != strings.Join(tc.wantFlags, " ") {
				t.Fatalf("flags = %v, want %v", d.Flags, tc.wantFlags)
			}
		})
	}
}

// RFC 5429 §2.4, enforced in either order and through stop.
func TestRejectConflicts(t *testing.T) {
	cases := []struct {
		name    string
		script  string
		wantErr string
	}{
		{"duplicate reject", `require "reject"; reject "a"; reject "b";`, "duplicate"},
		{"reject then ereject", `require ["reject", "ereject"]; reject "a"; ereject "b";`, "duplicate"},
		{"keep then reject", `require "reject"; keep; reject "no";`, "keep"},
		{"reject then keep", `require "reject"; reject "no"; keep;`, "keep"},
		{"reject then fileinto", `require ["reject", "fileinto"]; reject "no"; fileinto "Junk";`, "fileinto"},
		{"fileinto :copy then reject", `require ["reject", "fileinto", "copy"]; fileinto :copy "Junk"; reject "no";`, "fileinto"},
		{"reject then redirect", `require "reject"; reject "no"; redirect "a@example.com";`, "redirect"},
		{"redirect :copy then reject", `require ["reject", "copy"]; redirect :copy "a@example.com"; reject "no";`, "redirect"},
		{"vacation then reject", `require ["reject", "vacation"]; vacation "away"; reject "no";`, "vacation"},
		{"reject then vacation", `require ["reject", "vacation"]; reject "no"; vacation "away";`, "vacation"},
		{"reject, keep, stop", `require "reject"; reject "no"; keep; stop;`, "keep"},
		{"ereject then fileinto", `require ["ereject", "fileinto"]; ereject "no"; fileinto "Junk";`, "fileinto"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runReject(t, tc.script)
			if err == nil {
				t.Fatal("execute succeeded, want a conflict error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}
