package tests

import (
	"fmt"
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // the zones the test runs in, on hosts without zoneinfo
)

// The date test rejects malformed zone syntax instead of recovering it as
// UTC (RFC 5260 section 4). Known obsolete and unknown alphabetic names remain
// accepted according to RFC 5322 section 4.3, independently of the host's zone.
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

test "%[1]s is invalid" {
	if not date :count "eq" :comparator "i;ascii-numeric" "date" "year" "0" {
		test_fail "malformed zone counted as a date";
	}
	if date :originalzone "date" "hour" "12" {
		test_fail "malformed zone matched original hour";
	}
	if date :originalzone "date" "zone" "+0000" {
		test_fail "malformed zone matched UTC";
	}
	if date :zone "+0100" "date" "time" "13:00:00" {
		test_fail "malformed zone was shifted as UTC";
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
