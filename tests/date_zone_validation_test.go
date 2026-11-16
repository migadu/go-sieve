package tests

import (
	"context"
	"net/textproto"
	"strings"
	"testing"
	"time"

	sieve "github.com/migadu/go-sieve"
	"github.com/migadu/go-sieve/interp"
)

// A malformed zone is not a date for any date-part, including :count. Neither
// rewriting it nor a fallback layout may manufacture a UTC date from it.
func TestDateRejectsMalformedZoneTokens(t *testing.T) {
	opts := sieve.DefaultOptions()
	opts.EnabledExtensions = []string{"date", "fileinto", "relational", "comparator-i;ascii-numeric"}
	script, err := sieve.Load(strings.NewReader(`
require ["date", "fileinto", "relational", "comparator-i;ascii-numeric"];
if date :count "eq" :comparator "i;ascii-numeric" "X-Date" "year" "0" { fileinto "zero"; }
if date :originalzone "X-Date" "hour" "12" { fileinto "wrong-hour"; }
if date :originalzone "X-Date" "zone" "+0000" { fileinto "wrong-zone"; }
if date :zone "+0000" "X-Date" "year" "2019" { fileinto "wrong-year"; }
`), opts)
	if err != nil {
		t.Fatal(err)
	}
	zones := []string{
		"GMT+1", "GMT-00", "GMT+0000", "UTC-0500", "PDT+0400", "PDT/UTC", "CET/CEST", "EST5EDT", "PDT0",
		"+1", "-03", "+001", "+000", "+01:00", "+00000", "+01000", "+0060", "+2360", "+0000junk", "+0000GMT+1", "+0a00", "0000", "--0000",
		"GMT junk", "GMT (c) junk", "G(c)MT", "GMT (c) +0100", "+0000 junk", "+0000 (c) junk", "+0000 (c) +0100", "+00(c)00",
		"GMT\x00", "GMT\x01", "GMT\vX", "GMT\fX", "GMT\x7f", "GMTé", "éGMT", "GMT\\+1", "\"GMT\"", "[GMT]",
		"GMT\v", "GMT\f", "GMT\u0085", "GMT\u00a0", "GMT\rX", "GMT\nX",
		"GMT(c)\v", "GMT(c)\u00a0",
		"+0000\x00", "+0000\vX", "+0000\x7f", "+0000é", "+0000\v", "+0000\n", "+0000\r", "GMT (unclosed", "+0000 (bad\x00)",
	}
	values := make([]string, 0, len(zones)+10)
	for _, zone := range zones {
		values = append(values, "Tue, 01 Jan 2019 12:00:00 "+zone)
	}
	values = append(values,
		"2019-01-01T12:00:00+01:60", "2019-01-01T12:00:00+24:00", "2019-01-01T12:00:00Z\v",
		"Tue, 01 Jan 2019 12:00:00.125 GMT+1", "Tue, 01 Jan 2019 12:00:00.125 +0060")
	saved := time.Local
	t.Cleanup(func() { time.Local = saved })
	for _, location := range []*time.Location{time.UTC, time.FixedZone("CET", 3600), time.FixedZone("+01", 3600)} {
		time.Local = location
		for _, date := range values {
			for _, prefix := range []string{"", "from mail.example by mx.example; "} {
				value := prefix + date
				t.Run(location.String()+"/"+value, func(t *testing.T) {
					d := sieve.NewRuntimeData(script, interp.DummyPolicy{}, interp.EnvelopeStatic{}, interp.MessageStatic{
						Header: textproto.MIMEHeader{"X-Date": {value}},
					})
					if err := script.Execute(context.Background(), d); err != nil {
						t.Fatal(err)
					}
					if got := strings.Join(d.Mailboxes, ","); got != "zero" {
						t.Fatalf("filed into %q, want only zero for an invalid date", got)
					}
				})
			}
		}
	}
}

func TestDateZoneValidationPreservesAcceptedDates(t *testing.T) {
	opts := sieve.DefaultOptions()
	opts.EnabledExtensions = []string{"date", "fileinto", "relational", "comparator-i;ascii-numeric"}
	script, err := sieve.Load(strings.NewReader(`
require ["date", "fileinto", "relational", "comparator-i;ascii-numeric"];
if date :count "eq" :comparator "i;ascii-numeric" "X-Date" "year" "1" { fileinto "one"; }
if date :originalzone "X-Date" "iso8601" "2016-12-31T23:59:60Z" { fileinto "leap"; }
if date :originalzone "X-Date" "iso8601" "2019-01-01T12:00:00Z" { fileinto "utc"; }
if date :originalzone "X-Date" "iso8601" "2019-01-01T12:00:00-07:00" { fileinto "pdt"; }
if date :originalzone "X-Date" "iso8601" "2019-01-01T12:00:00+01:30" { fileinto "numeric"; }
if date :originalzone "X-Date" "iso8601" "2019-01-01T12:00:00-05:00" { fileinto "negative"; }
`), opts)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ value, want string }{
		{"Tue, 01 Jan 2019 12:00:00 +0000", "one,utc"},
		{"\tTue, 01 Jan 2019 12:00:00 GMT \t", "one,utc"},
		{"Tue, 01 Jan 2019 12:00:00 GMT\r\n", "one,utc"},
		{"Tue, 01 Jan 2019 12:00:00 GMT\r\n \t", "one,utc"},
		{"Tue, 01 Jan 2019 12:00:00 +0000\r\n\t(last hop)", "one,utc"},
		{"Tue, 01 Jan 2019 12:00:00 -0000 (outer (inner;))", "one,utc"},
		{"Tue, 01 Jan 2019 12:00:00 GMT (quoted\\\x00 and obsolete\vcomment)", "one,utc"},
		{"Tue, 01 Jan 2019 12:00:00 +0130", "one,numeric"},
		{"Tue, 01 Jan 2019 12:00:00 -0500", "one,negative"},
		{"Tue, 01 Jan 2019 12:00:00 pDt", "one,pdt"},
		{"Tue, 01 Jan 2019 12:00:00 CET", "one,utc"},
		{"Tue, 01 Jan 2019 12:00:00 XYZ", "one,utc"},
		{"Tue, 01 Jan 2019 12:00:00Z", "one,utc"},
		{"Sat, 31 Dec (year) 116 23 (h):59 (m):60 (s) GMT (last; hop)", "one,leap"},
		{"2019-01-01T12:00:00Z", "one,utc"},
		{"2019-01-01T12:00:00+01:30", "one,numeric"},
		{"Tue, 01 Jan 2019 12:00:00.125 +0000", "one,utc"},
		{"01 Jan 2019 12:00:00.125 XYZ", "one,utc"},
		{"01 Jan 2019 12:00:00,125 XYZ", "one,utc"},
		{"01 Jan 19 12:00 +0000", "one,utc"},
	}
	saved := time.Local
	t.Cleanup(func() { time.Local = saved })
	for _, location := range []*time.Location{time.UTC, time.FixedZone("CET", 3600), time.FixedZone("XYZ", 7200)} {
		time.Local = location
		for _, tc := range cases {
			for _, prefix := range []string{"", "from [a(] by mx (last; hop); "} {
				t.Run(location.String()+"/"+prefix+tc.value, func(t *testing.T) {
					d := sieve.NewRuntimeData(script, interp.DummyPolicy{}, interp.EnvelopeStatic{}, interp.MessageStatic{
						Header: textproto.MIMEHeader{"X-Date": {prefix + tc.value}},
					})
					if err := script.Execute(context.Background(), d); err != nil {
						t.Fatal(err)
					}
					if got := strings.Join(d.Mailboxes, ","); got != tc.want {
						t.Fatalf("filed into %q, want %q", got, tc.want)
					}
				})
			}
		}
	}
}
