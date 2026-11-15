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
		// A zone left to net/mail as written still gets the year read by section 4.3.
		{"Mon, 17 Jul 50 02:23:25 GMT+0000", time.Date(1950, 7, 17, 2, 23, 25, 0, time.UTC), 0},
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
		// Replacing only the alphabetic prefix would let net/mail discard the
		// rest after the numeric replacement, turning these errors into dates.
		"Wed, 17 Jul 1996 02:23:25 PDT+0400",
		"Wed, 17 Jul 1996 02:23:25 PDT0",
		"Wed, 17 Jul 1996 02:23:25 PDT-0700",
		"Wed, 17 Jul 1996 02:23:25\r PDT",
		"Wed, 17 Jul 1996 02:23:25\n PDT",
		// year = 4*DIGIT / obs-year, and obs-year is 2*DIGIT (RFC 5322
		// sections 3.3, 4.3).
		"Mon, 17 Jul 5 02:23:25 +0000",
		"Mon, 17 Jul 50 02:23:25 PDT+0400",
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
