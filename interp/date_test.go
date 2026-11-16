package interp

import (
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
		// Numeric minutes are bounded by 59, but RFC 5322 does not bound the
		// two hour digits. Preserve offsets older Go versions already accept.
		{"Wed, 17 Jul 1996 02:23:25 -0700 (PDT)", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -7 * h},
		{"Wed, 17 Jul 1996 02:23:25 +0530", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), 5*h + 30*60},
		{"Wed, 17 Jul 1996 02:23:25 +2400", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), 24 * h},
		{"Wed, 17 Jul 1996 02:23:25 -2459", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -(24*h + 59*60)},
		{"Wed, 17 Jul 1996 02:23:25 +2500", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), 25 * h},
		{"Wed, 17 Jul 1996 02:23:25 -9959", time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -(99*h + 59*60)},
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
		// Obsolete quoted-pair permits escaped controls; unquoted NUL is invalid.
		{"Wed, 17 Jul 1996 02:23:25 (quoted\\\x00) -0700", -7 * h},
		{"Wed, 17 Jul 1996 02:23:25 -0700 (quoted\\\nLF)", -7 * h},
		{"Wed, 17 Jul 1996 02:23:25 -0700 (quoted\\\rCR)", -7 * h},
		// Folding is removed before quoted-pair interpretation (section 2.2.3).
		{"Wed, 17 Jul 1996 02:23:25 -0700 (foldpair\\\r\n space)", -7 * h},
		{"Wed, 17 Jul 1996 02:23:25 -0700 (foldpair\\\r\n\ttab)", -7 * h},
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
		// An unknown alphabetic name has offset -0000.
		{"Wed , 17 Jul 1996 02:23:25 (c) XYZ", 0},
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

// A two- or three-digit year (obs-year) is read by RFC 5322 section 4.3: "If a
// two digit year is encountered whose value is between 00 and 49, the year is
// interpreted by adding 2000 ... If a two digit year is encountered with a
// value between 50 and 99, or any three digit year is encountered, the year is
// interpreted by adding 1900." net/mail.ParseDate reads a two-digit year with
// time.Parse's "06", which puts 50-68 in 20xx, and does not parse a
// three-digit year at all.
func TestParseDateHeaderObsoleteYear(t *testing.T) {
	const h = 3600
	cases := []struct {
		value  string
		wall   time.Time // the date and time as written, in UTC
		offset int       // seconds east of UTC
	}{
		{"Sat, 1 Jan 00 00:00:00 +0000", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), 0},
		{"Sat, 17 Jul 49 02:23:25 +0000", time.Date(2049, 7, 17, 2, 23, 25, 0, time.UTC), 0},
		{"Mon, 17 Jul 50 02:23:25 +0000", time.Date(1950, 7, 17, 2, 23, 25, 0, time.UTC), 0},
		{"Wed, 17 Jul 68 02:23:25 +0000", time.Date(1968, 7, 17, 2, 23, 25, 0, time.UTC), 0},
		{"Thu, 17 Jul 69 02:23:25 +0000", time.Date(1969, 7, 17, 2, 23, 25, 0, time.UTC), 0},
		{"Sat, 17 Jul 99 02:23:25 +0000", time.Date(1999, 7, 17, 2, 23, 25, 0, time.UTC), 0},
		{"17 Jul 50 02:23 PDT", time.Date(1950, 7, 17, 2, 23, 0, 0, time.UTC), -7 * h},
		{"Mon, 17 Jul  50  02:23:25 +0000", time.Date(1950, 7, 17, 2, 23, 25, 0, time.UTC), 0},
		{"Mon, 17 Jul\r\n 50\r\n 02:23:25 +0000", time.Date(1950, 7, 17, 2, 23, 25, 0, time.UTC), 0},
		// Three digits: years since 1900.
		{"Mon, 3 Jan 100 10:00:00 +0000", time.Date(2000, 1, 3, 10, 0, 0, 0, time.UTC), 0},
		{"Tue, 17 Nov 126 09:00:00 -0500", time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC), -5 * h},
		{"Sat, 1 Jan 049 00:00:00 +0000", time.Date(1949, 1, 1, 0, 0, 0, 0, time.UTC), 0},
		{"Mon, 1 Jan 000 00:00:00 +0000", time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC), 0},
		{"Thu, 31 Dec 999 23:59:59 +0000", time.Date(2899, 12, 31, 23, 59, 59, 0, time.UTC), 0},
		{"Mon, 3 Jan 100 10:00:00 EST", time.Date(2000, 1, 3, 10, 0, 0, 0, time.UTC), -5 * h},
		// An unknown alphabetic name still gets the year read by section 4.3.
		{"Mon, 17 Jul 50 02:23:25 XYZ", time.Date(1950, 7, 17, 2, 23, 25, 0, time.UTC), 0},
		// Four digits are taken as written.
		{"Sun, 17 Jul 2050 02:23:25 +0000", time.Date(2050, 7, 17, 2, 23, 25, 0, time.UTC), 0},
		{"Wed, 17 Jul 1968 02:23:25 +0000", time.Date(1968, 7, 17, 2, 23, 25, 0, time.UTC), 0},
	}
	for _, tc := range cases {
		got, err := parseDateHeader(tc.value)
		if err != nil {
			t.Errorf("%q: %v", tc.value, err)
			continue
		}
		want := tc.wall.Add(-time.Duration(tc.offset) * time.Second)
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
		// An unpaired CR or LF is not folding white space.
		"Wed, 17 Jul 1996 02:23:25\r PDT",
		"Wed, 17 Jul 1996 02:23:25\n PDT",
		// NUL is not comment text (ctext or obs-ctext), so removing a
		// malformed comment must not turn a previously invalid value into a date.
		"Wed, 17 Jul 1996 02:23:25 (bad\x00) -0700",
		"Wed, 17 Jul 1996 02:23:25 (outer (bad\x00)) PDT",
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
		// year = 4*DIGIT / obs-year, and obs-year is 2*DIGIT (RFC 5322
		// sections 3.3, 4.3).
		"Mon, 17 Jul 5 02:23:25 +0000",
	} {
		if got, err := parseDateHeader(value); err == nil {
			t.Errorf("%q parsed as %v", value, got)
		}
	}
}

// Invalid zone syntax is not a date-time (RFC 5260 section 4), independently
// of the host's zone. Only unknown alphabetic names have the -0000 recovery
// specified by RFC 5322 section 4.3; RFC 5256's invalid-zone recovery is for SORT.
func TestParseDateHeaderMalformedZone(t *testing.T) {
	values := []string{
		"Tue, 01 Jan 2019 12:00:00 GMT+1",
		"Tue, 01 Jan 2019 12:00:00 GMT-5",
		"Tue, 01 Jan 2019 12:00:00 GMT+01",
		"Tue, 01 Jan 2019 12:00:00 GMT+23",
		"Tue, 01 Jan 2019 12:00:00 GMT+0001",
		"Tue, 01 Jan 2019 12:00:00 GMT+10 (CET)",
		"Tue, 01 Jan 2019 12:00:00 GMT+1(CET)",
		"Tue, 01 Jan 2019 12:00:00 GMT+0200",
		"Tue, 01 Jan 2019 12:00:00 UTC-0500",
		"Tue, 01 Jan 2019 12:00:00 utc+0100",
		"Tue, 01 Jan 2019 12:00:00 PDT+0400",
		"Tue, 01 Jan 2019 12:00:00 PDT-0700",
		"Tue, 01 Jan 2019 12:00:00 PDT0",
		"Tue, 01 Jan 2019 12:00:00 CET/CEST",
		"Tue, 01 Jan 2019 12:00:00 PDT/UTC",
		"Tue, 01 Jan 2019 12:00:00 EST5EDT",
		"Tue, 01 Jan 2019 12:00:00 +01",
		"Tue, 01 Jan 2019 12:00:00 -03",
		"Tue, 01 Jan 2019 12:00:00 +001",
		"Tue, 01 Jan 2019 12:00:00 +4",
		"Tue, 01 Jan 2019 12:00:00 +01:00",
		"Tue, 01 Jan 2019 12:00:00\r\n +01 (CET)",
		"Tue, 01 Jan 2019 12:00:00 GMT+0",
		"Tue, 01 Jan 2019 12:00:00 GMT-00",
		"Tue, 01 Jan 2019 12:00:00 GMT+0000",
		// Year and leap-second normalization must not repair an invalid zone.
		"Mon, 17 Jul 50 02:23:25 PDT+0400",
		"Sat, 31 Dec 2016 23:59:60 GMT+1",
		"Sat, 31 Dec 2016 23:59:60 GMT+0000",
		// Exact numeric token and range, without permissive fallback layouts.
		"Tue, 01 Jan 2019 12:00:00 +0060",
		"Tue, 01 Jan 2019 12:00:00 -0060",
		"Tue, 01 Jan 2019 12:00:00 +2360",
		"Tue, 01 Jan 2019 12:00:00 +2460",
		"Tue, 01 Jan 2019 12:00:00 +00000",
		"Tue, 01 Jan 2019 12:00:00 +0000junk",
		"Tue, 01 Jan 2019 12:00:00 +0000GMT+1",
		"Tue, 01 Jan 2019 12:00:00 +0000 junk",
		"Tue, 01 Jan 2019 12:00:00 +0000 (c) +0100",
		"Tue, 01 Jan 2019 12:00:00 PDT+0400 -0700",
		"Tue, 01 Jan 2019 12:00:00 GMT junk",
		"Tue, 01 Jan 2019 12:00:00 GMT (c) junk",
		"Tue, 01 Jan 2019 12:00:00 G(c)MT",
		// Controls may not be stripped out of a malformed zone or suffix.
		"Tue, 01 Jan 2019 12:00:00 GMT\x00",
		"Tue, 01 Jan 2019 12:00:00 +0000\x00",
		"Tue, 01 Jan 2019 12:00:00 +0000\x0b",
		"Tue, 01 Jan 2019 12:00:00 GMT\x7f",
		// RFC3339 fallback must not normalize invalid offset components either.
		"2019-01-01T12:00:00+01:60",
		"2019-01-01T12:00:00+24:00",
	}

	saved := time.Local
	t.Cleanup(func() { time.Local = saved })
	for _, name := range []string{"UTC", "Africa/Casablanca", "America/Sao_Paulo", "Europe/Berlin", "America/Los_Angeles", "America/New_York"} {
		loc, err := time.LoadLocation(name)
		if err != nil {
			t.Fatal(err)
		}
		time.Local = loc // no test in this package runs in parallel
		for _, value := range values {
			if got, err := parseDateHeader(value); err == nil {
				t.Errorf("TZ=%s %q: parsed invalid date as %v", name, value, got)
			}
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

// A time-of-day whose second is 60, a leap second, is a date-time: RFC 5322
// section 3.3 has "the time-of-day MUST be in the range 00:00:00 through
// 23:59:60 (the number of seconds allowing for a leap second ...)".
// time.Time cannot hold second 60, so it is read as second 59 of the same
// minute, as Dovecot's message_date_parse does. net/mail.ParseDate does not
// parse it at all.
func TestParseDateHeaderLeapSecond(t *testing.T) {
	const h = 3600
	leap := time.Date(2016, 12, 31, 23, 59, 59, 0, time.UTC) // 2016-12-31T23:59:60Z, as second 59
	cases := []struct {
		value  string
		want   time.Time
		offset int // seconds east of UTC
	}{
		{"Sat, 31 Dec 2016 23:59:60 +0000", leap, 0},
		{"Sun, 1 Jan 2017 00:59:60 +0100", leap, h},
		{"Sat, 31 Dec 2016 15:59:60 -0800", leap, -8 * h},
		{"Sat, 31 Dec 2016 23:59:60 GMT", leap, 0},
		{"Sat, 31 Dec 2016 23:59:60 UT (leap second)", leap, 0},
		{"Sat, 31 Dec 2016 15:59:60 PST", leap, -8 * h},
		{"31 Dec 2016 23:59:60 +0000", leap, 0},
		{"Sat, 31 Dec 2016 23:59:60\r\n +0000", leap, 0},
		{"Tue, 30 Jun 2015 23:59:60 +0000", time.Date(2015, 6, 30, 23, 59, 59, 0, time.UTC), 0},
	}
	for _, tc := range cases {
		got, err := parseDateHeader(tc.value)
		if err != nil {
			t.Errorf("%q: %v", tc.value, err)
			continue
		}
		if !got.Equal(tc.want) {
			t.Errorf("%q: %v, want %v", tc.value, got, tc.want.In(time.FixedZone("", tc.offset)))
		}
		if _, offset := got.Zone(); offset != tc.offset {
			t.Errorf("%q: zone offset of %v is %d, want %d", tc.value, got, offset, tc.offset)
		}
	}

	// Only a second of 60 is a leap second.
	for _, value := range []string{
		"Sat, 31 Dec 2016 23:59:61 +0000",
		"Sat, 31 Dec 2016 23:59:600 +0000",
		"Sat, 31 Dec 2016 23:60:00 +0000",
		"Sat, 31 Dec 2016 24:00:00 +0000",
	} {
		if got, err := parseDateHeader(value); err == nil {
			t.Errorf("%q parsed as %v", value, got)
		}
	}
}

// The date-time of a Received: field is the text after its last semicolon (RFC
// 5260 §4: "Implementations MUST support extraction of RFC 2822 date-time
// information that either makes up the entire header field ... or appears at
// the end of a header field following a semicolon"), and with more than one
// date-time "the last one that appears SHOULD be used". A value that is a
// date-time as a whole is still read whole, even with a semicolon in a comment.
func TestParseDateHeaderReceived(t *testing.T) {
	const h = 3600
	cases := []struct {
		value  string
		wall   time.Time // the date and time as written, in UTC
		offset int       // seconds east of UTC
	}{
		{"from mx.example.org by mail.example.com; Wed, 17 Jul 1996 02:23:25 -0700",
			time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -7 * h},
		{"from mx.example.org (mx.example.org [192.0.2.1])\r\n\tby mail.example.com (Postfix) with ESMTPS id 4ABC\r\n\tfor <user@example.com>; Wed, 17 Jul 1996 02:23:25 -0700 (PDT)",
			time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -7 * h},
		{"from mx.example.org by mail.example.com;\r\n Wed, 17 Jul 1996 02:23:25 +0200",
			time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), 2 * h},
		{"by mail.example.com;Wed, 17 Jul 1996 02:23:25 -0700",
			time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -7 * h},
		{"from [2001:db8::1] by mail.example.com; 17 Jul 96 02:23 PDT",
			time.Date(1996, 7, 17, 2, 23, 0, 0, time.UTC), -7 * h},
		{"from mx.example.org (helo; x) by mail.example.com; Wed, 17 Jul 1996 02:23:25 -0700",
			time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -7 * h},
		// More than one date-time: the last one.
		{"by mail.example.com; Tue, 16 Jul 1996 01:00:00 +0000; Wed, 17 Jul 1996 02:23:25 -0700",
			time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -7 * h},
		// A whole date-time with a semicolon in a comment.
		{"Wed, 17 Jul 1996 02:23:25 -0700 (PDT; summer)",
			time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -7 * h},
		{"Wed, 17 Jul 1996 02:23:25 -0700 (resent; Thu, 18 Jul 1996 02:23:25 +0000)",
			time.Date(1996, 7, 17, 2, 23, 25, 0, time.UTC), -7 * h},
	}
	for _, tc := range cases {
		got, err := parseDateHeader(tc.value)
		if err != nil {
			t.Errorf("%q: %v", tc.value, err)
			continue
		}
		want := tc.wall.Add(-time.Duration(tc.offset) * time.Second)
		if !got.Equal(want) {
			t.Errorf("%q: %v, want %v", tc.value, got, want.In(time.FixedZone("", tc.offset)))
		}
		if _, offset := got.Zone(); offset != tc.offset {
			t.Errorf("%q: zone offset of %v is %d, want %d", tc.value, got, offset, tc.offset)
		}
	}

	// No date-time after the last semicolon is still an error.
	for _, value := range []string{
		"from mx.example.org by mail.example.com;",
		"from mx.example.org by mail.example.com; ",
		"from mx.example.org by mail.example.com; id 4ABC",
		"by mail.example.com; Wed, 17 Jul 1996 02:23:25 -0700; id 4ABC",
		";",
	} {
		if got, err := parseDateHeader(value); err == nil {
			t.Errorf("%q parsed as %v", value, got)
		}
	}
}

// Received's separator is outside comments and quoted strings; semicolons in
// the date-time's trailing CFWS belong to the comment, not the field syntax.
func TestParseDateHeaderReceivedSeparators(t *testing.T) {
	const date = "Wed, 17 Jul 1996 02:23:25 -0700"
	want := time.Date(1996, 7, 17, 9, 23, 25, 0, time.UTC)
	for _, value := range []string{
		"from mx by local; " + date + " (PDT; summer)",
		"from mx by local; " + date + " (PDT (a;b))",
		"from mx by local; " + date + " (PDT;)",
		`from "mx;host" by local; ` + date + " (summer; time)",
		`from "mx\";host" by local; ` + date,
		`from [a(] by local; ` + date,
		`from [a"] by local; ` + date,
		`from [a;b] by local; ` + date,
		`from [a\]("] by local; ` + date,
		`from mx (a \) ; nested (b;c)) by local; ` + date,
		`from mx by local; ` + date + ` (a \) ; b)`,
		`from mx by local; ` + date + ` (a \(; b)`,
		"by local; Tue, 16 Jul 1996 01:00:00 +0000; " + date + " (last; date)",
		date + " (resent; Thu, 18 Jul 1996 02:23:25 +0000)",
	} {
		got, err := parseDateHeader(value)
		if err != nil || !got.Equal(want) {
			t.Errorf("%q: %v, %v; want %v", value, got, err, want)
		}
	}
	for _, value := range []string{
		"from mx by local; " + date + " (unterminated; comment",
		"from mx (unterminated; " + date,
		`from "unterminated; ` + date,
		`from [unterminated; ` + date,
		"from mx by local; " + date + " (comment); ",
	} {
		if got, err := parseDateHeader(value); err == nil {
			t.Errorf("%q parsed as %v", value, got)
		}
	}
}
