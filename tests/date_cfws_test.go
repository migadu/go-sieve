package tests

import (
	"strings"
	"testing"
)

// The date test (RFC 5260 §4) reads a Date: header with comments and white
// space between the tokens of the obsolete date-time syntax. RFC 5322 §4.3:
// it "permits comments and folding white space between many of the tokens"
// (obs-day-of-week = [CFWS] day-name [CFWS], likewise obs-day, obs-year,
// obs-hour, obs-minute and obs-second), and §4: such forms "MUST be accepted
// and parsed by a conformant receiver". Read by net/mail.ParseDate, each of
// these was not a date, so every date test on it was false.
func TestDateCFWS(t *testing.T) {
	var script strings.Builder
	script.WriteString(`
require "vnd.dovecot.testsuite";
require "date";
require "relational";
require "comparator-i;ascii-numeric";
`)
	for _, date := range []string{
		"Wed, 17 Jul 1996 02:23:25 (c) -0700",
		"Wed , 17 Jul 1996 02:23:25 -0700",
		"Wed, 17 Jul 1996 02 : 23 : 25 -0700",
		"(Wed) 17 Jul 1996 02:23:25 -0700",
		"Wed, 17 (July) Jul 1996 02:23:25 -0700",
		"Wed, 17 Jul 1996 02:23:25 (Pacific) PDT",
	} {
		script.WriteString(`
test_set "message" text:
Date: ` + date + `
Subject: CFWS

Test.
.
;

test "` + date + `" {
	if not date :count "eq" :comparator "i;ascii-numeric" "date" "year" "1" {
		test_fail "not a date";
	}
	if not date :zone "-0700" "date" "hour" "02" {
		test_fail "hour in -0700 is not 02";
	}
	if not date :originalzone "date" "zone" "-0700" {
		test_fail "original zone is not -0700";
	}
	if not date :zone "+0000" "date" "date" "1996-07-17" {
		test_fail "not 1996-07-17 UTC";
	}
	if not date :zone "+0000" "date" "time" "09:23:25" {
		test_fail "not 09:23:25 UTC";
	}
}
`)
	}
	RunDovecotTestInline(t, "", script.String())
}
