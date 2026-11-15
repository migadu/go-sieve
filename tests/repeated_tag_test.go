package tests

import (
	"strings"
	"testing"

	"github.com/migadu/go-sieve"
)

func loadWithRepeatedTagExtensions(script string) error {
	opts := sieve.DefaultOptions()
	opts.EnabledExtensions = []string{"fileinto", "copy", "vacation", "variables", "imap4flags", "relational"}
	_, err := sieve.Load(strings.NewReader(script), opts)
	return err
}

// RFC 5228 §2.6: "It is an error for a script, on a single command, to use
// conflicting arguments or to use a tagged or optional argument more than
// once." §8.1: "Identifiers, tags, and numbers are case-insensitive". A
// repeated tag used to load, the last value winning; Pigeonhole refuses it
// ("encountered duplicate :days argument for the vacation command").
func TestRepeatedTaggedArgument(t *testing.T) {
	refused := []struct {
		name    string
		script  string
		wantErr string
	}{
		{"header :comparator", `if header :comparator "i;octet" :comparator "i;ascii-casemap" "Subject" "x" { stop; }`, ":comparator given more than once"},
		{"vacation :days", `require "vacation"; vacation :days 1 :days 2 "away";`, ":days given more than once"},
		{"vacation :subject", `require "vacation"; vacation :subject "a" :subject "b" "away";`, ":subject given more than once"},
		{"fileinto :copy", `require ["fileinto", "copy"]; fileinto :copy :copy "Archive";`, ":copy given more than once"},
		{"tags differing in case", `require "vacation"; vacation :days 1 :DAYS 2 "away";`, ":DAYS given more than once"},
		{"repeat apart", `require "vacation"; vacation :days 1 :subject "a" :days 2 "away";`, ":days given more than once"},
		{"keep :flags", `require "imap4flags"; keep :flags "\\Seen" :flags "\\Flagged";`, ":flags given more than once"},
		{"set :lower", `require "variables"; set :lower :lower "a" "B";`, ":lower given more than once"},
		{"header :is", `if header :is :is "Subject" "x" { stop; }`, ":is given more than once"},
		{"header :value", `require "relational"; if header :value "eq" :value "lt" "Subject" "x" { stop; }`, ":value given more than once"},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			err := loadWithRepeatedTagExtensions(tc.script)
			if err == nil {
				t.Fatal("load succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}

	accepted := []string{
		`if header :comparator "i;octet" :is "Subject" "x" { stop; }`,
		`require "vacation"; vacation :days 1 :subject "a" "away";`,
		`require "vacation"; vacation :subject "a" :days 1 :from "a@example.com" "away";`,
		`require ["fileinto", "copy"]; fileinto :copy "Archive";`,
		`require "imap4flags"; keep :flags ["\\Seen", "\\Flagged"];`,
		`require "variables"; set :lower :upperfirst "a" "B";`,
		// A repeat on another command is not a repeat.
		`require ["fileinto", "copy"]; fileinto :copy "A"; fileinto :copy "B";`,
		`if allof (header :is "Subject" "x", header :is "From" "y") { stop; }`,
	}
	for _, script := range accepted {
		if err := loadWithRepeatedTagExtensions(script); err != nil {
			t.Errorf("%s: %v", script, err)
		}
	}
}
