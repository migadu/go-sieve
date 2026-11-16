package tests

import (
	"fmt"
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // the zones the test runs in, on hosts without zoneinfo
)

// The date test (RFC 5260 §4) reads a Date: header whose zone is neither a
// numeric zone nor a name (RFC 5322 §3.3: zone = (FWS ( "+" / "-" ) 4DIGIT) /
// obs-zone) with the time of day as written and the zone "-0000", whatever
// zone the server runs in. RFC 5322 §4.3 has a name "whose meaning is not
// known" be "considered equivalent to "-0000"", and RFC 5256 §2.2 says "If the
// time zone is invalid, the date and time SHOULD be treated as UTC." RFC 5260
// §4.2 writes that zone with a positive sign ("An offset of 0 (Zulu) always has
// a positive sign") and its iso8601 date-part with "Z" ("a time zone offset of
// zero MUST be represented by "Z" and not "+00:00"").
//
// 12:00:00 GMT+1 was read as 13:00:00 +0100; +01 and CET/CEST were read by the
// server's zone; GMT+0200, UTC-0500 and PDT+0400 were not dates at all.
// Letters before a numeric offset after white space are still not a date.
func TestDateMalformedZone(t *testing.T) {
	var script strings.Builder
	script.WriteString(`
require "vnd.dovecot.testsuite";
require "date";
require "relational";
require "comparator-i;ascii-numeric";
`)
	for _, zone := range []string{
		"GMT+1", "GMT-5", "GMT+0200", "UTC-0500", "PDT+0400",
		"+01", "-03", "+01:00", "CET/CEST", "PDT/UTC", "GMT+10 (CET)",
	} {
		fmt.Fprintf(&script, `
test_set "message" text:
Date: Tue, 01 Jan 2019 12:00:00 %[1]s
Subject: %[1]s

Test.
.
;

test "%[1]s is -0000" {
	if not date :count "eq" :comparator "i;ascii-numeric" "date" "year" "1" {
		test_fail "not a date";
	}
	if not date :originalzone "date" "hour" "12" {
		test_fail "original hour is not 12";
	}
	if not date :originalzone "date" "zone" "+0000" {
		test_fail "original zone is not +0000";
	}
	if not date :originalzone "date" "iso8601" "2019-01-01T12:00:00Z" {
		test_fail "original iso8601 is not 2019-01-01T12:00:00Z";
	}
	if not date :originalzone "date" "std11" "Tue, 01 Jan 2019 12:00:00 +0000" {
		test_fail "original std11 is not Tue, 01 Jan 2019 12:00:00 +0000";
	}
	if not date :zone "+0100" "date" "time" "13:00:00" {
		test_fail "not 13:00:00 in +0100";
	}
}
`, zone)
	}
	script.WriteString(`
test_set "message" text:
Date: Tue, 01 Jan 2019 12:00:00 PM -0700
Subject: Letters before an offset

Test.
.
;

test "Letters before an offset are not a zone" {
	if not date :count "eq" :comparator "i;ascii-numeric" "date" "year" "0" {
		test_fail "a date";
	}
	if date :originalzone "date" "hour" "12" {
		test_fail "original hour is 12";
	}
}
`)

	saved := time.Local
	t.Cleanup(func() { time.Local = saved })
	for _, name := range []string{"UTC", "Africa/Casablanca", "America/Sao_Paulo", "Europe/Berlin", "America/Los_Angeles"} {
		loc, err := time.LoadLocation(name)
		if err != nil {
			t.Fatal(err)
		}
		time.Local = loc // no test in this package runs in parallel
		t.Run(name, func(t *testing.T) {
			RunDovecotTestInline(t, "", script.String())
		})
	}
}
