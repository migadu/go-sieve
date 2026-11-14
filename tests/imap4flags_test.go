package tests

import (
	"context"
	"net/textproto"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/migadu/go-sieve"
	"github.com/migadu/go-sieve/interp"
)

func TestExtensionsImap4flagsBasic(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "extensions", "imap4flags", "basic.svtest"))
}

func TestExtensionsImap4flagsHasflag(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "extensions", "imap4flags", "hasflag.svtest"))
}

var imap4flagsExtensions = []string{
	"fileinto", "imap4flags", "variables", "relational", "envelope",
	"comparator-i;octet", "comparator-i;ascii-numeric", "regex",
}

func runImap4flags(t *testing.T, script string) *interp.RuntimeData {
	t.Helper()
	opts := sieve.DefaultOptions()
	opts.EnabledExtensions = imap4flagsExtensions
	loaded, err := sieve.Load(strings.NewReader(script), opts)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	hdr := textproto.MIMEHeader{}
	hdr.Set("Subject", "Hello")
	data := sieve.NewRuntimeData(loaded, interp.DummyPolicy{},
		interp.EnvelopeStatic{From: "sender@example.org", To: "user@example.com"},
		interp.MessageStatic{Header: hdr, Size: 100})
	if err := loaded.Execute(context.Background(), data); err != nil {
		t.Fatalf("execute: %v", err)
	}
	return data
}

// RFC 5232 §3: setflag, addflag and removeflag take an optional variable name
// before the list of flags ("setflag [<variablename: string>] <list-of-flags:
// string-list>"), and then change that variable instead of the internal one.
func TestImap4flagsVariableName(t *testing.T) {
	cases := []struct {
		name      string
		script    string
		variables map[string]string // the flag variables after the run
		internal  []string          // the internal variable after the run, if not nil
		mailboxes []string
	}{
		{
			// RFC 5232 §3.1's example. The internal variable stays empty.
			name: "setflag then fileinto :flags",
			script: `require ["imap4flags", "variables", "fileinto"];
				setflag "flagvar" "\\Flagged";
				if hasflag "\\Flagged" { fileinto "Wrong"; }
				fileinto :flags "${flagvar}" "Archive";`,
			variables: map[string]string{"flagvar": `\flagged`},
			mailboxes: []string{"Archive"},
		},
		{
			// RFC 5232 §3.2: the two addflags give the same result as one
			// with both flags.
			name: "addflag twice",
			script: `require ["imap4flags", "variables"];
				addflag "flagvar" "\\Deleted";
				addflag "flagvar" "\\Answered";
				addflag "other" ["\\Deleted", "\\Answered"];
				addflag "third" "\\Answered \\Deleted";`,
			variables: map[string]string{
				"flagvar": `\answered \deleted`,
				"other":   `\answered \deleted`,
				"third":   `\answered \deleted`,
			},
		},
		{
			// RFC 5232 §3.3's example.
			name: "removeflag",
			script: `require ["imap4flags", "variables", "fileinto"];
				addflag "flagvar" "$MDNRequired";
				removeflag "flagvar" "$mdnrequired";
				fileinto :flags "${flagvar}" "INBOX.imap-list";`,
			variables: map[string]string{"flagvar": ""},
			mailboxes: []string{"INBOX.imap-list"},
		},
		{
			name: "addflag to a variable set by set",
			script: `require ["imap4flags", "variables"];
				set "Flags" "  Work  \\seen work ";
				addflag "flags" "Later";
				removeflag "FLAGS" "\\Seen";`,
			variables: map[string]string{"flags": "later work"},
		},
		{
			name: "internal variable left alone",
			script: `require ["imap4flags", "variables"];
				setflag "\\Seen";
				setflag "v" "A";
				addflag "v" "B";
				removeflag "v" "A";`,
			variables: map[string]string{"v": "b"},
			internal:  []string{`\seen`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := runImap4flags(t, tc.script)
			for name, want := range tc.variables {
				if got := data.Variables[name]; got != want {
					t.Errorf("variable %q = %q, want %q", name, got, want)
				}
			}
			if tc.internal != nil && !reflect.DeepEqual(data.Flags, tc.internal) {
				t.Errorf("internal variable = %q, want %q", data.Flags, tc.internal)
			}
			if !reflect.DeepEqual(data.Mailboxes, tc.mailboxes) {
				t.Errorf("mailboxes = %q, want %q", data.Mailboxes, tc.mailboxes)
			}
		})
	}
}

// RFC 5232 §4: "hasflag [MATCH-TYPE] [COMPARATOR] [<variable-list:
// string-list>] <list-of-flags: string-list>".
func TestImap4flagsHasflag(t *testing.T) {
	cases := []struct {
		name string
		test string // the hasflag test, after the script's setup
		want bool
	}{
		{"internal :is", `hasflag "\\SEEN"`, true},
		{"internal absent", `hasflag "\\Flagged"`, false},
		{"internal list of flags in one string", `hasflag "$Nope \\Seen"`, true},
		{"internal empty string ignored", `hasflag ""`, false},
		{"internal empty :contains ignored", `hasflag :contains ""`, false},
		{"internal :count", `hasflag :count "eq" :comparator "i;ascii-numeric" "2"`, true},
		{"variable :is", `hasflag "MyVar" "junk"`, true},
		{"variable :contains", `hasflag :contains "MyVar" "forward"`, true},
		{"variable :contains miss", `hasflag :contains "MyVar" ["label1", "label2"]`, false},
		{"variable :matches", `hasflag :matches "MyVar" "$Not*"`, true},
		{"variable :regex", `hasflag :regex "MyVar" "^gnus-"`, true},
		{"variable :count distinct flags", `hasflag :count "eq" :comparator "i;ascii-numeric" "MyVar" "8"`, true},
		{"variable list :count sums", `hasflag :count "eq" :comparator "i;ascii-numeric" ["MyVar", "Two"] "10"`, true},
		{"variable list any matches", `hasflag ["Unset", "Two"] "B"`, true},
		{"unset variable", `hasflag "Unset" "\\Seen"`, false},
		{"variable from a key", `hasflag "MyVar" "${key}"`, true},
		{"variable list of flags in a key", `hasflag "MyVar" "${keys}"`, true},
		{"i;octet keeps the variable's case", `hasflag :comparator "i;octet" "MyVar" "NonJunk"`, true},
		{"i;octet case differs", `hasflag :comparator "i;octet" "MyVar" "nonjunk"`, false},
	}
	setup := `require ["imap4flags", "variables", "relational", "fileinto", "regex",
			"comparator-i;ascii-numeric", "comparator-i;octet"];
		addflag "\\Seen $Label1";
		set "MyVar" "NonJunk Junk gnus-forward $Forwarded NotJunk JunkRecorded $Junk $NotJunk junk";
		set "Two" "A b";
		set "key" "junk";
		set "keys" "$Nope Junk";
		`
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := runImap4flags(t, setup+`if `+tc.test+` { fileinto "Matched"; }`)
			got := len(data.Mailboxes) == 1
			if got != tc.want {
				t.Errorf("%s = %v, want %v", tc.test, got, tc.want)
			}
		})
	}

	t.Run("match variables", func(t *testing.T) {
		data := runImap4flags(t, `require ["imap4flags", "variables", "fileinto"];
			setflag "$Label7";
			if hasflag :matches "$label*" { fileinto "L${1}"; }`)
		if want := []string{"L7"}; !reflect.DeepEqual(data.Mailboxes, want) {
			t.Errorf("mailboxes = %q, want %q", data.Mailboxes, want)
		}
	})

	// The second script of migadu/sora#883.
	t.Run("addflag then hasflag", func(t *testing.T) {
		data := runImap4flags(t, `require ["imap4flags", "fileinto"];
			addflag "\\Seen";
			if hasflag "\\Seen" { fileinto "Read"; }`)
		if want := []string{"Read"}; !reflect.DeepEqual(data.Mailboxes, want) {
			t.Errorf("mailboxes = %q, want %q", data.Mailboxes, want)
		}
	})
}

func TestImap4flagsLoadErrors(t *testing.T) {
	cases := []struct {
		name    string
		script  string
		wantErr string
	}{
		{"variable name needs variables", `require "imap4flags"; setflag "v" "\\Seen";`, `require 'variables'`},
		{"hasflag variable list needs variables", `require "imap4flags"; if hasflag "v" "\\Seen" { stop; }`, `require 'variables'`},
		{"hasflag needs imap4flags", `if hasflag "\\Seen" { stop; }`, `require 'imap4flags'`},
		{"hasflag :count needs relational", `require "imap4flags"; if hasflag :count "eq" "1" { stop; }`, `require 'relational'`},
		{"variable name list", `require ["imap4flags", "variables"]; addflag ["a", "b"] "\\Seen";`, `variable name`},
		{"invalid variable name", `require ["imap4flags", "variables"]; setflag "1x" "\\Seen";`, `variable`},
		{"variable reference as a name", `require ["imap4flags", "variables"]; set "x" "y"; setflag "${x}" "\\Seen";`, `variable`},
		{"envelope variable is not settable", `require ["imap4flags", "variables", "envelope"]; setflag "envelope.from" "\\Seen";`, `variable`},
		{"invalid hasflag variable", `require ["imap4flags", "variables"]; if hasflag ["a", "b-c"] "\\Seen" { stop; }`, `variable`},
		{"too many arguments", `require ["imap4flags", "variables"]; removeflag "a" "b" "c";`, `too many arguments`},
		{"hasflag without flags", `require "imap4flags"; if hasflag :contains { stop; }`, `argument is required`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := sieve.DefaultOptions()
			opts.EnabledExtensions = imap4flagsExtensions
			_, err := sieve.Load(strings.NewReader(tc.script), opts)
			if err == nil {
				t.Fatal("load succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}

	// An envelope variable can be read, so it may be tested.
	opts := sieve.DefaultOptions()
	opts.EnabledExtensions = imap4flagsExtensions
	script := `require ["imap4flags", "variables", "envelope"]; if hasflag "envelope.from" "x" { stop; }`
	if _, err := sieve.Load(strings.NewReader(script), opts); err != nil {
		t.Errorf("%s: %v", script, err)
	}
}
