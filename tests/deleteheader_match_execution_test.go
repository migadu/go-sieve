package tests

import (
	"context"
	"errors"
	"net/textproto"
	"strings"
	"testing"
	"time"

	sieve "github.com/migadu/go-sieve"
	"github.com/migadu/go-sieve/interp"
)

// TestDeleteHeaderMatchExecution covers runtime error propagation, deadline
// enforcement, and statement-entry variable expansion in deleteheader value
// matching:
//   - A runtime :regex pattern from a variable that fails to compile or exceeds
//     MaxProgramSize must fail script execution rather than being ignored.
//   - A match stopped by MaxExecTime or context cancellation/deadline must
//     abort execution immediately rather than continuing across remaining
//     headers and subsequent actions.
//   - Value-patterns containing match variables (such as ${1}) must use the
//     values current when control reaches deleteheader (RFC 5229 §3), expanded
//     once, without earlier matching headers clobbering patterns for later
//     headers.
func TestDeleteHeaderMatchExecution(t *testing.T) {
	t.Run("runtime invalid and oversized regex keys fail execution", func(t *testing.T) {
		tooLarge := "(?:" + strings.Repeat("a", 990) + "){1000}"
		tooLong := strings.Repeat("a", interp.DefaultRegexLimits.MaxPatternLength+1)

		cases := []struct {
			name    string
			script  string
			pattern string
			wantErr string
		}{
			{
				name:    "invalid regex without index",
				script:  `require ["editheader", "variables", "regex", "fileinto"]; deleteheader :regex "X-Foo" "${p}"; fileinto "after";`,
				pattern: "(",
				wantErr: "regex compile error",
			},
			{
				name:    "invalid regex with index",
				script:  `require ["editheader", "variables", "regex", "fileinto"]; deleteheader :index 1 :regex "X-Foo" "${p}"; fileinto "after";`,
				pattern: "(",
				wantErr: "regex compile error",
			},
			{
				name:    "program too complex",
				script:  `require ["editheader", "variables", "regex", "fileinto"]; deleteheader :regex "X-Foo" "${p}"; fileinto "after";`,
				pattern: tooLarge,
				wantErr: "too complex",
			},
			{
				name:    "pattern too long",
				script:  `require ["editheader", "variables", "regex", "fileinto"]; deleteheader :regex "X-Foo" "${p}"; fileinto "after";`,
				pattern: tooLong,
				wantErr: "too long",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				loaded, err := loadWithRegexKeyExtensions(tc.script)
				if err != nil {
					t.Fatalf("load: %v", err)
				}
				hdr := textproto.MIMEHeader{"X-Foo": {"first", "second"}}
				data := sieve.NewRuntimeData(loaded, interp.DummyPolicy{}, interp.EnvelopeStatic{}, interp.MessageStatic{Header: hdr})
				data.Variables["p"] = tc.pattern
				err = loaded.Execute(context.Background(), data)
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Execute err = %v, want error containing %q (mailboxes=%v)", err, tc.wantErr, data.Mailboxes)
				}
				if len(data.Mailboxes) != 0 {
					t.Fatalf("subsequent actions ran after runtime regex error: mailboxes=%v", data.Mailboxes)
				}
			})
		}
	})

	t.Run("MaxExecTime and context cancellation stop deleteheader and abort script", func(t *testing.T) {
		opts := sieve.DefaultOptions()
		opts.EnabledExtensions = []string{"editheader", "regex", "fileinto"}
		opts.Interp.RegexLimits = interp.RegexLimits{MaxExecTime: 25 * time.Millisecond}

		loaded, err := sieve.Load(strings.NewReader(`
require ["editheader", "regex", "fileinto"];
deleteheader :regex "X-Big" ["x.{0,1000}y", "x.{0,1000}z", "x.{0,1000}w"];
fileinto "after-timeout";
`), opts)
		if err != nil {
			t.Fatalf("load: %v", err)
		}

		big := strings.Repeat("x", interp.DefaultRegexLimits.MaxInputLength)
		hdr := textproto.MIMEHeader{"X-Big": {big, big, big}}

		start := time.Now()
		data := sieve.NewRuntimeData(loaded, interp.DummyPolicy{}, interp.EnvelopeStatic{}, interp.MessageStatic{Header: hdr})
		err = loaded.Execute(context.Background(), data)
		elapsed := time.Since(start)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("MaxExecTime: Execute err = %v, want context.DeadlineExceeded (mailboxes=%v)", err, data.Mailboxes)
		}
		if elapsed > 500*time.Millisecond {
			t.Fatalf("MaxExecTime: Execute took %v, want prompt stop on first timed-out match", elapsed)
		}
		if len(data.Mailboxes) != 0 {
			t.Fatalf("MaxExecTime: subsequent actions executed after timeout: %v", data.Mailboxes)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		dataCanceled := sieve.NewRuntimeData(loaded, interp.DummyPolicy{}, interp.EnvelopeStatic{}, interp.MessageStatic{Header: hdr})
		err = loaded.Execute(ctx, dataCanceled)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled ctx: Execute err = %v, want context.Canceled (mailboxes=%v)", err, dataCanceled.Mailboxes)
		}
		if len(dataCanceled.Mailboxes) != 0 {
			t.Fatalf("canceled ctx: subsequent actions executed: %v", dataCanceled.Mailboxes)
		}
	})

	t.Run("value-patterns keep statement-entry match variables across headers and expand once", func(t *testing.T) {
		RunDovecotTestInline(t, "", `
require ["vnd.dovecot.testsuite", "editheader", "variables", "regex", "relational", "comparator-i;ascii-numeric"];

test_set "message" text:
Subject: drop:target
X-Tag: prefix-first
X-Tag: keep
X-Tag: target
X-Literal: prefix-one
X-Literal: ${secret}
X-Literal: SECRET

Body.
.
;

test "earlier header match does not clobber ${1} in deleteheader value-patterns" {
	if header :matches "Subject" "drop:*" {
		deleteheader :matches "X-Tag" ["prefix-*", "${1}"];
	}
	if not header :count "eq" :comparator "i;ascii-numeric" "X-Tag" "1" {
		test_fail "expected only 'keep' X-Tag to remain";
	}
	if not header :is "X-Tag" "keep" {
		test_fail "wrong X-Tag field remained";
	}
}

test "expanded match variable holding ${...} text is not expanded a second time" {
	set "secret" "SECRET";
	set "dollar" "$";
	if string :matches "${dollar}{secret}" "*" {
		deleteheader :matches "X-Literal" ["prefix-*", "${1}"];
	}
	if header :is "X-Literal" "${dollar}{secret}" {
		test_fail "literal ${secret} header was not deleted";
	}
	if not header :is "X-Literal" "SECRET" {
		test_fail "value of variable secret was falsely deleted by second expansion";
	}
}
`)
	})
}
