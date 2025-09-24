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
