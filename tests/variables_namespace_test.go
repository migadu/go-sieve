package tests

import (
	"strings"
	"testing"

	"github.com/migadu/go-sieve"
)

// RFC 5229 §3: a variable reference into a namespace whose extension is not
// required is an error. It is refused when the script loads; expandVars used
// to panic on it at execution time, in every positional or tagged string.
func TestNamespaceVariablesCheckedAtLoad(t *testing.T) {
	all := []string{"fileinto", "variables", "envelope", "vacation", "imap4flags", "reject", "copy", "editheader"}
	cases := []struct {
		name    string
		script  string
		enabled []string
		wantErr string
	}{
		{"fileinto envelope without require", `require ["fileinto","variables"]; fileinto "${envelope.from}";`, all, `${envelope.from} needs require "envelope"`},
		{"reject envelope without require", `require ["reject","variables"]; reject "${envelope.from}";`, all, `${envelope.from} needs require "envelope"`},
		{"redirect envelope without require", `require "variables"; redirect "${envelope.to}";`, all, `${envelope.to} needs require "envelope"`},
		{"vacation subject tag", `require ["vacation","variables"]; vacation :subject "${envelope.from}" "away";`, all, `${envelope.from} needs require "envelope"`},
		{"fileinto flags list tag", `require ["fileinto","imap4flags","variables"]; fileinto :flags ["\\Seen", "${envelope.from}"] "x";`, all, `${envelope.from} needs require "envelope"`},
		{"addheader string list", `require ["editheader","variables"]; addheader "X-A" "${envelope.from}";`, all, `${envelope.from} needs require "envelope"`},
		{"set value", `require "variables"; set "a" "${envelope.from}";`, all, `${envelope.from} needs require "envelope"`},
		{"unknown namespace", `require ["fileinto","variables"]; fileinto "${foo.bar}";`, all, `unknown namespace "foo"`},
		{"test key", `require ["fileinto","variables"]; if header :is "subject" "${envelope.from}" { fileinto "x"; }`, all, `${envelope.from} needs require "envelope"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := sieve.DefaultOptions()
			opts.EnabledExtensions = tc.enabled
			_, err := sieve.Load(strings.NewReader(tc.script), opts)
			if err == nil {
				t.Fatal("load succeeded, want an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}

	accepted := []string{
		`require ["fileinto","variables","envelope"]; fileinto "${envelope.from}";`,
		`require ["fileinto","variables"]; set "box" "Lists"; fileinto "${box}";`,
		`require ["fileinto","variables"]; if header :matches "subject" "*" { fileinto "${1}"; }`,
		`require ["fileinto","variables"]; fileinto "${ENVELOPE_LIKE}";`,
		// Without the variables extension "${envelope.from}" is literal text.
		`require "fileinto"; fileinto "${envelope.from}";`,
		`require ["fileinto","variables"]; fileinto "${1}";`,
	}
	for _, script := range accepted {
		opts := sieve.DefaultOptions()
		opts.EnabledExtensions = all
		if _, err := sieve.Load(strings.NewReader(script), opts); err != nil {
			t.Errorf("%s: %v", script, err)
		}
	}
}
