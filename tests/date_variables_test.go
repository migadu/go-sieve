package tests

import (
	"strings"
	"testing"
	"time"
)

// RFC 5229 §3: "When a string is evaluated, substrings matching variable-ref
// SHALL be replaced by the value of variable-name." RFC 5260 does not make
// the :zone or date-part of the date and currentdate tests constant, so a
// variable reference in either is expanded when the test runs. Only a literal
// is checked when the script loads, as in Pigeonhole: the value a variable
// will hold is not known then.
func TestDateLoadZoneAndDatePartWithVariables(t *testing.T) {
	accepted := []string{
		`require ["date", "variables"]; if date :zone "${z}" "date" "hour" "02" { stop; }`,
		`require ["date", "variables"]; if date "date" "${p}" "02" { stop; }`,
		`require ["date", "variables"]; if date :zone "-${z}" "date" "${P}" "02" { stop; }`,
		`require ["date", "variables"]; if currentdate :zone "${z}" "hour" "02" { stop; }`,
		`require ["date", "variables"]; if currentdate "${p}" "02" { stop; }`,
	}
	for _, script := range accepted {
		if err := loadWithArgumentRulesExtensions(script); err != nil {
			t.Errorf("%s: %v", script, err)
		}
	}

	// A literal is still checked, and so is a string that only looks like a
	// variable reference because "variables" is not required.
	refused := []struct {
		script  string
		wantErr string
	}{
		{`require "date"; if date :zone "${z}" "date" "hour" "02" { stop; }`, `invalid zone`},
		{`require "date"; if date "date" "${p}" "02" { stop; }`, `invalid date-part`},
		{`require "date"; if currentdate :zone "${z}" "hour" "02" { stop; }`, `invalid zone`},
		{`require "date"; if currentdate "${p}" "02" { stop; }`, `invalid date-part`},
		{`require ["date", "variables"]; if date :zone "PDT" "date" "hour" "02" { stop; }`, `invalid zone`},
		{`require ["date", "variables"]; if date "date" "fortnight" "02" { stop; }`, `invalid date-part`},
		{`require ["date", "variables"]; if currentdate :zone "+07" "hour" "02" { stop; }`, `invalid zone`},
		{`require ["date", "variables"]; if currentdate "fortnight" "02" { stop; }`, `invalid date-part`},
		// RFC 5260 §4.1: time-zone = ( "+" / "-" ) 4DIGIT
		{`require "date"; if date :zone "+-130" "date" "hour" "02" { stop; }`, `invalid zone`},
		{`require "date"; if date :zone "++130" "date" "hour" "02" { stop; }`, `invalid zone`},
		{`require "date"; if currentdate :zone "-+130" "hour" "02" { stop; }`, `invalid zone`},
	}
	for _, tc := range refused {
		err := loadWithArgumentRulesExtensions(tc.script)
		if err == nil {
			t.Errorf("%s: loaded, want an error", tc.script)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%s: error %q, want it to contain %q", tc.script, err, tc.wantErr)
		}
	}
}

// The expanded :zone and date-part are the values the variables hold when
// the test runs (RFC 5229 §3: "The expanded string MUST use the variable
// values that are current when control reaches the statement the string is
// part of."). As in Pigeonhole, an expanded zone that is not a time-zone of
// RFC 5260 §4.1 is the local zone, and an unknown expanded date-part makes
// the test false.
func TestDateZoneAndDatePartFromVariables(t *testing.T) {
	const script = `
require "vnd.dovecot.testsuite";
require "date";
require "variables";
require "relational";
require "comparator-i;ascii-numeric";

test_set "message" text:
Date: Wed, 17 Jul 1996 02:23:25 -0700
Subject: variables

Test.
.
;

test "date :zone and date-part from variables" {
	set "z" "-0700";
	set "p" "hour";
	if not date :zone "${z}" "date" "${p}" "02" {
		test_fail "hour in -0700 is not 02";
	}
	set "z" "+0000";
	if not date :zone "${z}" "date" "${p}" "09" {
		test_fail "the zone is not the variable's current value";
	}
	set "p" "TIME";
	if not date :zone "${z}" "date" "${p}" "09:23:25" {
		test_fail "the date-part is not the variable's current value";
	}
	set "sign" "+";
	if not date :zone "${sign}0530" "date" "zone" "+0530" {
		test_fail "a zone built from a variable is not used";
	}
}

test "an expanded zone that is not a time-zone is the local zone" {
	set "z" "PDT";
	if not date :zone "${z}" "date" "zone" "+0300" {
		test_fail "PDT is not the local zone";
	}
	set "z" "+-130";
	if not date :zone "${z}" "date" "zone" "+0300" {
		test_fail "+-130 is not the local zone";
	}
	set "z" "";
	if not date :zone "${z}" "date" "time" "12:23:25" {
		test_fail "an empty zone is not the local zone";
	}
	if not currentdate :zone "${z}" "zone" "+0300" {
		test_fail "currentdate: an empty zone is not the local zone";
	}
}

test "an unknown expanded date-part makes the test false" {
	set "p" "fortnight";
	if date :matches "date" "${p}" "*" {
		test_fail "date matched an unknown date-part";
	}
	if date :count "ge" :comparator "i;ascii-numeric" "date" "${p}" "0" {
		test_fail "date :count matched an unknown date-part";
	}
	if currentdate :matches "${p}" "*" {
		test_fail "currentdate matched an unknown date-part";
	}
}

test "currentdate :zone and date-part from variables" {
	set "z" "-0700";
	set "p" "zone";
	if not currentdate :zone "${z}" "${p}" "-0700" {
		test_fail "currentdate zone in -0700 is not -0700";
	}
	set "z" "+1245";
	if not currentdate :zone "${z}" "${p}" "+1245" {
		test_fail "currentdate zone is not the variable's current value";
	}
}
`
	saved := time.Local
	t.Cleanup(func() { time.Local = saved })
	time.Local = time.FixedZone("", 3*3600) // no test in this package runs in parallel
	RunDovecotTestInline(t, "", script)
}
