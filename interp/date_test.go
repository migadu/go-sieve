package interp

import (
	"net/mail"
	"testing"
	"time"
	_ "time/tzdata" // the zones the test runs parseDateHeader in, on hosts without zoneinfo
)

// An alphabetic zone has the offset RFC 5322 section 4.3 gives it ("PDT is
// semantically equivalent to -0700"), and a military or unknown zone "-0000",
// in whatever zone the host runs. net/mail.ParseDate gives PDT +0000 on a host
// in UTC, EST in July -0400 on one in America/New_York and CET +0100 on one in
// Europe/Berlin, and does not parse a lower-case or military zone at all.
func TestParseDateHeaderObsoleteZone(t *testing.T) {
	const h = 3600
	cases := []struct {
		value  string
		wall   time.Time // the date and time as written, in UTC
		offset int       // seconds east of UTC
	}{
		{"Wed, 17 Jul 1996 02:23:25 UT", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), 0},
		{"Wed, 17 Jul 1996 02:23:25 GMT", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), 0},
		{"Wed, 17 Jul 1996 02:23:25 EDT", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -4 * h},
		{"Wed, 17 Jul 1996 02:23:25 EST", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -5 * h},
		{"Wed, 17 Jul 1996 02:23:25 CDT", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -5 * h},
		{"Wed, 17 Jan 1996 02:23:25 CST", time.Date(1996, 1, 17, 2, 23, 25, 0, time.UTC), -6 * h},
		{"Wed, 17 Jul 1996 02:23:25 MDT", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -6 * h},
		{"Wed, 17 Jan 1996 02:23:25 MST", time.Date(1996, 1, 17, 2, 23, 25, 0, time.UTC), -7 * h},
		{"Wed, 17 Jul 1996 02:23:25 PDT", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -7 * h},
		{"Wed, 17 Jan 1996 02:23:25 PST", time.Date(1996, 1, 17, 2, 23, 25, 0, time.UTC), -8 * h},
		{"Wed, 17 Jul 1996 02:23:25 pdt", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -7 * h},
		{"Wed, 17 Jul 1996 02:23:25 Gmt", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), 0},
		{"Wed, 17 Jul 1996 02:23:25 PDT (Pacific Daylight Time)", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -7 * h},
		{"Wed, 17 Jul 1996 02:23:25\r\n PDT", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -7 * h},
		{"Wed, 17 Jul 1996 02:23:25\r\n\tPdT", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -7 * h},
		{"Wed, 17 Jul 1996 02:23:25PDT", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -7 * h},
		{"Wed, 17 Jul 1996 02:23:25 PDT(comment)", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -7 * h},
		{"17 Jul 96 02:23 PDT", time.Date(1996, 7, 17, 2, 23, 0, 0, time.UTC), -7 * h},
		// Military zones and other names: "-0000".
		{"Wed, 17 Jul 1996 02:23:25 Z", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), 0},
		{"Wed, 17 Jul 1996 02:23:25 z", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), 0},
		{"Wed, 17 Jul 1996 02:23:25 A", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), 0},
		{"Wed, 17 Jul 1996 02:23:25 Y", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), 0},
		{"Wed, 17 Jan 1996 02:23:25 CET", time.Date(1996, 1, 17, 2, 23, 25, 0, time.UTC), 0},
		{"Wed, 17 Jul 1996 02:23:25 CEST", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), 0},
		{"Wed, 17 Jul 1996 02:23:25 MSK", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), 0},
		// Numeric zones are net/mail's, unchanged.
		{"Wed, 17 Jul 1996 02:23:25 -0700 (PDT)", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -7 * h},
		{"Wed, 17 Jul 1996 02:23:25 +0530", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), 5*h + 30*60},
	}

	saved := time.Local
	t.Cleanup(func() { time.Local = saved })
	for _, name := range []string{"UTC", "America/Los_Angeles", "America/New_York", "Europe/Berlin", "Asia/Kolkata"} {
		loc, err := time.LoadLocation(name)
		if err != nil {
			t.Fatal(err)
		}
		time.Local = loc // no test in this package runs in parallel
		for _, tc := range cases {
			got, err := parseDateHeader(tc.value)
			if err != nil {
				t.Errorf("TZ=%s %q: %v", name, tc.value, err)
				continue
			}
			want := tc.wall.Add(-time.Duration(tc.offset) * time.Second)
			if !got.Equal(want) {
				t.Errorf("TZ=%s %q: %v, want %v", name, tc.value, got, want.In(time.FixedZone("", tc.offset)))
			}
			if _, offset := got.Zone(); offset != tc.offset {
				t.Errorf("TZ=%s %q: zone offset of %v is %d, want %d", name, tc.value, got, offset, tc.offset)
			}
		}
	}
}

// Comments and white space between the tokens of a date-time are read as
// separators. RFC 5322 section 4.3: the obsolete syntax "permits comments and
// folding white space between many of the tokens" (obs-day-of-week = [CFWS]
// day-name [CFWS], obs-hour = [CFWS] 2DIGIT [CFWS], ...), and section 4: such
// forms "MUST be accepted and parsed by a conformant receiver".
// net/mail.ParseDate takes a comment only after the zone, and white space only
// where its layouts have one space.
func TestParseDateHeaderCFWS(t *testing.T) {
	const h = 3600
	wall := time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC)
	cases := []struct {
		value  string
		offset int // seconds east of UTC
	}{
		{"Wed, 17 Jul 1996 02:23:25 (c) -0700", -7 * h},
		{"Wed , 17 Jul 1996 02:23:25 -0700", -7 * h},
		{"Wed, 17 Jul 1996 02 : 23 : 25 -0700", -7 * h},
		{"(Wed) 17 Jul 1996 02:23:25 -0700", -7 * h},
		{"Wed, 17 (July) Jul 1996 02:23:25 -0700", -7 * h},
		{"Wed, 17 Jul 1996 02:23:25 (Pacific) PDT", -7 * h},
		{"Wed (Wednesday) , (the) 17 (th) Jul (of) 1996 (at) 02 (h) : (m) 23 (s) : 25 (z) -0700 (PDT)", -7 * h},
		{"Wed, 17 Jul 1996 02(h):(m)23(s):25 -0700", -7 * h},
		{"Wed, 17 Jul 1996 02:23:25 PDT(Pacific)(Daylight Time)", -7 * h},
		// Nested comments and quoted-pairs (RFC 5322 section 3.2.2).
		{"Wed, 17 Jul 1996 02:23:25 (a (nested) \\) comment) -0700", -7 * h},
		{"Wed, 17 Jul 1996 02:23:25 (\\(c) PDT", -7 * h},
		// A comment holding the characters the zone and the time are found by.
		{"Wed, 17 Jul 1996 02:23:25 (UTC-7) PDT", -7 * h},
		{"(Wed: 17 Jul) Wed, 17 Jul 1996 02:23:25 PDT", -7 * h},
		// Folding white space: tabs, runs, a fold around a comment.
		{"Wed,\t17\tJul\t1996\t02:23:25\t-0700", -7 * h},
		{"Wed,   17  Jul   1996  02:23:25    PDT", -7 * h},
		{"Wed, 17 Jul 1996 02:23:25\r\n (c)\r\n\t-0700", -7 * h},
		// day = ([FWS] 1*2DIGIT FWS): the white space after the comma is optional.
		{"Wed,17 Jul 1996 02:23:25 -0700", -7 * h},
		{"Wed,(c)17 Jul 1996 02:23:25 +0530", 5*h + 30*60},
		// obs-year = [CFWS] 2*DIGIT [CFWS].
		{"Wed, 17 Jul (y) 96 (t) 02:23:25 PDT", -7 * h},
		// A zone left to net/mail as written (an extension it accepts).
		{"Wed , 17 Jul 1996 02:23:25 (c) GMT+0000", 0},
	}
	for _, tc := range cases {
		got, err := parseDateHeader(tc.value)
		if err != nil {
			t.Errorf("%q: %v", tc.value, err)
			continue
		}
		want := wall.Add(-time.Duration(tc.offset) * time.Second)
		if !got.Equal(want) {
			t.Errorf("%q: %v, want %v", tc.value, got, want.In(time.FixedZone("", tc.offset)))
		}
		if _, offset := got.Zone(); offset != tc.offset {
			t.Errorf("%q: zone offset of %v is %d, want %d", tc.value, got, offset, tc.offset)
		}
	}
}

// A value that is not a date-time is still an error.
func TestParseDateHeaderInvalid(t *testing.T) {
	for _, value := range []string{
		"", "PDT", "Wed, 17 Jul 1996 PDT", "not a date 12:00 PDT",
		// Replacing only the alphabetic prefix would let net/mail discard the
		// rest after the numeric replacement, turning these errors into dates.
		"Wed, 17 Jul 1996 02:23:25 PDT+0400",
		"Wed, 17 Jul 1996 02:23:25 PDT0",
		"Wed, 17 Jul 1996 02:23:25 PDT-0700",
		"Wed, 17 Jul 1996 02:23:25\r PDT",
		"Wed, 17 Jul 1996 02:23:25\n PDT",
		// A comment that does not close, and a zone that is only in a comment.
		"Wed, 17 Jul 1996 02:23:25 -0700 (PDT",
		"Wed, 17 Jul 1996 02:23:25 PDT (Pacific",
		"Wed, 17 Jul 1996 02:23:25 (c -0700",
		"Wed, 17 Jul 1996 02:23:25 -0700 (c\\)",
		"Wed, 17 Jul 1996 02:23:25 (-0700)",
		"Wed, 17 Jul 1996 02:23:25 (PDT)",
		// Nor are letters a zone when a numeric offset follows them after
		// white space or a comment: the RFC 5322 section 3.3 grammar has one
		// zone after the time of day ("time = time-of-day zone"), and the
		// offset after the letters would be discarded.
		"Wed, 17 Jul 1996 02:23:25 PM -0700",
		"Wed, 17 Jul 1996 02:23:25 PDT +0500",
		"Wed, 17 Jul 1996 02:23:25 PDT\r\n +0500",
		"Wed, 17 Jul 1996 02:23:25 Z\t-0700",
		"Wed, 17 Jul 1996 02:23:25 PDT (c) +0500",
		"Wed, 17 Jul 1996 02:23:25 (c) PM (c) -0700",
	} {
		if got, err := parseDateHeader(value); err == nil {
			t.Errorf("%q parsed as %v", value, got)
		}
	}
}

// An alphabetic prefix that is not a whole obs-zone keeps net/mail's reading,
// including its accepted GMT extension and permissive suffix handling.
func TestParseDateHeaderPreservesParserExtensions(t *testing.T) {
	for _, value := range []string{
		"Wed, 17 Jul 1996 02:23:25 GMT+0000",
		"Wed, 17 Jul 1996 02:23:25 PDT/UTC",
	} {
		want, err := mail.ParseDate(value)
		if err != nil {
			t.Fatalf("net/mail: %q: %v", value, err)
		}
		got, err := parseDateHeader(value)
		if err != nil {
			t.Fatalf("%q: %v", value, err)
		}
		_, wantOffset := want.Zone()
		_, gotOffset := got.Zone()
		if !got.Equal(want) || gotOffset != wantOffset {
			t.Errorf("%q: %v, want %v", value, got, want)
		}
	}
}

// The iso8601 date-part writes a zero offset as "Z", whatever zone has it (RFC
// 5260 §4.2: "a time zone offset of zero MUST be represented by "Z" and not
// "+00:00""), and any other offset as "+hh:mm" or "-hh:mm" (RFC 3339 §5.6).
func TestExtractDatePartISO8601(t *testing.T) {
	london, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Fatal(err)
	}
	utc := time.Date(1996, 1, 17, 2, 23, 25, 0, time.UTC)
	for _, tc := range []struct {
		loc  *time.Location
		want string
	}{
		{time.UTC, "1996-01-17T02:23:25Z"},
		{time.FixedZone("", 0), "1996-01-17T02:23:25Z"},
		{time.FixedZone("GMT", 0), "1996-01-17T02:23:25Z"},
		{london, "1996-01-17T02:23:25Z"}, // GMT in January
		{time.FixedZone("", 2*3600), "1996-01-17T04:23:25+02:00"},
		{time.FixedZone("", -(5*3600 + 30*60)), "1996-01-16T20:53:25-05:30"},
	} {
		got, err := extractDatePart(utc.In(tc.loc), DatePartISO8601)
		if err != nil {
			t.Fatalf("%v: %v", tc.loc, err)
		}
		if got != tc.want {
			t.Errorf("%v: iso8601 = %q, want %q", tc.loc, got, tc.want)
		}
	}
}
