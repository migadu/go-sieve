package tests

import "testing"

// RFC 5229 §3: "When a string is evaluated, substrings matching variable-ref
// SHALL be replaced by the value of variable-name. Only one pass through the
// string SHALL be done." A match key holding a variable reference was
// expanded twice, so a variable whose value is "${...}" text, from the
// message or built as RFC 5229 §3.1 suggests, was read as a reference again.
//
// "${dollar}{b}" is one pass: "${dollar}" becomes "$" and "{b}" stays, so s
// holds the text "${b}", not the value of b.
func TestMatchKeyExpandedOnce(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "variables";
require "body";

test_set "message" text:
From: a@example.com
Subject: ${b}

Hello ${b}.
.
;

test "a key is expanded once" {
	set "b" "SECRET";
	set "dollar" "$";
	set "s" "${dollar}{b}";

	if not string :is "${s}" "${s}" {
		test_fail "string :is: key ${s} was expanded again";
	}
	if not string :contains "${s}" "${s}" {
		test_fail "string :contains: key ${s} was expanded again";
	}
	if not string :matches "${s}" "${s}" {
		test_fail "string :matches: key ${s} was expanded again";
	}
	if string :is "${s}" "SECRET" {
		test_fail "string :is: the source was expanded again";
	}
	if not header :is "subject" "${s}" {
		test_fail "header :is: key ${s} was expanded again";
	}
	if not header :matches "subject" "${s}" {
		test_fail "header :matches: key ${s} was expanded again";
	}
	if not body :raw :matches "*${s}*" {
		test_fail "body :matches: key ${s} was expanded again";
	}
}

test "a key copied from the message is expanded once" {
	set "b" "SECRET";
	set "dollar" "$";
	if not header :matches "subject" "*" {
		test_fail "subject did not match *";
	}
	set "s" "${1}";
	if not string :is "${s}" "${dollar}{b}" {
		test_fail "s is not the subject";
	}
	if not header :is "subject" "${s}" {
		test_fail "the subject did not match itself";
	}
	if header :is "subject" "SECRET" {
		test_fail "the subject matched the value of b";
	}
}
`)
}

// A variable reference that a second pass would read but the script cannot
// (an unknown namespace, or envelope without require "envelope") made the
// second pass panic. The script itself was accepted: it has no such
// reference, only a variable holding the text of one.
func TestMatchKeyHoldingUnusableReference(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "variables";

test "a key holding the text of an unusable reference" {
	set "dollar" "$";
	set "ns" "${dollar}{nosuch.name}";
	set "env" "${dollar}{envelope.from}";
	if not string :is "${ns}" "${ns}" {
		test_fail "key ${ns} did not match itself";
	}
	if not string :is "${env}" "${env}" {
		test_fail "key ${env} did not match itself";
	}
}
`)
}

// RFC 5173 §6: "if the extension is present, variable references in the key
// strings or content type strings are evaluated as described in this
// document." The :content list was compared as loaded, so a type from a
// variable never matched.
func TestBodyContentTypeFromVariable(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "variables";
require "body";

test_set "message" text:
From: a@example.com
Subject: hello
Content-Type: text/plain

Hello world.
.
;

test "a :content type from a variable" {
	set "t" "text";
	set "sub" "plain";
	set "other" "image";
	if not body :content "${t}" :contains "Hello" {
		test_fail ":content ${t} did not match text/plain";
	}
	if not body :content "${t}/${sub}" :contains "Hello" {
		test_fail ":content ${t}/${sub} did not match text/plain";
	}
	if not body :content ["${other}", "${t}"] :contains "Hello" {
		test_fail ":content [${other}, ${t}] did not match text/plain";
	}
	if body :content "${other}" :contains "Hello" {
		test_fail ":content ${other} matched text/plain";
	}
	if not body :content "${unset}" :contains "Hello" {
		test_fail ":content of an unset variable (the empty string) did not match every part";
	}
}
`)
}
