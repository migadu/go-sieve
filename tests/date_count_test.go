package tests

import (
	"testing"
)

// RFC 5260 §4: "If multiple header fields with the same name are present, only
// the first field that is found is used." and "The count of a date test is 1
// if the specified field exists and contains a valid date; 0, otherwise." The
// count was of every field with a valid date, so two valid Resent-Date: fields
// counted 2.
func TestDateCountFirstField(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "date";
require "relational";
require "comparator-i;ascii-numeric";

test_set "message" text:
Resent-Date: Wed, 12 Nov 2014 18:18:31 +0100
Resent-Date: Wed, 12 Nov 2014 18:18:30 +0100
Subject: two valid Resent-Date fields

Test.
.
;

test "Two valid fields count 1" {
	if not date :count "eq" :comparator "i;ascii-numeric" "resent-date" "year" "1" {
		test_fail "count of two valid Resent-Date fields is not 1";
	}
	if date :count "gt" :comparator "i;ascii-numeric" "resent-date" "year" "1" {
		test_fail "count of two valid Resent-Date fields is more than 1";
	}
}

test_set "message" text:
Resent-Date: not a date
Resent-Date: Wed, 12 Nov 2014 18:18:30 +0100
Subject: first Resent-Date invalid

Test.
.
;

test "Invalid first field counts 0" {
	if not date :count "eq" :comparator "i;ascii-numeric" "resent-date" "year" "0" {
		test_fail "count of an invalid first Resent-Date is not 0";
	}
	if date :matches "resent-date" "year" "*" {
		test_fail "date read a Resent-Date other than the first";
	}
}

test_set "message" text:
Resent-Date: Wed, 12 Nov 2014 18:18:31 +0100
Resent-Date: not a date
Subject: second Resent-Date invalid

Test.
.
;

test "Valid first field counts 1" {
	if not date :count "eq" :comparator "i;ascii-numeric" "resent-date" "year" "1" {
		test_fail "count of a valid first Resent-Date is not 1";
	}
}

test_set "message" text:
Subject: no Resent-Date

Test.
.
;

test "Missing field counts 0" {
	if not date :count "eq" :comparator "i;ascii-numeric" "resent-date" "year" "0" {
		test_fail "count of a missing Resent-Date is not 0";
	}
}
`)
}
