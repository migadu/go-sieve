package interp

import (
	"strings"
	"testing"
	"time"
)

// RFC 5322 permits two zone hour digits and bounds only their minutes. RFC
// 5260's iso8601 part instead requires RFC 3339, whose offset hour is 00..23.
// Other date parts keep the original zone and wall clock.
func TestHeaderISO8601UnrepresentableZone(t *testing.T) {
	for _, tc := range []struct{ zone, want string }{
		{"+2500", "2018-12-31T11:00:00Z"},
		{"-2500", "2019-01-02T13:00:00Z"},
		{"+2400", "2018-12-31T12:00:00Z"},
		{"-2400", "2019-01-02T12:00:00Z"},
		{"+2359", "2019-01-01T12:00:00+23:59"},
		{"-2359", "2019-01-01T12:00:00-23:59"},
		{"+0100", "2019-01-01T12:00:00+01:00"},
		{"+0000", "2019-01-01T12:00:00Z"},
	} {
		t.Run(tc.zone, func(t *testing.T) {
			p, err := parseHeaderDate("Tue, 01 Jan 2019 12:00:00 " + tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			got, err := extractHeaderDatePart(p.Time, DatePartISO8601, p.leapSecond)
			if err != nil || got != tc.want {
				t.Fatalf("iso8601=%q err=%v, want %q", got, err, tc.want)
			}
			rendered, err := time.Parse(time.RFC3339, got)
			if err != nil || !rendered.Equal(p.Time) {
				t.Fatalf("output lost instant: %q err=%v", got, err)
			}
			zone, _ := extractHeaderDatePart(p.Time, DatePartZone, false)
			hour, _ := extractHeaderDatePart(p.Time, DatePartHour, false)
			std11, _ := extractHeaderDatePart(p.Time, DatePartStd11, false)
			if zone != tc.zone || hour != "12" || !strings.HasSuffix(std11, tc.zone) {
				t.Fatalf("non-ISO parts changed: zone=%q hour=%q std11=%q", zone, hour, std11)
			}
		})
	}
	for _, zone := range []string{"+0060", "+2360", "+2460"} {
		if _, err := parseHeaderDate("Tue, 01 Jan 2019 12:00:00 " + zone); err == nil {
			t.Errorf("invalid minute zone accepted: %s", zone)
		}
	}
}

func TestISO8601LargeZonePreservesHeaderLeapSecond(t *testing.T) {
	for _, tc := range []struct{ zone, want string }{
		{"+2500", "2016-12-31T23:59:60Z"},
		{"+2400", "2017-01-01T00:59:60Z"},
		{"+2359", "2017-01-02T00:59:60+23:59"},
		{"+0000", "2017-01-02T00:59:60Z"},
	} {
		t.Run(tc.zone, func(t *testing.T) {
			p, err := parseHeaderDate("Mon, 02 Jan 2017 00:59:60 " + tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			got, err := extractHeaderDatePart(p.Time, DatePartISO8601, p.leapSecond)
			if err != nil || got != tc.want {
				t.Fatalf("iso8601=%q err=%v, want %q", got, err, tc.want)
			}
			second, _ := extractHeaderDatePart(p.Time, DatePartSecond, p.leapSecond)
			clock, _ := extractHeaderDatePart(p.Time, DatePartTime, p.leapSecond)
			if second != "60" || clock != "00:59:60" {
				t.Fatalf("non-ISO leap parts changed: %q/%q", second, clock)
			}
		})
	}
}

// The representable currentdate path and an explicit :zone use the same common
// formatter; they retain the actual instant even when their zone is too large.
func TestDateISO8601ExplicitLargeZone(t *testing.T) {
	rd := &RuntimeData{Script: &Script{}}
	base := time.Date(2019, 1, 1, 12, 0, 0, 0, time.UTC)
	converted := (DateTest{Zone: "+2500"}).applyZone(rd, base)
	got, err := extractDatePart(converted, DatePartISO8601)
	if err != nil || got != "2019-01-01T12:00:00Z" {
		t.Fatalf("explicit-zone ISO=%q err=%v", got, err)
	}
	zone, _ := extractDatePart(converted, DatePartZone)
	hour, _ := extractDatePart(converted, DatePartHour)
	if zone != "+2500" || hour != "13" {
		t.Fatalf("explicit-zone other parts changed: %q/%q", zone, hour)
	}
}
