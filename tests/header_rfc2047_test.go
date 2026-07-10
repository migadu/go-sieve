package tests

import (
	"testing"
)

// Header values using RFC 2047 encoded-words must be decoded to UTF-8
// before comparison (RFC 5228, Section 2.7.2).
func TestHeaderRFC2047Decoding(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";

test_set "message" text:
From: shop@example.com
To: steven@example.com
Subject: =?UTF-8?Q?Steven,_Bestellung_best=C3=A4tigt?=

Order confirmation.
.
;

test "Q-encoded UTF-8 subject :contains" {
	if not header :contains "Subject" "Bestellung bestätigt" {
		test_fail "did not match Q-encoded UTF-8 subject with :contains";
	}
}

test "Q-encoded UTF-8 subject :is" {
	if not header :is "Subject" "Steven, Bestellung bestätigt" {
		test_fail "did not match Q-encoded UTF-8 subject with :is";
	}
}

test "Q-encoded UTF-8 subject :matches" {
	if not header :matches "Subject" "*Bestellung*" {
		test_fail "did not match Q-encoded UTF-8 subject with :matches";
	}
}

test_set "message" text:
From: shop@example.com
To: steven@example.com
Subject: =?ISO-8859-1?Q?Bestellung_best=E4tigt?=

Order confirmation.
.
;

test "Q-encoded ISO-8859-1 subject" {
	if not header :contains "Subject" "Bestellung bestätigt" {
		test_fail "did not match Q-encoded ISO-8859-1 subject";
	}
}

test_set "message" text:
From: shop@example.com
To: steven@example.com
Subject: =?UTF-8?B?QmVzdGVsbHVuZyBiZXN0w6R0aWd0?=

Order confirmation.
.
;

test "B-encoded UTF-8 subject" {
	if not header :contains "Subject" "Bestellung bestätigt" {
		test_fail "did not match B-encoded UTF-8 subject";
	}
}

test_set "message" text:
From: shop@example.com
To: steven@example.com
Subject: =?UTF-8?Q?Bestellung_?= =?UTF-8?Q?best=C3=A4tigt?=

Order confirmation.
.
;

test "Adjacent encoded-words" {
	if not header :is "Subject" "Bestellung bestätigt" {
		test_fail "adjacent encoded-words were not joined per RFC 2047";
	}
}

test_set "message" text:
From: shop@example.com
To: steven@example.com
Subject: Plain ascii subject with =? that is not an encoded word

Order confirmation.
.
;

test "Malformed encoded-word left as-is" {
	if not header :contains "Subject" "with =? that" {
		test_fail "non-encoded-word text must be matched literally";
	}
}
`)
}
