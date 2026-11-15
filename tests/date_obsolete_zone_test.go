package tests

import (
	"testing"
	"time"
	_ "time/tzdata" // the zones the test runs in, on hosts without zoneinfo
)

// The date test (RFC 5260 §4) reads an obsolete alphabetic zone of a Date:
// header by RFC 5322 §4.3, whatever zone the server runs in: "PDT is
// semantically equivalent to -0700", and a military zone or a name whose
// meaning is not known "SHOULD ... be considered equivalent to "-0000"".
// Read in the server's zone, PDT was +0000 on a server in UTC, and a
// lower-case or military zone was not a date at all.
func TestDateObsoleteZone(t *testing.T) {
	const script = `
require "vnd.dovecot.testsuite";
require "date";
require "relational";
require "comparator-i;ascii-numeric";

test_set "message" text:
Date: Wed, 17 Jul 1996 02:23:25 PDT
Subject: PDT

Test.
.
;

test "PDT is -0700" {
	if not date :zone "-0700" "date" "hour" "02" {
		test_fail "PDT not shifted as -0700";
	}
	if not date :originalzone "date" "zone" "-0700" {
		test_fail "original zone of PDT is not -0700";
	}
	if not date :zone "+0000" "date" "time" "09:23:25" {
		test_fail "PDT not 09:23:25 UTC";
	}
}

test_set "message" text:
Date: Wed, 17 Jul 1996 02:23:25 pdt
Subject: pdt

Test.
.
;

test "pdt is -0700" {
	if not date :count "eq" :comparator "i;ascii-numeric" "date" "year" "1" {
		test_fail "pdt is not a date";
	}
	if not date :zone "-0700" "date" "hour" "02" {
		test_fail "pdt not shifted as -0700";
	}
	if not date :originalzone "date" "zone" "-0700" {
		test_fail "original zone of pdt is not -0700";
	}
}

test_set "message" text:
Date: Wed, 17 Jul 1996 02:23:25 EST
Subject: EST in July

Test.
.
;

test "EST is -0500 in July" {
	if not date :originalzone "date" "zone" "-0500" {
		test_fail "original zone of EST is not -0500";
	}
	if not date :zone "+0000" "date" "time" "07:23:25" {
		test_fail "EST not 07:23:25 UTC";
	}
}

test_set "message" text:
Date: Wed, 17 Jul 1996 02:23:25 Z
Subject: Military zone

Test.
.
;

test "Military zone is -0000" {
	if not date :count "eq" :comparator "i;ascii-numeric" "date" "year" "1" {
		test_fail "a military zone is not a date";
	}
	if not date :originalzone "date" "zone" "+0000" {
		test_fail "original zone of Z is not +0000";
	}
	if not date :zone "+0000" "date" "time" "02:23:25" {
		test_fail "Z not 02:23:25 UTC";
	}
}

test_set "message" text:
Date: Wed, 17 Jan 1996 02:23:25 CET
Subject: Unknown zone

Test.
.
;

test "Unknown zone is -0000" {
	if not date :originalzone "date" "zone" "+0000" {
		test_fail "original zone of CET is not +0000";
	}
	if not date :zone "+0000" "date" "time" "02:23:25" {
		test_fail "CET not 02:23:25 UTC";
	}
}
`
	saved := time.Local
	t.Cleanup(func() { time.Local = saved })
	for _, name := range []string{"UTC", "America/Los_Angeles", "America/New_York", "Europe/Berlin"} {
		loc, err := time.LoadLocation(name)
		if err != nil {
			t.Fatal(err)
		}
		time.Local = loc // no test in this package runs in parallel
		t.Run(name, func(t *testing.T) {
			RunDovecotTestInline(t, "", script)
		})
	}
}
