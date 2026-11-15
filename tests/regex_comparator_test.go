package tests

import (
	"strings"
	"testing"

	"github.com/migadu/go-sieve"
)

// RFC 5228 §2.7.1: "It is an error to use a comparator with ":contains" or
// ":matches" that is not compatible with it." draft-murchison-sieve-regex §3:
// "The ":regex" match type is subject to the same rules and restrictions as
// the standard match types defined in [SIEVE]." i;ascii-numeric "does not
// support the substring operation" (RFC 4790 §9.1.1), so every command that
// takes a match type refuses :regex with it when the script loads, as
// Pigeonhole does, instead of failing the test on every run.
func TestRegexRefusesNumericComparator(t *testing.T) {
	load := func(script string) error {
		opts := sieve.DefaultOptions()
		opts.EnabledExtensions = []string{
			"regex", "comparator-i;ascii-numeric", "envelope", "variables",
			"body", "date", "editheader",
		}
		_, err := sieve.Load(strings.NewReader(
			`require ["regex", "comparator-i;ascii-numeric", "envelope", "variables", "body", "date", "editheader"]; `+script), opts)
		return err
	}

	refused := []struct {
		name   string
		script string
	}{
		{"header", `if header :regex :comparator "i;ascii-numeric" "Subject" "^1" { stop; }`},
		{"address", `if address :regex :comparator "i;ascii-numeric" "from" "^1" { stop; }`},
		{"envelope", `if envelope :regex :comparator "i;ascii-numeric" "from" "^1" { stop; }`},
		{"string", `if string :regex :comparator "i;ascii-numeric" "123" "^1" { stop; }`},
		{"body", `if body :regex :comparator "i;ascii-numeric" "^1" { stop; }`},
		{"date", `if date :regex :comparator "i;ascii-numeric" "date" "year" "^2" { stop; }`},
		{"currentdate", `if currentdate :regex :comparator "i;ascii-numeric" "year" "^2" { stop; }`},
		{"deleteheader with value patterns", `deleteheader :regex :comparator "i;ascii-numeric" "X-Count" "^1";`},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			err := load(tc.script)
			if err == nil {
				t.Fatal("load succeeded, want an error")
			}
			if !strings.Contains(err.Error(), "numeric comparator") {
				t.Fatalf("error %q, want it to contain %q", err, "numeric comparator")
			}
		})
	}

	// RFC 5293 §5: "If no value-patterns are specified, then the comparator
	// and match-type options are silently ignored."
	accepted := []string{
		`deleteheader :regex :comparator "i;ascii-numeric" "X-Count";`,
		`if header :regex :comparator "i;octet" "Subject" "^1" { stop; }`,
		`if header :regex :comparator "i;ascii-casemap" "Subject" "^1" { stop; }`,
	}
	for _, script := range accepted {
		if err := load(script); err != nil {
			t.Errorf("%s: %v", script, err)
		}
	}
}
