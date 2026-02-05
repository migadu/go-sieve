package interp

import (
	"context"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

// DatePart represents the various date parts that can be extracted from a date-time value
type DatePart string

const (
	DatePartYear    DatePart = "year"
	DatePartMonth   DatePart = "month"
	DatePartDay     DatePart = "day"
	DatePartDate    DatePart = "date"
	DatePartJulian  DatePart = "julian"
	DatePartHour    DatePart = "hour"
	DatePartMinute  DatePart = "minute"
	DatePartSecond  DatePart = "second"
	DatePartTime    DatePart = "time"
	DatePartISO8601 DatePart = "iso8601"
	DatePartStd11   DatePart = "std11"
	DatePartZone    DatePart = "zone"
	DatePartWeekday DatePart = "weekday"
)

// ValidDateParts contains all valid date-part values
var ValidDateParts = map[DatePart]struct{}{
	DatePartYear:    {},
	DatePartMonth:   {},
	DatePartDay:     {},
	DatePartDate:    {},
	DatePartJulian:  {},
	DatePartHour:    {},
	DatePartMinute:  {},
	DatePartSecond:  {},
	DatePartTime:    {},
	DatePartISO8601: {},
	DatePartStd11:   {},
	DatePartZone:    {},
	DatePartWeekday: {},
}

// extractDatePart extracts the specified part from a time value
func extractDatePart(t time.Time, part DatePart) (string, error) {
	switch part {
	case DatePartYear:
		return strconv.Itoa(t.Year()), nil
	case DatePartMonth:
		return fmt.Sprintf("%02d", int(t.Month())), nil
	case DatePartDay:
		return fmt.Sprintf("%02d", t.Day()), nil
	case DatePartDate:
		return t.Format("2006-01-02"), nil
	case DatePartJulian:
		// Modified Julian Day - days since November 17, 1858
		return strconv.Itoa(modifiedJulianDay(t)), nil
	case DatePartHour:
		return fmt.Sprintf("%02d", t.Hour()), nil
	case DatePartMinute:
		return fmt.Sprintf("%02d", t.Minute()), nil
	case DatePartSecond:
		return fmt.Sprintf("%02d", t.Second()), nil
	case DatePartTime:
		return t.Format("15:04:05"), nil
	case DatePartISO8601:
		// ISO 8601 format with timezone offset like +03:00
		return t.Format("2006-01-02T15:04:05-07:00"), nil
	case DatePartStd11:
		// RFC 2822/5322 format (originally from RFC 822)
		return t.Format(time.RFC1123Z), nil
	case DatePartZone:
		return t.Format("-0700"), nil
	case DatePartWeekday:
		// 0 = Sunday, 6 = Saturday
		return strconv.Itoa(int(t.Weekday())), nil
	default:
		return "", fmt.Errorf("unknown date-part: %s", part)
	}
}

// modifiedJulianDay calculates the Modified Julian Day for a given time.
// MJD is the number of days since November 17, 1858 00:00 UTC.
// This corresponds to the regular Julian Day minus 2400000.5.
func modifiedJulianDay(t time.Time) int {
	year := t.Year()
	month := int(t.Month())
	day := t.Day()

	// Algorithm from https://en.wikipedia.org/wiki/Julian_day
	a := (14 - month) / 12
	y := year + 4800 - a
	m := month + 12*a - 3

	// Gregorian calendar - Julian Day Number
	jdn := day + (153*m+2)/5 + 365*y + y/4 - y/100 + y/400 - 32045

	// Convert to Modified Julian Day
	// MJD = JDN - 2400001 (integer version, as we don't track fractional days)
	return jdn - 2400001
}

// parseZoneOffset parses a zone offset string like "+0500" or "-0800" and returns the offset in seconds
func parseZoneOffset(zone string) (int, error) {
	if len(zone) != 5 {
		return 0, fmt.Errorf("invalid zone format: %s", zone)
	}

	sign := 1
	if zone[0] == '-' {
		sign = -1
	} else if zone[0] != '+' {
		return 0, fmt.Errorf("invalid zone format: %s", zone)
	}

	hours, err := strconv.Atoi(zone[1:3])
	if err != nil {
		return 0, fmt.Errorf("invalid zone hours: %s", zone)
	}

	minutes, err := strconv.Atoi(zone[3:5])
	if err != nil {
		return 0, fmt.Errorf("invalid zone minutes: %s", zone)
	}

	return sign * (hours*3600 + minutes*60), nil
}

// parseDateHeader parses a date from a header value
// It supports various common date formats
func parseDateHeader(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty date value")
	}

	// Try RFC 2822 format first (most common for email)
	t, err := mail.ParseDate(value)
	if err == nil {
		return t, nil
	}

	// Try other common formats
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		time.RFC3339,
		time.RFC3339Nano,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"2 Jan 2006 15:04:05 -0700",
		"2 Jan 2006 15:04:05 MST",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
		"02 Jan 2006 15:04:05 -0700",
		"02 Jan 2006 15:04:05 MST",
	}

	for _, format := range formats {
		t, err := time.Parse(format, value)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", value)
}

// DateTest implements the "date" test from RFC 5260
// It extracts a date-time from a header field and compares a date-part against key strings
type DateTest struct {
	matcherTest

	Header       string   // Header field to extract date from
	DatePart     DatePart // Part of date to compare
	Zone         string   // Time zone offset (e.g., "+0500")
	OriginalZone bool     // Use original zone from header
	Index        int      // Index for multiple headers (from "index" extension)
	Last         bool     // Use last header instead of first (from "index" extension)
}

func (d DateTest) Check(_ context.Context, rd *RuntimeData) (bool, error) {
	header := expandVars(rd, d.Header)

	values, err := rd.Msg.HeaderGet(header)
	if err != nil {
		return false, err
	}

	// Handle :count match type
	if d.isCount() {
		// Count valid dates in the header values
		validCount := uint64(0)
		for _, value := range values {
			if _, err := parseDateHeader(value); err == nil {
				validCount++
			}
		}
		return d.countMatches(rd, validCount), nil
	}

	if len(values) == 0 {
		return false, nil
	}

	// Select the appropriate header value based on index
	var value string
	if d.Index > 0 {
		idx := d.Index - 1 // Convert to 0-based
		if d.Last {
			// Index from the end
			idx = len(values) - d.Index
		}
		if idx < 0 || idx >= len(values) {
			return false, nil
		}
		value = values[idx]
	} else {
		value = values[0]
	}

	// Parse the date from the header
	t, err := parseDateHeader(value)
	if err != nil {
		return false, nil // Invalid date doesn't match
	}

	// Apply zone transformation
	t = d.applyZone(t)

	// Extract the date part
	datePart := DatePart(strings.ToLower(expandVars(rd, string(d.DatePart))))
	partValue, err := extractDatePart(t, datePart)
	if err != nil {
		return false, err
	}

	// Match against keys
	return d.matcherTest.tryMatch(rd, partValue)
}

func (d DateTest) applyZone(t time.Time) time.Time {
	if d.OriginalZone {
		// Keep the original zone
		return t
	}

	if d.Zone != "" {
		// Apply specified zone
		offset, err := parseZoneOffset(d.Zone)
		if err == nil {
			loc := time.FixedZone("", offset)
			return t.In(loc)
		}
	}

	// Default: use local time zone
	return t.Local()
}

// CurrentDateTest implements the "currentdate" test from RFC 5260
// It compares a date-part of the current date/time against key strings
type CurrentDateTest struct {
	matcherTest

	DatePart DatePart // Part of date to compare
	Zone     string   // Time zone offset (e.g., "+0500")
}

func (c CurrentDateTest) Check(_ context.Context, rd *RuntimeData) (bool, error) {
	// Get current time
	t := time.Now()

	// Apply zone transformation
	if c.Zone != "" {
		offset, err := parseZoneOffset(c.Zone)
		if err == nil {
			loc := time.FixedZone("", offset)
			t = t.In(loc)
		}
	}
	// If no zone specified, use local time (which is what time.Now() returns)

	// Extract the date part
	datePart := DatePart(strings.ToLower(expandVars(rd, string(c.DatePart))))
	partValue, err := extractDatePart(t, datePart)
	if err != nil {
		return false, err
	}

	// Match against keys
	return c.matcherTest.tryMatch(rd, partValue)
}
