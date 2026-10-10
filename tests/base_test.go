package tests

import (
	"path/filepath"
	"testing"
)

func TestTestsuite(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "testsuite.svtest"))
}

func TestLexer(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "lexer.svtest"))
}

func TestControlIf(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "control-if.svtest"))
}

func TestControlStop(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "control-stop.svtest"))
}

// TestTestAddress runs "Invalid addresses" from TestTestAddressInvalidUTF8Build
// instead: the pinned copy expects a non-ASCII local part to be invalid, which
// holds only for a Dovecot build without mail UTF-8 support.
func TestTestAddress(t *testing.T) {
	RunDovecotTestWithout(t, filepath.Join("pigeonhole", "tests", "test-address.svtest"),
		[]string{"Invalid addresses"})
}

// TestTestAddressInvalidUTF8Build is "Invalid addresses" as current Pigeonhole
// has it (tests/test-address.svtest.in, commit d6a78f6), verbatim. The #UTF8#
// lines are comments, as on a build with mail UTF-8 support: a non-ASCII local
// part is an address (RFC 6532), not an invalid one.
func TestTestAddressInvalidUTF8Build(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";

test_set "message" text:
From: stephan@
To: @example.org
Cc: nonsense
Resent-To:
Bcc: nico@frop.example.com, @example.org
Resent-Cc:<jürgen@example.com>
Subject: Invalid addresses

Test.
.
;

test "Invalid addresses" {
	if address :localpart "from" "stephan" {
		test_fail ":localpart matched invalid address";
	}

#UTF8#	if address :localpart "resent-cc" "jürgen" {
#UTF8#		test_fail ":localpart matched invalid UTF-8 address";
#UTF8#	}

	if address :domain "to" "example.org" {
		test_fail ":domain matched invalid address";
	}

#UTF8#	if address :domain "resent-cc" "example.com" {
#UTF8#		test_fail ":domain matched invalid UTF-8 address";
#UTF8#	}

	if not address :is :all "resent-to" "" {
		test_fail ":all failed to match empty address";
	}

	if not address :is :all "cc" "nonsense" {
		test_fail ":all failed to match invalid address";
	}

#UTF8#	if not address :is :all "resent-cc" "<jürgen@example.com>" {
#UTF8#		test_fail ":all failed to match invalid UTF-8 address";
#UTF8#	}

	if address :is :localpart "bcc" "" {
		test_fail ":localpart matched invalid address";
	}

	if address :is :domain "cc" "example.org" {
		test_fail ":domain matched invalid address";
	}
}
`)
}

func TestTestAllof(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "test-allof.svtest"))
}

func TestTestAnyof(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "test-anyof.svtest"))
}

func TestTestExists(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "test-exists.svtest"))
}

func TestTestHeader(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "test-header.svtest"))
}

func TestTestSize(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "test-size.svtest"))
}
