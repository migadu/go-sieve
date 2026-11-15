package tests

import "testing"

// TestDateSeesHeaderEdits covers that the date test reads the header as
// addheader and deleteheader left it. RFC 5293 §7: "Tests and actions such as
// "exists", "header", or "vacation" [VACATION] that examine header fields MUST
// examine the current state of a header as modified by any actions that have
// taken place so far."
//
// The date test read the message's original fields: it did not find a field
// that addheader added, and still found one that deleteheader removed.
func TestDateSeesHeaderEdits(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "date";
require "editheader";
require "relational";
require "comparator-i;ascii-numeric";

test_set "message" text:
From: a@example.com
Date: Tue, 1 Apr 1997 09:06:31 -0800
Subject: hello

body
.
;

test "a deleted field is not found" {
	deleteheader "Date";
	if exists "Date" {
		test_fail "exists found the deleted Date";
	}
	if date :is "Date" "year" "1997" {
		test_fail "date found the deleted Date";
	}
	if not date :count "eq" :comparator "i;ascii-numeric" "Date" "year" "0" {
		test_fail "count of the deleted Date is not 0";
	}
}

test "an added field is found" {
	addheader "X-D" "Tue, 1 Apr 1997 09:06:31 -0800";
	if not date :is "X-D" "year" "1997" {
		test_fail "date did not find the added X-D";
	}
	if not date :count "eq" :comparator "i;ascii-numeric" "X-D" "year" "1" {
		test_fail "count of the added X-D is not 1";
	}
}

test "a field added first is the first field" {
	addheader "Date" "Wed, 1 Apr 1998 09:06:31 -0800";
	if not date :is "Date" "year" "1998" {
		test_fail "date did not read the Date added before the original";
	}
	if date :is "Date" "year" "1997" {
		test_fail "date read the original Date, not the first";
	}
}
`)
}
