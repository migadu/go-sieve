package tests

import (
	"testing"
)

// The date test reads the date-time at the end of a Received: field, after its
// last semicolon (RFC 5260 §4: "Implementations MUST support extraction of RFC
// 2822 date-time information that ... appears at the end of a header field
// following a semicolon (e.g., as it does in a standard Received: header
// field)"). It read only a value that was a date-time as a whole, so every date
// test on a Received: field was false and its :count 0.
func TestDateReceived(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "date";
require "index";
require "relational";
require "comparator-i;ascii-numeric";

test_set "message" text:
Received: from mx.example.org by mail.example.com; Wed, 17 Jul 1996 02:23:25 -0700
Subject: One hop

Test.
.
;

test "Received date-time" {
	if not date :originalzone "received" "hour" "02" {
		test_fail "hour of the Received: date-time is not 02";
	}
	if not date :originalzone "received" "std11" "Wed, 17 Jul 1996 02:23:25 -0700" {
		test_fail "Received: date-time is not Wed, 17 Jul 1996 02:23:25 -0700";
	}
	if not date :zone "+0000" "received" "time" "09:23:25" {
		test_fail "Received: date-time in zone +0000 is not 09:23:25";
	}
}

test "Received count" {
	if not date :count "eq" :comparator "i;ascii-numeric" "received" "date" "1" {
		test_fail "count of a Received: field with a date-time is not 1";
	}
}

test_set "message" text:
Received: from relay.example.org (relay.example.org [192.0.2.2])
	by mail.example.com (Postfix) with ESMTPS id 4ABC
	for <user@example.com>; Thu, 18 Jul 1996 10:00:00 +0200 (CEST)
Received: from client.example.org (helo; x) by relay.example.org;
	Wed, 17 Jul 1996 02:23:25 -0700 (PDT)
Subject: Two hops

Test.
.
;

test "Folded Received date-time" {
	if not date :originalzone "received" "date" "1996-07-18" {
		test_fail "first Received: date is not 1996-07-18";
	}
	if not date :originalzone "received" "zone" "+0200" {
		test_fail "first Received: zone is not +0200";
	}
	if not date :index 1 :last :originalzone "received" "date" "1996-07-17" {
		test_fail "last Received: date is not 1996-07-17";
	}
	if not date :index 2 :originalzone "received" "time" "02:23:25" {
		test_fail "second Received: time is not 02:23:25";
	}
}

test_set "message" text:
Received: from mx.example.org by mail.example.com; id 4ABC
Subject: No date-time

Test.
.
;

test "Received without a date-time" {
	if date :originalzone :matches "received" "date" "*" {
		test_fail "matched a Received: field without a date-time";
	}
	if not date :count "eq" :comparator "i;ascii-numeric" "received" "date" "0" {
		test_fail "count of a Received: field without a date-time is not 0";
	}
}
`)
}
