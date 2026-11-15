package tests

import (
	"strings"
	"testing"

	"github.com/migadu/go-sieve"
)

var argumentRulesExtensions = []string{
	"fileinto", "envelope", "copy", "vacation", "variables", "relational",
	"body", "date", "editheader",
	"comparator-i;octet", "comparator-i;ascii-casemap",
	"comparator-i;ascii-numeric", "comparator-i;unicode-casemap",
}

func loadWithArgumentRulesExtensions(script string) error {
	opts := sieve.DefaultOptions()
	opts.EnabledExtensions = argumentRulesExtensions
	_, err := sieve.Load(strings.NewReader(script), opts)
	return err
}

// RFC 5228 §2.7.3: "Comparators other than "i;octet" and "i;ascii-casemap"
// must be declared with require, as they are extensions. [...] If the
// comparator is not declared with require, it is also an error, even if the
// comparator is supported."
func TestComparatorNeedsRequire(t *testing.T) {
	refused := []struct {
		name    string
		script  string
		wantErr string
	}{
		{"header", `if header :is :comparator "i;ascii-numeric" "X-Count" "1" { stop; }`, `comparator-i;ascii-numeric`},
		{"header :value", `require "relational"; if header :value "eq" :comparator "i;unicode-casemap" "Subject" "x" { stop; }`, `comparator-i;unicode-casemap`},
		{"another comparator required", `require "comparator-i;unicode-casemap"; if header :comparator "i;ascii-numeric" "X-Count" "1" { stop; }`, `comparator-i;ascii-numeric`},
		{"address", `if address :comparator "i;unicode-casemap" "from" "a@example.com" { stop; }`, `comparator-i;unicode-casemap`},
		{"envelope", `require "envelope"; if envelope :comparator "i;unicode-casemap" "from" "a@example.com" { stop; }`, `comparator-i;unicode-casemap`},
		{"string", `require "variables"; if string :comparator "i;ascii-numeric" "1" "1" { stop; }`, `comparator-i;ascii-numeric`},
		{"body", `require "body"; if body :comparator "i;unicode-casemap" :contains "x" { stop; }`, `comparator-i;unicode-casemap`},
		{"date", `require ["date", "relational"]; if date :value "ge" :comparator "i;ascii-numeric" "date" "year" "2000" { stop; }`, `comparator-i;ascii-numeric`},
		{"currentdate", `require ["date", "relational"]; if currentdate :value "ge" :comparator "i;ascii-numeric" "year" "2000" { stop; }`, `comparator-i;ascii-numeric`},
		{"deleteheader with value patterns", `require "editheader"; deleteheader :comparator "i;ascii-numeric" "X-Count" "1";`, `comparator-i;ascii-numeric`},
		{"deleteheader without value patterns", `require "editheader"; deleteheader :comparator "i;ascii-numeric" "X-Count";`, `comparator-i;ascii-numeric`},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			err := loadWithArgumentRulesExtensions(tc.script)
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
		`if header :comparator "i;ascii-casemap" :contains "Subject" "x" { stop; }`,
		`if header :is "Subject" "x" { stop; }`,
		`require "comparator-i;ascii-numeric"; if header :is :comparator "i;ascii-numeric" "X-Count" "1" { stop; }`,
		`require ["relational", "comparator-i;unicode-casemap"]; if header :value "eq" :comparator "i;unicode-casemap" "Subject" "x" { stop; }`,
		`require ["date", "relational", "comparator-i;ascii-numeric"]; if currentdate :value "ge" :comparator "i;ascii-numeric" "year" "2000" { stop; }`,
		`require ["editheader", "comparator-i;ascii-numeric"]; deleteheader :comparator "i;ascii-numeric" "X-Count" "1";`,
	}
	for _, script := range accepted {
		if err := loadWithArgumentRulesExtensions(script); err != nil {
			t.Errorf("%s: %v", script, err)
		}
	}
}

// RFC 5228 §2.6.2: "Tagged arguments must appear before positional arguments,
// but they may appear in any order with other tagged arguments."
func TestTaggedArgumentsBeforePositional(t *testing.T) {
	refused := []struct {
		name   string
		script string
	}{
		{"match type between positionals", `if header "Subject" :is "x" { stop; }`},
		{"match type after positionals", `if header "Subject" "x" :is { stop; }`},
		{"comparator after positionals", `if header "Subject" "x" :comparator "i;octet" { stop; }`},
		{"address part", `if address "from" :all "a@example.com" { stop; }`},
		{"size", `if size 100 :over { stop; }`},
		{"fileinto :copy", `require ["fileinto", "copy"]; fileinto "Archive" :copy;`},
		{"redirect :copy", `require "copy"; redirect "a@example.com" :copy;`},
		{"vacation :days", `require "vacation"; vacation "away" :days 3;`},
		{"set modifier", `require "variables"; set "a" :lower "B";`},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			err := loadWithArgumentRulesExtensions(tc.script)
			if err == nil {
				t.Fatal("load succeeded, want an error")
			}
			if !strings.Contains(err.Error(), "after positional arguments") {
				t.Fatalf("error %q, want it to say the tag is after positional arguments", err)
			}
		})
	}

	accepted := []string{
		`if header :is "Subject" "x" { stop; }`,
		`if header :comparator "i;octet" :is "Subject" "x" { stop; }`,
		`if header :is :comparator "i;octet" "Subject" "x" { stop; }`,
		`if address :all :is "from" "a@example.com" { stop; }`,
		`if size :over 100 { stop; }`,
		`require ["fileinto", "copy"]; fileinto :copy "Archive";`,
		`require "vacation"; vacation :days 3 "away";`,
		`require "variables"; set :lower "a" "B";`,
		`if anyof (header :is "Subject" "x", not size :under 10) { stop; }`,
	}
	for _, script := range accepted {
		if err := loadWithArgumentRulesExtensions(script); err != nil {
			t.Errorf("%s: %v", script, err)
		}
	}
}
