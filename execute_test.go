package sieve

import (
	"bufio"
	"context"
	"net/textproto"
	"reflect"
	"strings"
	"testing"

	"github.com/foxcpp/go-sieve/interp"
)

var eml string = `Date: Tue, 1 Apr 1997 09:06:31 -0800 (PST)
From: coyote@desert.example.org
To: roadrunner@acme.example.com
Subject: I have a present for you

Look, I'm sorry about the whole anvil thing, and I really
didn't mean to try and drop it on you from the top of the
cliff.  I want to try to make it up to you.  I've got some
great birdseed over here at my place--top of the line
stuff--and if you come by, I'll have it all wrapped up
for you.  I'm really sorry for all the problems I've caused
for you over the years, but I know we can work this out.
--
Wile E. Coyote   "Super Genius"   coyote@desert.example.org
`

type Result struct {
	Redirect     []string
	Fileinto     []string
	ImplicitKeep bool
	Keep         bool
	Flags        []string
}

func testExecute(ctx context.Context, t *testing.T, in string, eml string, shouldFail bool, intendedResult Result) {
	t.Helper()

	msgHdr, err := textproto.NewReader(bufio.NewReader(strings.NewReader(eml))).ReadMIMEHeader()
	if err != nil {
		t.Fatal(err)
	}

	script := bufio.NewReader(strings.NewReader(in))

	// Enable all extensions for testing
	opts := DefaultOptions()
	opts.EnabledExtensions = []string{
		"fileinto", "envelope", "encoded-character",
		"comparator-i;octet", "comparator-i;ascii-casemap",
		"comparator-i;ascii-numeric", "comparator-i;unicode-casemap",
		"imap4flags", "variables", "relational", "vacation", "copy", "regex",
		"date", "index", "editheader", "mailbox", "subaddress",
	}
	loadedScript, err := Load(script, opts)
	if err != nil {
		if shouldFail {
			return
		}
		t.Fatal(err)
	}
	env := interp.EnvelopeStatic{
		From: "from@test.com",
		To:   "to@test.com",
	}
	msg := interp.MessageStatic{
		Size:   len(eml),
		Header: msgHdr,
	}
	data := NewRuntimeData(loadedScript, interp.DummyPolicy{}, env, msg)

	if err := loadedScript.Execute(ctx, data); err != nil {
		if shouldFail {
			return
		}
		t.Fatal(err)
	}

	if shouldFail {
		t.Fatal("expected test to fail, but it succeeded")
	}

	r := Result{
		Redirect:     data.RedirectAddr,
		Fileinto:     data.Mailboxes,
		Keep:         data.Keep,
		ImplicitKeep: data.ImplicitKeep,
		Flags:        data.Flags,
	}

	if !reflect.DeepEqual(r, intendedResult) {
		t.Log("Wrong Execute output")
		t.Log("Actual:  ", r)
		t.Log("Expected:", intendedResult)
		t.FailNow()
	}
}

func TestFileinto(t *testing.T) {
	ctx := context.Background()
	t.Run("single", func(t *testing.T) {
		testExecute(ctx, t, `require "fileinto"; fileinto "test";`, eml, false, Result{
			Fileinto:     []string{"test"},
			ImplicitKeep: false,
		})
	})
	t.Run("multiple", func(t *testing.T) {
		testExecute(ctx, t, `require "fileinto"; fileinto "test"; fileinto "test2";`, eml, false, Result{
			Fileinto:     []string{"test", "test2"},
			ImplicitKeep: false,
		})
	})
}

func TestRedirect(t *testing.T) {
	ctx := context.Background()
	testExecute(ctx, t, `redirect "user@example.com";`, eml, false, Result{
		Redirect:     []string{"user@example.com"},
		ImplicitKeep: false,
	})
}

func TestAddress(t *testing.T) {
	// Assumes the `address` test will trigger a `keep` action on success.
	// This is a common pattern for testing boolean tests.
	ctx := context.Background()
	t.Run("is", func(t *testing.T) {
		testExecute(ctx, t, `if address :is "From" "coyote@desert.example.org" { keep; }`, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true, // keep does NOT cancel implicit keep
		})
	})
	t.Run("contains-domain", func(t *testing.T) {
		testExecute(ctx, t, `if address :contains :domain "To" "acme.example.com" { keep; }`, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true, // keep does NOT cancel implicit keep
		})
	})
}

func TestEnvelope(t *testing.T) {
	ctx := context.Background()
	t.Run("is-from", func(t *testing.T) {
		testExecute(ctx, t, `require "envelope"; if envelope :is "from" "from@test.com" { keep; }`, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true, // keep does NOT cancel implicit keep
		})
	})
	t.Run("contains-to", func(t *testing.T) {
		testExecute(ctx, t, `require ["envelope", "copy"]; if envelope :contains "to" "test.com" { redirect :copy "another@example.com"; }`, eml, false, Result{
			Redirect:     []string{"another@example.com"},
			ImplicitKeep: true,
		})
	})
}

func TestExists(t *testing.T) {
	ctx := context.Background()
	t.Run("simple-true", func(t *testing.T) {
		// The "From" header exists in the test message.
		testExecute(ctx, t, `if exists "From" { keep; }`, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true, // keep does NOT cancel implicit keep
		})
	})
	t.Run("simple-false", func(t *testing.T) {
		// The "X-Nonexistent-Header" does not exist. The `if` block is skipped.
		testExecute(ctx, t, `if exists "X-Nonexistent-Header" { discard; }`, eml, false, Result{
			ImplicitKeep: true, // Implicit keep remains true
		})
	})
	t.Run("multiple-headers-fail", func(t *testing.T) {
		// ALL headers must exist for the test to be true (RFC 5228).
		// Since "X-Nonexistent-Header" doesn't exist, the test is false and keep is not executed.
		testExecute(ctx, t, `if exists ["X-Nonexistent-Header", "Subject"] { keep; }`, eml, false, Result{
			Keep:         false,
			ImplicitKeep: true, // No action taken, implicit keep remains
		})
	})
	t.Run("multiple-headers-pass", func(t *testing.T) {
		// Both "Subject" and "From" exist, so the test is true and keep is executed.
		testExecute(ctx, t, `if exists ["Subject", "From"] { keep; }`, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true, // keep does NOT cancel implicit keep
		})
	})
}

func TestHeader(t *testing.T) {
	ctx := context.Background()
	t.Run("is-true", func(t *testing.T) {
		testExecute(ctx, t, `if header :is "Subject" "I have a present for you" { keep; }`, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("contains-true", func(t *testing.T) {
		testExecute(ctx, t, `if header :contains "From" "desert.example" { keep; }`, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("is-false", func(t *testing.T) {
		testExecute(ctx, t, `if header :is "Subject" "Not the right subject" { keep; }`, eml, false, Result{
			ImplicitKeep: true,
		})
	})
}

func TestRegex(t *testing.T) {
	ctx := context.Background()
	t.Run("string-regex-match", func(t *testing.T) {
		// Test regex matching with string test
		script := `require ["variables", "regex"]; set "subject" "I have a present for you"; if string :comparator "i;octet" :regex "${subject}" "I have a (.*) for you" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true, // keep does NOT cancel implicit keep
		})
	})
	t.Run("header-regex-match", func(t *testing.T) {
		// Test regex matching with header test
		script := `require "regex"; if header :comparator "i;octet" :regex "Subject" "I have a (.*) for you" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true, // keep does NOT cancel implicit keep
		})
	})
	t.Run("header-regex-case-insensitive", func(t *testing.T) {
		// Test case-insensitive regex matching
		script := `require "regex"; if header :regex "Subject" "(?i)I HAVE A (.*) FOR YOU" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true, // keep does NOT cancel implicit keep
		})
	})
	t.Run("regex-no-match", func(t *testing.T) {
		// Test regex that doesn't match
		script := `require "regex"; if header :regex "Subject" "No match pattern" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			ImplicitKeep: true, // No action taken, implicit keep remains
		})
	})
	t.Run("regex-without-require-error", func(t *testing.T) {
		// Test that regex without require fails
		script := `if header :regex "Subject" "test" { keep; }`
		testExecute(ctx, t, script, eml, true, Result{})
	})
}

func TestAllOf(t *testing.T) {
	ctx := context.Background()
	t.Run("all-true", func(t *testing.T) {
		// Both `exists` and `size` are true, so the block is executed.
		script := `if allof (exists "Subject", size :over 100) { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("one-false", func(t *testing.T) {
		// The `exists` test is false, so the `allof` is false and the block is skipped.
		script := `if allof (exists "X-Nonexistent-Header", size :over 100) { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			ImplicitKeep: true,
		})
	})
}

func TestAnyOf(t *testing.T) {
	ctx := context.Background()
	t.Run("one-true", func(t *testing.T) {
		// The `exists` test is false, but `size` is true, so the block is executed.
		script := `if anyof (exists "X-Nonexistent-Header", size :over 100) { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("all-false", func(t *testing.T) {
		// Both tests are false, so the `anyof` is false and the block is skipped.
		script := `if anyof (exists "X-Nonexistent-Header", size :under 100) { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			ImplicitKeep: true,
		})
	})
}

func TestNot(t *testing.T) {
	ctx := context.Background()
	t.Run("not-true-is-false", func(t *testing.T) {
		// `exists "From"` is true, so `not exists "From"` is false. Block is skipped.
		script := `if not exists "From" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			ImplicitKeep: true,
		})
	})
	t.Run("not-false-is-true", func(t *testing.T) {
		// `exists "X-Nonexistent"` is false, so `not exists "X-Nonexistent"` is true. Block is executed.
		script := `if not exists "X-Nonexistent" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("not-allof-false-is-true", func(t *testing.T) {
		// `allof (exists "From", exists "X-Nonexistent")` is false, so `not allof (...)` is true. Block is executed.
		script := `if not allof (exists "From", exists "X-Nonexistent") { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
}

func TestSize(t *testing.T) {
	ctx := context.Background()

	t.Run("over-true", func(t *testing.T) {
		// messageSize (606) > 600 is true
		testExecute(ctx, t, `if size :over 600 { keep; }`, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true, // keep does NOT cancel implicit keep
		})
	})
	t.Run("over-false-equal", func(t *testing.T) {
		// messageSize (606) > 606 is false
		testExecute(ctx, t, `if size :over 606 { keep; }`, eml, false, Result{
			Keep:         false, // keep not executed
			ImplicitKeep: true,
		})
	})
	t.Run("over-false-greater", func(t *testing.T) {
		// messageSize (606) > 607 is false
		testExecute(ctx, t, `if size :over 607 { keep; }`, eml, false, Result{
			Keep:         false, // keep not executed
			ImplicitKeep: true,
		})
	})
	t.Run("under-true", func(t *testing.T) {
		// messageSize (606) < 607 is true
		testExecute(ctx, t, `if size :under 607 { keep; }`, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true, // keep does NOT cancel implicit keep
		})
	})
	t.Run("under-false-equal", func(t *testing.T) {
		// messageSize (606) < 606 is false
		testExecute(ctx, t, `if size :under 606 { keep; }`, eml, false, Result{
			Keep:         false, // keep not executed
			ImplicitKeep: true,
		})
	})
	t.Run("under-false-less", func(t *testing.T) {
		// messageSize (606) < 605 is false
		testExecute(ctx, t, `if size :under 605 { keep; }`, eml, false, Result{
			Keep:         false, // keep not executed
			ImplicitKeep: true,
		})
	})
	t.Run("no-tag-error", func(t *testing.T) {
		testExecute(ctx, t, `if size 100 { keep; }`, eml, true, Result{})
	})
	t.Run("both-tags-error", func(t *testing.T) {
		testExecute(ctx, t, `if size :over 100 :under 200 { keep; }`, eml, true, Result{})
	})
	t.Run("invalid-number-error", func(t *testing.T) {
		testExecute(ctx, t, `if size :over "abc" { keep; }`, eml, true, Result{})
	})
}

func TestDate(t *testing.T) {
	ctx := context.Background()
	t.Run("date-year", func(t *testing.T) {
		// Date header: Tue, 1 Apr 1997 09:06:31 -0800 (PST)
		script := `require "date"; if date :is :originalzone "date" "year" "1997" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("date-month", func(t *testing.T) {
		script := `require "date"; if date :is :originalzone "date" "month" "04" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("date-weekday", func(t *testing.T) {
		// April 1, 1997 was a Tuesday (weekday = 2)
		script := `require "date"; if date :is :originalzone "date" "weekday" "2" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("date-hour-originalzone", func(t *testing.T) {
		// The date has hour 09 in -0800 timezone
		script := `require "date"; if date :is :originalzone "date" "hour" "09" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("date-zone-shift", func(t *testing.T) {
		// Shift from -0800 to +0000, hour should be 17 (09 + 8)
		script := `require "date"; if date :is :zone "+0000" "date" "hour" "17" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("date-relational", func(t *testing.T) {
		// Year >= 1990
		script := `require ["date", "relational"]; if date :value "ge" :originalzone "date" "year" "1990" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("date-no-match", func(t *testing.T) {
		script := `require "date"; if date :is :originalzone "date" "year" "2020" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			ImplicitKeep: true,
		})
	})
	t.Run("currentdate-year", func(t *testing.T) {
		// Current year should match 20* pattern
		script := `require "date"; if currentdate :matches "year" "20*" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("date-without-require-error", func(t *testing.T) {
		script := `if date :is "date" "year" "1997" { keep; }`
		testExecute(ctx, t, script, eml, true, Result{})
	})
	t.Run("currentdate-vacation-style", func(t *testing.T) {
		// This tests the pattern: vacation with date range using currentdate
		script := `require ["date", "relational"];
		if allof (
		  currentdate :value "ge" "date" "2020-01-01",
		  currentdate :value "le" "date" "2030-12-31"
		) {
		  keep;
		}`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
}

func TestEditheader(t *testing.T) {
	ctx := context.Background()
	t.Run("addheader-and-exists", func(t *testing.T) {
		// Add a header and verify it exists
		script := `require "editheader"; addheader "X-Test" "hello"; if exists "X-Test" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("addheader-and-header-test", func(t *testing.T) {
		// Add a header and verify its value with header test
		script := `require "editheader"; addheader "X-Test" "hello world"; if header :contains "X-Test" "hello" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("addheader-without-require", func(t *testing.T) {
		// addheader without require should fail
		script := `addheader "X-Test" "hello"; keep;`
		testExecute(ctx, t, script, eml, true, Result{})
	})
	t.Run("deleteheader-without-require", func(t *testing.T) {
		// deleteheader without require should fail
		script := `deleteheader "Subject"; keep;`
		testExecute(ctx, t, script, eml, true, Result{})
	})
	t.Run("addheader-last", func(t *testing.T) {
		// Add a header at the end with :last
		script := `require "editheader"; addheader :last "X-Test" "world"; if exists "X-Test" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("deleteheader-protected-received", func(t *testing.T) {
		// Deleting "Received" should be silently ignored (protected header)
		script := `require "editheader"; deleteheader "Received"; keep;`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("deleteheader-protected-auto-submitted", func(t *testing.T) {
		// Deleting "Auto-Submitted" should be silently ignored (protected header)
		script := `require "editheader"; deleteheader "Auto-Submitted"; keep;`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("addheader-then-delete", func(t *testing.T) {
		// Add a header, then delete it - should not exist after
		script := `require "editheader"; addheader "X-Test" "value"; deleteheader "X-Test"; if not exists "X-Test" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("delete-existing-subject", func(t *testing.T) {
		// Delete the Subject header that exists in the test message
		script := `require "editheader"; deleteheader "Subject"; if not exists "Subject" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("deleteheader-with-is-match", func(t *testing.T) {
		// Delete header only if it matches a specific value
		script := `require "editheader"; deleteheader :is "Subject" "I have a present for you"; if not exists "Subject" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("deleteheader-with-is-no-match", func(t *testing.T) {
		// Don't delete if value doesn't match
		script := `require "editheader"; deleteheader :is "Subject" "wrong value"; if exists "Subject" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("deleteheader-with-contains", func(t *testing.T) {
		// Delete header if it contains a substring
		script := `require "editheader"; deleteheader :contains "Subject" "present"; if not exists "Subject" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("deleteheader-with-matches", func(t *testing.T) {
		// Delete header matching wildcard pattern
		script := `require "editheader"; deleteheader :matches "Subject" "I have*"; if not exists "Subject" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("addheader-multiple-same-name", func(t *testing.T) {
		// Add multiple headers with same name
		script := `require "editheader"; addheader "X-Test" "value1"; addheader "X-Test" "value2"; if header :contains "X-Test" "value1" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("addheader-with-variables", func(t *testing.T) {
		// Add header with variable expansion
		script := `require ["editheader", "variables"]; set "tag" "important"; addheader "X-Tag" "${tag}"; if header :is "X-Tag" "important" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("deleteheader-index", func(t *testing.T) {
		// Delete specific header by index
		script := `require "editheader"; addheader "X-Test" "first"; addheader "X-Test" "second"; deleteheader :index 1 "X-Test"; if exists "X-Test" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("deleteheader-index-last", func(t *testing.T) {
		// Delete from end with :index :last
		script := `require "editheader"; addheader "X-Test" "first"; addheader :last "X-Test" "second"; deleteheader :index 1 :last "X-Test"; if exists "X-Test" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("addheader-case-insensitive-check", func(t *testing.T) {
		// Header names are case-insensitive
		script := `require "editheader"; addheader "x-test" "hello"; if exists "X-TEST" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("deleteheader-case-insensitive", func(t *testing.T) {
		// deleteheader should be case-insensitive for header name
		script := `require "editheader"; deleteheader "SUBJECT"; if not exists "subject" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
}

func TestMailbox(t *testing.T) {
	ctx := context.Background()
	t.Run("mailboxexists-true", func(t *testing.T) {
		// Without MailboxChecker, mailboxexists returns true (optimistic)
		script := `require "mailbox"; if mailboxexists "INBOX" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("mailboxexists-multiple", func(t *testing.T) {
		// Test with multiple mailboxes
		script := `require "mailbox"; if mailboxexists ["INBOX", "Drafts"] { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("mailboxexists-without-require", func(t *testing.T) {
		// mailboxexists without require should fail
		script := `if mailboxexists "INBOX" { keep; }`
		testExecute(ctx, t, script, eml, true, Result{})
	})
	t.Run("fileinto-create", func(t *testing.T) {
		// fileinto with :create flag
		script := `require ["fileinto", "mailbox"]; fileinto :create "NewFolder";`
		testExecute(ctx, t, script, eml, false, Result{
			Fileinto:     []string{"NewFolder"},
			ImplicitKeep: false,
		})
	})
	t.Run("fileinto-create-without-require", func(t *testing.T) {
		// :create without mailbox require should fail
		script := `require "fileinto"; fileinto :create "NewFolder";`
		testExecute(ctx, t, script, eml, true, Result{})
	})
	t.Run("fileinto-create-and-copy", func(t *testing.T) {
		// fileinto with :create and :copy flags
		script := `require ["fileinto", "mailbox", "copy"]; fileinto :create :copy "NewFolder";`
		testExecute(ctx, t, script, eml, false, Result{
			Fileinto:     []string{"NewFolder"},
			ImplicitKeep: true, // :copy preserves implicit keep
		})
	})
	t.Run("mailboxexists-with-variable", func(t *testing.T) {
		// mailboxexists with variable expansion
		script := `require ["mailbox", "variables"]; set "folder" "INBOX"; if mailboxexists "${folder}" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("fileinto-create-with-variable", func(t *testing.T) {
		// fileinto :create with variable expansion
		script := `require ["fileinto", "mailbox", "variables"]; set "folder" "Archive"; fileinto :create "${folder}";`
		testExecute(ctx, t, script, eml, false, Result{
			Fileinto:     []string{"Archive"},
			ImplicitKeep: false,
		})
	})
	t.Run("fileinto-create-with-flags", func(t *testing.T) {
		// fileinto :create combined with flags
		script := `require ["fileinto", "mailbox", "imap4flags"]; fileinto :create :flags "\\Seen" "Archive";`
		testExecute(ctx, t, script, eml, false, Result{
			Fileinto:     []string{"Archive"},
			Flags:        []string{"\\seen"},
			ImplicitKeep: false,
		})
	})
	t.Run("mailboxexists-in-condition", func(t *testing.T) {
		// Use mailboxexists to conditionally file
		script := `require ["fileinto", "mailbox"]; if mailboxexists "Archive" { fileinto "Archive"; } else { fileinto :create "Archive"; }`
		testExecute(ctx, t, script, eml, false, Result{
			Fileinto:     []string{"Archive"},
			ImplicitKeep: false,
		})
	})
	t.Run("mailboxexists-not", func(t *testing.T) {
		// not mailboxexists (always false without checker, so not is true... wait)
		// Without checker, mailboxexists returns true, so not mailboxexists is false
		script := `require "mailbox"; if not mailboxexists "NonExistent" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			ImplicitKeep: true, // Without checker, mailboxexists returns true, so not is false
		})
	})
	t.Run("fileinto-multiple-create", func(t *testing.T) {
		// Multiple fileinto :create commands
		script := `require ["fileinto", "mailbox"]; fileinto :create "Folder1"; fileinto :create "Folder2";`
		testExecute(ctx, t, script, eml, false, Result{
			Fileinto:     []string{"Folder1", "Folder2"},
			ImplicitKeep: false,
		})
	})
}

// Email message with subaddress (user+detail@domain)
var emlWithSubaddress string = `Date: Tue, 1 Apr 1997 09:06:31 -0800 (PST)
From: ken+sieve@example.org
To: user+mailing-list@acme.example.com
Cc: admin+support@example.org
Subject: Test subaddress

Test message with subaddress
`

func TestSubaddress(t *testing.T) {
	ctx := context.Background()
	// Test message has From: coyote@desert.example.org (no subaddress)
	t.Run("address-user-no-separator", func(t *testing.T) {
		// :user extracts the user part (entire local-part if no separator)
		script := `require "subaddress"; if address :user "From" "coyote" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("address-detail-no-separator", func(t *testing.T) {
		// :detail fails to match if no separator exists in address
		script := `require "subaddress"; if address :detail "From" "" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			ImplicitKeep: true, // Should not match because no separator exists
		})
	})
	t.Run("subaddress-without-require", func(t *testing.T) {
		// :user without require should fail
		script := `if address :user "From" "coyote" { keep; }`
		testExecute(ctx, t, script, eml, true, Result{})
	})
	t.Run("envelope-user", func(t *testing.T) {
		// Test envelope :user with from@test.com
		script := `require ["envelope", "subaddress"]; if envelope :user "from" "from" { keep; }`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	// Tests with email containing subaddress (ken+sieve@example.org)
	t.Run("address-user-with-separator", func(t *testing.T) {
		// :user extracts "ken" from "ken+sieve@example.org"
		script := `require "subaddress"; if address :user "From" "ken" { keep; }`
		testExecute(ctx, t, script, emlWithSubaddress, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("address-detail-with-separator", func(t *testing.T) {
		// :detail extracts "sieve" from "ken+sieve@example.org"
		script := `require "subaddress"; if address :detail "From" "sieve" { keep; }`
		testExecute(ctx, t, script, emlWithSubaddress, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("address-detail-to-header", func(t *testing.T) {
		// :detail extracts "mailing-list" from To header
		script := `require "subaddress"; if address :detail "To" "mailing-list" { keep; }`
		testExecute(ctx, t, script, emlWithSubaddress, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("address-user-contains", func(t *testing.T) {
		// :user with :contains match type
		script := `require "subaddress"; if address :user :contains "From" "k" { keep; }`
		testExecute(ctx, t, script, emlWithSubaddress, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("address-detail-contains", func(t *testing.T) {
		// :detail with :contains match type
		script := `require "subaddress"; if address :detail :contains "From" "siev" { keep; }`
		testExecute(ctx, t, script, emlWithSubaddress, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("address-detail-matches", func(t *testing.T) {
		// :detail with :matches (wildcard)
		script := `require "subaddress"; if address :detail :matches "To" "mailing-*" { keep; }`
		testExecute(ctx, t, script, emlWithSubaddress, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("address-user-no-match", func(t *testing.T) {
		// :user that doesn't match
		script := `require "subaddress"; if address :user "From" "wrong" { keep; }`
		testExecute(ctx, t, script, emlWithSubaddress, false, Result{
			ImplicitKeep: true,
		})
	})
	t.Run("address-detail-no-match", func(t *testing.T) {
		// :detail that doesn't match
		script := `require "subaddress"; if address :detail "From" "wrong" { keep; }`
		testExecute(ctx, t, script, emlWithSubaddress, false, Result{
			ImplicitKeep: true,
		})
	})
	t.Run("address-detail-empty-string", func(t *testing.T) {
		// :detail matching empty string when detail is present but empty (user+@domain)
		// Note: For "ken+sieve@example.org", detail is "sieve", not empty
		script := `require "subaddress"; if address :detail "From" "" { keep; }`
		testExecute(ctx, t, script, emlWithSubaddress, false, Result{
			ImplicitKeep: true, // "sieve" != ""
		})
	})
	t.Run("subaddress-multiple-headers", func(t *testing.T) {
		// Test :user across multiple headers (From, Cc both have subaddresses)
		script := `require "subaddress"; if address :user ["From", "Cc"] "admin" { keep; }`
		testExecute(ctx, t, script, emlWithSubaddress, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("subaddress-detail-multiple-values", func(t *testing.T) {
		// Test :detail with multiple possible values
		script := `require "subaddress"; if address :detail "From" ["other", "sieve", "more"] { keep; }`
		testExecute(ctx, t, script, emlWithSubaddress, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("subaddress-with-fileinto", func(t *testing.T) {
		// Practical example: file based on subaddress detail
		script := `require ["subaddress", "fileinto"]; if address :detail "To" "mailing-list" { fileinto "lists"; }`
		testExecute(ctx, t, script, emlWithSubaddress, false, Result{
			Fileinto:     []string{"lists"},
			ImplicitKeep: false,
		})
	})
	t.Run("subaddress-with-variables", func(t *testing.T) {
		// Capture detail using :matches and use in fileinto
		script := `require ["subaddress", "fileinto", "mailbox", "variables"]; 
		if address :detail :matches "To" "*" { 
			set :lower "folder" "${1}"; 
			fileinto :create "${folder}"; 
		}`
		testExecute(ctx, t, script, emlWithSubaddress, false, Result{
			Fileinto:     []string{"mailing-list"},
			ImplicitKeep: false,
		})
	})
	t.Run("detail-without-require-error", func(t *testing.T) {
		// :detail without require should fail
		script := `if address :detail "From" "sieve" { keep; }`
		testExecute(ctx, t, script, emlWithSubaddress, true, Result{})
	})
	t.Run("address-user-case-insensitive", func(t *testing.T) {
		// :user comparison should be case-insensitive by default
		script := `require "subaddress"; if address :user "From" "KEN" { keep; }`
		testExecute(ctx, t, script, emlWithSubaddress, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
	t.Run("address-detail-case-insensitive", func(t *testing.T) {
		// :detail comparison should be case-insensitive by default
		script := `require "subaddress"; if address :detail "From" "SIEVE" { keep; }`
		testExecute(ctx, t, script, emlWithSubaddress, false, Result{
			Keep:         true,
			ImplicitKeep: true,
		})
	})
}

func TestFlags(t *testing.T) {
	ctx := context.Background()
	t.Run("set-add-remove", func(t *testing.T) {
		script := `require ["fileinto", "imap4flags"]; setflag ["flag1", "flag2"]; addflag ["flag2", "flag3"]; removeflag ["flag1"]; fileinto "test";`
		testExecute(ctx, t, script, eml, false, Result{
			Fileinto:     []string{"test"},
			Flags:        []string{"flag2", "flag3"},
			ImplicitKeep: false,
		})
	})
	t.Run("add-remove", func(t *testing.T) {
		script := `require ["fileinto", "imap4flags"]; addflag ["flag2", "flag3"]; removeflag ["flag3", "flag4"]; fileinto "test";`
		testExecute(ctx, t, script, eml, false, Result{
			Fileinto:     []string{"test"},
			Flags:        []string{"flag2"},
			ImplicitKeep: false,
		})
	})
	t.Run("case-insensitivity", func(t *testing.T) {
		script := `require "imap4flags"; setflag "Seen"; addflag "FLAGGED"; removeflag "seen"; keep;`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			Flags:        []string{"flagged"},
			ImplicitKeep: true, // keep does NOT cancel implicit keep
		})
	})
	t.Run("keep-with-flags", func(t *testing.T) {
		script := `require "imap4flags"; keep :flags ["\\Answered", "MyFlag"];`
		testExecute(ctx, t, script, eml, false, Result{
			Keep:         true,
			Flags:        []string{"\\answered", "myflag"},
			ImplicitKeep: true, // keep does NOT cancel implicit keep
		})
	})
}
