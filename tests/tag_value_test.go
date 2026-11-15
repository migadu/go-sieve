package tests

import (
	"strings"
	"testing"

	"github.com/migadu/go-sieve"
)

func loadWithTagValueExtensions(script string) error {
	opts := sieve.DefaultOptions()
	opts.EnabledExtensions = []string{"fileinto", "imap4flags", "vacation", "foreverypart"}
	_, err := sieve.Load(strings.NewReader(script), opts)
	return err
}

// A tagged argument that takes a value cannot end the argument list: RFC
// 5232 §5 "Usage:   ":flags" <list-of-flags: string-list>", RFC 5703 §3
// "Usage:  foreverypart [":name" string] block" and "Usage:  break [":name"
// string];". The commands below have no required positional argument, so
// the dropped tag used to go unnoticed.
func TestTaggedArgumentNeedsItsValue(t *testing.T) {
	refused := []struct {
		name    string
		script  string
		wantErr string
	}{
		{"keep :flags", `require "imap4flags"; keep :flags;`, ":flags requires a value"},
		{"keep :flags without require", `keep :flags;`, ":flags requires a value"},
		{"foreverypart :name", `require "foreverypart"; foreverypart :name { }`, ":name requires a value"},
		{"break :name", `require "foreverypart"; foreverypart { break :name; }`, ":name requires a value"},
		{"vacation :days", `require "vacation"; vacation :days;`, ":days requires a value"},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			err := loadWithTagValueExtensions(tc.script)
			if err == nil {
				t.Fatal("load succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}

	accepted := []string{
		`require "imap4flags"; keep :flags "\\Seen";`,
		`require "imap4flags"; keep :flags ["\\Seen", "\\Flagged"];`,
		`keep;`,
		`require ["fileinto", "imap4flags"]; fileinto :flags "\\Seen" "INBOX";`,
		`require "foreverypart"; foreverypart :name "outer" { break :name "outer"; }`,
		`require "foreverypart"; foreverypart { break; }`,
		`require "vacation"; vacation :days 3 "Away";`,
	}
	for _, script := range accepted {
		if err := loadWithTagValueExtensions(script); err != nil {
			t.Errorf("%s: %v", script, err)
		}
	}
}
