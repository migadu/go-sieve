package tests

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/migadu/go-sieve"
)

// RFC 5260 §6: "The "index" extension, if specified, adds optional :index and
// :last arguments to the header, address, and date tests". Only the date test
// took them; header and address refused both ("unknown tagged argument").

// The suite's date tests read the date-time after the semicolon of a
// Received: field, which the date test does not do yet (RFC 5260 §4,
// migadu/sora#1376); TestIndexDateCount covers :index on the date test.
func TestExtensionsIndexBasic(t *testing.T) {
	RunDovecotTestWithout(t, filepath.Join("pigeonhole", "tests", "extensions", "index", "basic.svtest"),
		[]string{"Date :index", "Date :index :last"})
}

func TestExtensionsIndexErrors(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "extensions", "index", "errors.svtest"))
}

// "If :index <fieldno> is specified, the attempts to match a value are limited
// to the header field fieldno (beginning at 1, the first named header field).
// If :last is also specified, the count is backwards [...]. If more than one
// header field name is specified, all the named header fields are counted in
// the order specified by the header-list." And ":index only counts separate
// header fields, not multiple occurrences within a single field."
func TestIndexHeaderAddress(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "index";
require "mime";
require "relational";
require "comparator-i;ascii-numeric";

test_set "message" text:
From: sender@example.com
To: first@example.com
X-A: First
Cc: c1@example.com, c2@example.com
To: second@example.com, extra@example.com
X-B: B-one
X-A: Second
Cc: c3@example.com
X-A: Third
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary=b

--b
Content-Type: text/plain
X-Part: one
X-Addr: p1@example.com

one
--b
Content-Type: text/html
X-Part: two
X-Addr: p2@example.com

two
--b--
.
;

test "header :index counts the fields of all names in list order" {
	if not header :index 4 ["X-A", "X-B"] "B-one" {
		test_fail "4th of X-A, X-B is not B-one";
	}
	if not header :index 1 ["X-B", "X-A"] "B-one" {
		test_fail "1st of X-B, X-A is not B-one";
	}
	if not header :index 2 ["X-B", "X-A"] "First" {
		test_fail "2nd of X-B, X-A is not First";
	}
	if header :index 2 ["X-B", "X-A"] ["B-one", "Second", "Third"] {
		test_fail "2nd of X-B, X-A matched another field";
	}
}

test "header :index :last counts backwards over all names" {
	if not header :index 1 :last ["X-B", "X-A"] "Third" {
		test_fail "last of X-B, X-A is not Third";
	}
	if not header :index 1 :last ["X-A", "X-B"] "B-one" {
		test_fail "last of X-A, X-B is not B-one";
	}
	if not header :last :index 4 ["X-A", "X-B"] "First" {
		test_fail "4th from last of X-A, X-B is not First";
	}
}

test "header :index past the fields matches nothing" {
	if header :matches :index 5 ["X-A", "X-B"] "*" {
		test_fail ":index 5 of 4 fields matched";
	}
	if header :matches :index 5 :last ["X-A", "X-B"] "*" {
		test_fail ":index 5 :last of 4 fields matched";
	}
	if header :matches :index 1 "X-Missing" "*" {
		test_fail ":index of a missing header matched";
	}
}

test "header :count with :index counts the selected field" {
	if not header :count "eq" :comparator "i;ascii-numeric" :index 2 "X-A" "1" {
		test_fail "count of one X-A field is not 1";
	}
	if not header :count "eq" :comparator "i;ascii-numeric" :index 9 "X-A" "0" {
		test_fail "count past the fields is not 0";
	}
}

test "address :index counts fields, not addresses" {
	if not address :index 2 "To" "second@example.com" {
		test_fail "2nd To has no second@example.com";
	}
	if not address :index 2 "To" "extra@example.com" {
		test_fail "2nd To has no extra@example.com";
	}
	if address :index 2 "To" "first@example.com" {
		test_fail "2nd To matched the 1st";
	}
	if not address :count "eq" :comparator "i;ascii-numeric" :index 2 "To" "2" {
		test_fail "2nd To does not hold 2 addresses";
	}
}

test "address :index counts the fields of all names in list order" {
	if not address :index 3 ["To", "Cc"] "c2@example.com" {
		test_fail "3rd of To, Cc has no c2@example.com";
	}
	if address :index 3 ["To", "Cc"] ["second@example.com", "c3@example.com"] {
		test_fail "3rd of To, Cc matched another field";
	}
	if not address :localpart :index 1 :last ["To", "Cc"] "c3" {
		test_fail "last of To, Cc is not c3";
	}
	if not address :domain :last :index 4 ["Cc", "To"] "example.com" {
		test_fail "4th from last of Cc, To does not exist";
	}
	if address :matches :index 5 ["To", "Cc"] "*" {
		test_fail ":index 5 of 4 fields matched";
	}
}

test "header :mime :anychild :index counts by name, then by part" {
	if not header :mime :anychild :contenttype :index 2 "Content-Type" "text/plain" {
		test_fail "2nd Content-Type is not text/plain";
	}
	if not header :mime :anychild :contenttype :index 1 :last "Content-Type" "text/html" {
		test_fail "last Content-Type is not text/html";
	}
	if not header :mime :anychild :index 2 ["X-Part", "Content-Type"] "two" {
		test_fail "2nd of X-Part, Content-Type is not two";
	}
	if not header :mime :anychild :contenttype :index 3 ["X-Part", "Content-Type"] "multipart/mixed" {
		test_fail "3rd of X-Part, Content-Type is not the message's";
	}
}

test "address :mime :anychild :index counts by name, then by part" {
	if not address :mime :anychild :index 2 "X-Addr" "p2@example.com" {
		test_fail "2nd X-Addr is not p2@example.com";
	}
	if address :mime :anychild :index 1 "X-Addr" "p2@example.com" {
		test_fail "1st X-Addr matched the 2nd";
	}
	if not address :mime :anychild :index 2 ["X-Addr", "From"] "p2@example.com" {
		test_fail "2nd of X-Addr, From is not p2@example.com";
	}
	if not address :mime :anychild :index 1 :last ["X-Addr", "From"] "sender@example.com" {
		test_fail "last of X-Addr, From is not sender@example.com";
	}
}
`)
}

// RFC 5260 §4: "The count of a date test is 1 if the specified field exists
// and contains a valid date; 0, otherwise." With :index the specified field is
// the one it selects; the count was of every field with a valid date.
func TestIndexDateCount(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "index";
require "date";
require "relational";
require "comparator-i;ascii-numeric";

test_set "message" text:
Resent-Date: Wed, 12 Nov 2014 18:18:31 +0100
Resent-Date: Wed, 12 Nov 2014 18:18:30 +0100
Subject: two Resent-Date fields

Test.
.
;

test "date :count with :index counts the selected field" {
	if not date :count "eq" :comparator "i;ascii-numeric" :index 2 "resent-date" "year" "1" {
		test_fail "count of the 2nd Resent-Date is not 1";
	}
	if not date :count "eq" :comparator "i;ascii-numeric" :index 1 :last "resent-date" "year" "1" {
		test_fail "count of the last Resent-Date is not 1";
	}
	if not date :count "eq" :comparator "i;ascii-numeric" :index 3 "resent-date" "year" "0" {
		test_fail "count past the Resent-Date fields is not 0";
	}
	if not date :index 2 "resent-date" "second" "30" {
		test_fail "2nd Resent-Date is not at :30";
	}
}
`)
}

// The index extension's argument rules, as Pigeonhole's
// tests/extensions/index/errors/syntax.sieve checks them one per script:
// ":index" takes a number, which counts from 1, and "Specifying :last
// without :index is an error". Without require "index" the tags are unknown.
func TestIndexArgumentRules(t *testing.T) {
	cases := []struct {
		name   string
		script string
		ok     bool
	}{
		{"header :last :index", `require "index"; if header :last :index 2 "to" "ok" { }`, true},
		{"header :index :last", `require "index"; if header :index 444 :last "to" "ok" { }`, true},
		{"header with match type", `require "index"; if header :matches :index 1 :comparator "i;octet" "to" "ok*" { }`, true},
		{"address", `require "index"; if address :index 1 "From" "a@example.com" { }`, true},
		{"address :last with address part", `require "index"; if address :localpart :index 1 :last "From" "a" { }`, true},
		{"date", `require ["date", "index"]; if date :index 2 "received" "year" "2026" { }`, true},

		{"header :index without number", `require "index"; if header :index "to" "ok" {}`, false},
		{"header :index then :last", `require "index"; if header :index :last "to" "ok" {}`, false},
		{"header :index string", `require "index"; if header :index "frop" "to" "ok" {}`, false},
		{"header :last without :index", `require "index"; if header :last "to" "ok" {}`, false},
		{"header :index 0", `require "index"; if header :index 0 "to" "ok" {}`, false},
		{"header :index 0 :last", `require "index"; if header :index 0 :last "to" "ok" {}`, false},
		{"header without require", `if header :index 1 "to" "ok" {}`, false},
		{"header :last without require", `if header :last "to" "ok" {}`, false},
		{"address :last without :index", `require "index"; if address :last "From" "a@example.com" {}`, false},
		{"address :index 0", `require "index"; if address :index 0 "From" "a@example.com" {}`, false},
		{"address without require", `if address :index 1 "From" "a@example.com" {}`, false},
		{"date :index 0", `require ["date", "index"]; if date :index 0 "received" "year" "2026" {}`, false},
		{"date :last without :index", `require ["date", "index"]; if date :last "received" "year" "2026" {}`, false},
		{"date without require", `require "date"; if date :index 1 "received" "year" "2026" {}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := sieve.DefaultOptions()
			opts.EnabledExtensions = []string{"index", "date", "comparator-i;octet"}
			_, err := sieve.Load(strings.NewReader(tc.script), opts)
			if tc.ok && err != nil {
				t.Fatalf("did not compile: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("compiled; want an error")
			}
		})
	}
}
