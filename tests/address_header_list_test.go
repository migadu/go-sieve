package tests

import "testing"

// RFC 5228 §5.1: "Implementations MUST restrict the address test to headers
// that contain addresses". An address test on a header outside that list is
// false: the header is skipped. When the header and address tests took :index
// (RFC 5260 §6), such a header was read as one with no fields, and an absent
// header matches the empty string, so `address :matches "X-Envelope-From" "*"`
// became true for every message.
func TestAddressSkipsNonAddressHeader(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "index";

test_set "message" text:
From: sender@example.com
Subject: hello
X-Envelope-From: bounce@example.com

body
.
;

test "a header that is not an address header is skipped" {
	if address :matches "X-Envelope-From" "*" {
		test_fail "X-Envelope-From matched";
	}
	if address :is "Subject" "" {
		test_fail "Subject matched the empty string";
	}
	if address :matches "Return-Path" "*" {
		test_fail "the absent Return-Path matched";
	}
	if address :matches ["X-Envelope-From", "Subject"] "*" {
		test_fail "a list of such headers matched";
	}
}

test "such a header does not count for :index" {
	if not address :index 1 ["X-Envelope-From", "From"] "sender@example.com" {
		test_fail "the 1st field is not From";
	}
	if address :index 2 :matches ["X-Envelope-From", "From"] "*" {
		test_fail "a 2nd field was found";
	}
}
`)
}
