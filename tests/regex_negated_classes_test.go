package tests

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/migadu/go-sieve"
	"github.com/migadu/go-sieve/interp"
)

// Negated classes exclude both cases of an ASCII letter under the default
// comparator. Equivalent spellings retain that meaning, including when a
// complement appears inside a union or is negated again. Each case goes
// through the Sieve loader and interpreter with a literal and an expanded key.
func TestRegexNegatedClasses(t *testing.T) {
	cases := []struct {
		name       string
		pattern    string
		value      string
		comparator string // empty uses the default i;ascii-casemap
		match      bool
		captures   []string
	}{
		{name: "inverse property excludes upper", pattern: `^(\P{Lu})$`, value: "A"},
		{name: "inverse property excludes lower", pattern: `^(\P{Lu})$`, value: "a"},
		{name: "outer negation excludes lower", pattern: `^([^\p{Lu}])$`, value: "a"},
		{name: "inverse lower property excludes upper", pattern: `^(\P{Ll})$`, value: "A"},
		{name: "inverse property keeps digits", pattern: `^(\P{Lu})$`, value: "7", match: true},
		{name: "double inverse includes upper", pattern: `^([^\P{Lu}])$`, value: "A", match: true},
		{name: "double inverse includes lower", pattern: `^([^\P{Lu}])$`, value: "a", match: true},
		{name: "double inverse excludes digits", pattern: `^([^\P{Lu}])$`, value: "7"},
		{name: "property caret excludes lower", pattern: `^(\p{^Lu})$`, value: "a"},
		{name: "two property inversions include lower", pattern: `^(\P{^Lu})$`, value: "a", match: true},
		{name: "outer property caret includes lower", pattern: `^([^\p{^Lu}])$`, value: "a", match: true},
		{name: "inverse universal property stays empty", pattern: `^(\P{Any})$`, value: "A"},
		{name: "double inverse universal property matches", pattern: `^([^\P{Any}])$`, value: "A", match: true},
		{name: "inverse POSIX upper excludes upper", pattern: `^([[:^upper:]])$`, value: "A"},
		{name: "inverse POSIX upper excludes lower", pattern: `^([[:^upper:]])$`, value: "a"},
		{name: "inverse POSIX lower excludes upper", pattern: `^([[:^lower:]])$`, value: "A"},
		{name: "outer POSIX negation excludes lower", pattern: `^([^[:upper:]])$`, value: "a"},
		{name: "double POSIX inverse includes lower", pattern: `^([^[:^upper:]])$`, value: "a", match: true},
		{name: "inverse union explicitly includes A", pattern: `^([a\P{Lu}])$`, value: "A", match: true},
		{name: "inverse union excludes B", pattern: `^([a\P{Lu}])$`, value: "B"},
		{name: "negated inverse union excludes a", pattern: `^([^a\P{Lu}])$`, value: "a"},
		{name: "negated inverse union includes B", pattern: `^([^a\P{Lu}])$`, value: "B", match: true},
		{name: "POSIX inverse union excludes B", pattern: `^([a[:^upper:]])$`, value: "B"},
		{name: "negated POSIX inverse union includes B", pattern: `^([^a[:^upper:]])$`, value: "B", match: true},
		{name: "ASCII inverse excludes Unicode upper", pattern: `^(\P{Lu})$`, value: "É"},
		{name: "ASCII inverse preserves Unicode lower", pattern: `^(\P{Lu})$`, value: "é", match: true},
		{name: "ASCII double inverse preserves Unicode upper", pattern: `^([^\P{Lu}])$`, value: "É", match: true},
		{name: "ASCII double inverse excludes Unicode lower", pattern: `^([^\P{Lu}])$`, value: "é"},
		{name: "Unicode inverse excludes Unicode lower", pattern: `^(\P{Lu})$`, value: "é", comparator: "i;unicode-casemap"},
		{name: "Unicode double inverse includes Unicode lower", pattern: `^([^\P{Lu}])$`, value: "é", comparator: "i;unicode-casemap", match: true},
		{name: "octet inverse includes ASCII lower", pattern: `^(\P{Lu})$`, value: "a", comparator: "i;octet", match: true},
		{name: "octet double inverse excludes ASCII lower", pattern: `^([^\P{Lu}])$`, value: "a", comparator: "i;octet"},
		{name: "octet inverse lower includes ASCII upper", pattern: `^([[:^lower:]])$`, value: "A", comparator: "i;octet", match: true},
		{name: "ASCII literal excludes Kelvin sign", pattern: `^(k)$`, value: "K"},
		{name: "Unicode literal captures Kelvin sign", pattern: `^(k)$`, value: "K", comparator: "i;unicode-casemap", match: true},
		{name: "original capture spelling", pattern: `^([^\P{Lu}]+)-([[:upper:]]+)$`, value: "UrGeNt-SuBjEcT", match: true, captures: []string{"UrGeNt-SuBjEcT", "UrGeNt", "SuBjEcT"}},
	}
	for _, tc := range cases {
		for _, dynamic := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/dynamic=%v", tc.name, dynamic), func(t *testing.T) {
				opts := sieve.DefaultOptions()
				opts.EnabledExtensions = []string{"regex", "variables", "comparator-i;unicode-casemap"}
				requires := `require ["regex", "variables"];`
				comparator := ""
				if tc.comparator != "" {
					comparator = ":comparator " + strconv.Quote(tc.comparator)
					if tc.comparator == "i;unicode-casemap" {
						requires += ` require "comparator-i;unicode-casemap";`
					}
				}
				key := tc.pattern
				if dynamic {
					key = "${pattern}"
				}
				script := fmt.Sprintf(`%s
set "input" %s;
set "pattern" %s;
if string :regex %s "${input}" %s {
	set "matched" "yes";
	set "whole" "${0}";
	set "first" "${1}";
	set "second" "${2}";
}`, requires, strconv.Quote(tc.value), strconv.Quote(tc.pattern), comparator, strconv.Quote(key))
				loaded, err := sieve.Load(strings.NewReader(script), opts)
				if err != nil {
					t.Fatalf("load: %v", err)
				}
				data := sieve.NewRuntimeData(loaded, interp.DummyPolicy{}, interp.EnvelopeStatic{}, interp.MessageStatic{})
				if err := loaded.Execute(context.Background(), data); err != nil {
					t.Fatalf("execute: %v", err)
				}
				if got := data.Variables["matched"] == "yes"; got != tc.match {
					t.Fatalf("%q against %q: match %v, want %v", tc.pattern, tc.value, got, tc.match)
				}
				if !tc.match {
					return
				}
				captures := tc.captures
				if captures == nil {
					captures = []string{tc.value, tc.value, ""}
				}
				for i, name := range []string{"whole", "first", "second"} {
					if got := data.Variables[name]; got != captures[i] {
						t.Errorf("capture %d = %q, want %q", i, got, captures[i])
					}
				}
			})
		}
	}
}
