package interp

import (
	"context"
	"fmt"
	"net/mail"
	"regexp"
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
		// RFC 3339 date-time, with a zero offset as "Z" and not "+00:00"
		// (RFC 5260 §4.2).
		return t.Format("2006-01-02T15:04:05Z07:00"), nil
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

// parseZoneOffset parses a time-zone of RFC 5260 §4.1, ( "+" / "-" ) 4DIGIT
// like "+0500" or "-0800", and returns the offset in seconds.
func parseZoneOffset(zone string) (int, error) {
	if len(zone) != 5 || (zone[0] != '+' && zone[0] != '-') {
		return 0, fmt.Errorf("invalid zone format: %s", zone)
	}
	for _, c := range zone[1:] {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid zone format: %s", zone)
		}
	}

	hours := int(zone[1]-'0')*10 + int(zone[2]-'0')
	minutes := int(zone[3]-'0')*10 + int(zone[4]-'0')
	offset := hours*3600 + minutes*60
	if zone[0] == '-' {
		offset = -offset
	}
	return offset, nil
}

// zoneLocation returns the time zone a :zone argument names once its
// variables are expanded (RFC 5229 §3). An empty zone is the local one (RFC
// 5260 §4.1: "If both the :zone and :originalzone arguments are omitted, the
// local time zone MUST be used."), and so, as in Pigeonhole, is an expanded
// one that is not a time-zone: a literal one was refused when the script
// loaded.
func zoneLocation(rd *RuntimeData, zone string) *time.Location {
	if zone != "" {
		if offset, err := parseZoneOffset(expandVars(rd, zone)); err == nil {
			return time.FixedZone("", offset)
		}
	}
	return time.Local
}

// expandDatePart expands the variables of a date-part (RFC 5229 §3) and
// reports whether the result is a known date-part. Only a literal one was
// checked when the script loaded; as in Pigeonhole, an unknown expanded one
// makes the test false.
func expandDatePart(rd *RuntimeData, part DatePart) (DatePart, bool) {
	part = DatePart(strings.ToLower(expandVars(rd, string(part))))
	_, ok := ValidDateParts[part]
	return part, ok
}

// obsZones maps the obsolete alphabetic zones RFC 5322 §4.3 gives a meaning to
// the numeric zone each one is "semantically equivalent to". ABNF strings are
// case insensitive (RFC 5234 §2.3), so the keys are upper case and lookups fold
// case.
var obsZones = map[string]string{
	"UT": "+0000", "GMT": "+0000",
	"EDT": "-0400", "EST": "-0500",
	"CDT": "-0500", "CST": "-0600",
	"MDT": "-0600", "MST": "-0700",
	"PDT": "-0700", "PST": "-0800",
}

// alphabeticZone matches a date-time (RFC 5322 §3.3) up to the end of its
// time-of-day (the field's first colon is the one after the hour) and the run
// of letters that follows it after any folding white space: the zone.
var alphabeticZone = regexp.MustCompile(`^[^:]*\d:\d{2}(?::\d{2})?([ \t]*)([A-Za-z]+)`)

// leapSecond matches a date-time (RFC 5322 §3.3) up to the seconds of its
// time-of-day (the field's first colon is the one after the hour) when they
// are 60: a leap second.
var leapSecond = regexp.MustCompile(`^[^:]*\d:\d{2}:(60)(?:\D|$)`)

// obsYear matches a date-time (RFC 5322 §3.3) up to a year of two or three
// digits (obs-year, §4.3): the digits right before the hour, which the field's
// first colon follows.
var obsYear = regexp.MustCompile(`^[^:]*[ \t](\d{2,3})[ \t]+\d{1,2}:`)

// parseRFC5322Date parses an RFC 5322 date-time (§3.3) like net/mail.ParseDate,
// but reads an alphabetic zone by RFC 5322 §4.3 instead of by the server's time
// zone, and a two- or three-digit year by §4.3 too.
//
// net/mail.ParseDate gives an alphabetic zone the offset it has in time.Local,
// and offset zero when time.Local does not know it: PDT is 02:23:25 UTC on a
// server in UTC, and EST in July is given in EDT on one in America/New_York.
// Here the zone is replaced by its numeric equivalent before parsing: the ten
// names of §4.3 by their offsets, and the military zones and any other name by
// "-0000", which §4.3 says they "SHOULD all be considered equivalent to"
// without out-of-band information.
//
// net/mail.ParseDate reads a two-digit year with time.Parse's "06", which puts
// 00-68 in 20xx, and does not parse a three-digit year (tm_year written as is
// after 1999). §4.3 adds 2000 to a two-digit year from 00 to 49, and 1900 to
// one from 50 to 99 "or any three digit year", so the year is written out in
// four digits before parsing.
//
// A time-of-day whose second is 60, a leap second (§3.3: "the time-of-day MUST
// be in the range 00:00:00 through 23:59:60"), is read as second 59 of the
// same minute, as Dovecot reads it: time.Time cannot hold second 60, and
// net/mail.ParseDate does not parse it.
//
// net/mail.ParseDate also takes a comment only after the zone, and white space
// only where its layouts have a space. §4.3 permits comments and folding white
// space between the tokens (obs-day-of-week, obs-day, obs-year, obs-hour,
// obs-minute and obs-second are each "[CFWS] ... [CFWS]"), so they are first
// reduced to the separators net/mail's layouts have.
func parseRFC5322Date(value string) (time.Time, error) {
	// Unfold exactly as net/mail does, without turning an unpaired CR or LF
	// before the zone into valid white space during the replacement below.
	unfolded := strings.ReplaceAll(value, "\r\n", "")
	if strings.ContainsAny(unfolded, "\r\n") {
		return mail.ParseDate(value)
	}
	unfolded = normalizeCFWS(unfolded)
	if m := obsYear.FindStringSubmatchIndex(unfolded); m != nil {
		year, _ := strconv.Atoi(unfolded[m[2]:m[3]])
		if m[3]-m[2] == 2 && year < 50 {
			year += 2000
		} else {
			year += 1900
		}
		unfolded = unfolded[:m[2]] + strconv.Itoa(year) + unfolded[m[3]:]
	}
	if m := leapSecond.FindStringSubmatchIndex(unfolded); m != nil {
		unfolded = unfolded[:m[2]] + "59" + unfolded[m[3]:]
	}
	if m := alphabeticZone.FindStringSubmatchIndex(unfolded); m != nil {
		end := m[5]
		// Replace a whole zone token only. For example, PDT+0400 is not PDT:
		// net/mail would discard the suffix after a numeric replacement. Keep
		// its existing behavior for extensions it accepts, such as GMT+0000
		// (unfolded already has its separators, year and leap second rewritten).
		if end < len(unfolded) && unfolded[end] != ' ' && unfolded[end] != '\t' && unfolded[end] != '(' {
			return mail.ParseDate(unfolded)
		}
		// Nor are the letters the zone when a numeric offset follows them after
		// white space or a comment, as in "02:23:25 PM -0700": the grammar has
		// one zone after the time of day ("time = time-of-day zone", §3.3), and
		// net/mail would discard the offset after the replacement.
		if rest := strings.TrimLeft(unfolded[end:], " \t"); rest != "" && (rest[0] == '+' || rest[0] == '-') {
			return mail.ParseDate(unfolded)
		}
		zone, ok := obsZones[strings.ToUpper(unfolded[m[4]:end])]
		if !ok {
			zone = "-0000"
		}
		// An obs-zone needs no white space before it, a numeric zone does.
		unfolded = unfolded[:m[2]] + " " + zone + unfolded[end:]
	}
	return mail.ParseDate(unfolded)
}

// normalizeCFWS returns an unfolded date-time with every comment (RFC 5322
// §3.2.2) and run of white space between two tokens written as one space, as
// net/mail's layouts separate tokens, except that there is none before the
// comma after the day name or around a colon between the digits of the time
// of day, and always one after that comma: day = ([FWS] 1*2DIGIT FWS) makes it
// optional (§3.3). A date-time has no quoted strings, so every "(" opens a
// comment; the rest of the value from one that does not close is left as
// written, for net/mail to reject.
func normalizeCFWS(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	var prev byte   // the last byte written
	sep := false    // white space or a comment since prev
	digits := false // a digit was written: the day-of-week is behind
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' {
			sep = true
			continue
		}
		if c == '(' {
			if n := commentLen(s[i:]); n > 0 {
				i += n - 1
				sep = true
				continue
			}
		}
		dayComma := c == ',' && !digits
		timeColon := c == ':' && isDigit(prev) || prev == ':' && isDigit(c)
		if sep && prev != 0 && !dayComma && !timeColon {
			b.WriteByte(' ')
		}
		if c == '(' {
			b.WriteString(s[i:])
			break
		}
		b.WriteByte(c)
		prev, sep, digits = c, dayComma, digits || isDigit(c)
	}
	return b.String()
}

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

// commentLen returns the length of the comment s starts with, nested comments
// and quoted-pairs included (RFC 5322 §3.2.2), or 0 if it does not close.
func commentLen(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '(':
			depth++
		case ')':
			if depth--; depth == 0 {
				return i + 1
			}
		}
	}
	return 0
}

// parseDateHeader parses the date-time of a header value: the whole value, as
// in a Date: field, or else the text after its last semicolon, as in a
// Received: field. It supports various common date formats.
func parseDateHeader(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty date value")
	}

	// Try RFC 2822 format first (most common for email)
	t, err := parseRFC5322Date(value)
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

	// RFC 5260 §4: the date-time may also appear "at the end of a header field
	// following a semicolon (e.g., as it does in a standard Received: header
	// field)", and with more than one "the last one that appears SHOULD be
	// used". Semicolons in comments or quoted strings are not separators.
	if i := lastDateSeparator(value); i >= 0 {
		return parseDateHeader(value[i+1:])
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", value)
}

// lastDateSeparator returns the last semicolon outside comments, quoted
// strings and domain-literals (the tokens of a Received field, RFC 5322
// section 3.6.7). Each may contain quoted-pairs; comments may also nest. The
// suffix is parsed as a complete date-time, including trailing CFWS.
func lastDateSeparator(value string) int {
	last, depth := -1, 0
	quoted, literal := false, false
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c == '\\' && (quoted || literal || depth > 0) {
			i++ // a quoted-pair cannot open or close either construct
			continue
		}
		if depth > 0 {
			switch c {
			case '(':
				depth++
			case ')':
				depth--
			}
			continue
		}
		if quoted {
			if c == '"' {
				quoted = false
			}
			continue
		}
		if literal {
			if c == ']' {
				literal = false
			}
			continue
		}

		switch c {
		case '(':
			depth = 1
		case '"':
			quoted = true
		case '[':
			literal = true
		case ';':
			last = i
		}
	}
	return last
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

func (d DateTest) Check(ctx context.Context, rd *RuntimeData) (bool, error) {
	datePart, ok := expandDatePart(rd, d.DatePart)
	if !ok {
		return false, nil
	}
	header := expandVars(rd, d.Header)

	// The fields as editheader left them (RFC 5293 §7: tests "MUST examine
	// the current state of a header as modified by any actions that have
	// taken place so far").
	index := fieldIndex{Index: d.Index, Last: d.Last}
	byName, err := index.selectFields([]string{header}, func(name string) ([]string, error) {
		return GetHeaderWithEdits(rd, name)
	})
	if err != nil {
		return false, err
	}
	values := byName[0]

	// RFC 5260 §4: "The count of a date test is 1 if the specified field
	// exists and contains a valid date; 0, otherwise." The specified field
	// is the first one, or the one :index selects, as below.
	if d.isCount() {
		count := uint64(0)
		if len(values) > 0 {
			if _, err := parseDateHeader(values[0]); err == nil {
				count = 1
			}
		}
		return d.countMatches(rd, count), nil
	}

	if len(values) == 0 {
		return false, nil
	}

	// The first field (RFC 5260 §4), or the one :index selects.
	value := values[0]

	// Parse the date from the header
	t, err := parseDateHeader(value)
	if err != nil {
		return false, nil // Invalid date doesn't match
	}

	// Apply zone transformation
	t = d.applyZone(rd, t)

	// Extract the date part
	partValue, err := extractDatePart(t, datePart)
	if err != nil {
		return false, err
	}

	// Match against keys
	return d.matcherTest.tryMatch(ctx, rd, partValue)
}

func (d DateTest) applyZone(rd *RuntimeData, t time.Time) time.Time {
	if d.OriginalZone {
		// Keep the original zone
		return t
	}
	return t.In(zoneLocation(rd, d.Zone))
}

// CurrentDateTest implements the "currentdate" test from RFC 5260
// It compares a date-part of the current date/time against key strings
type CurrentDateTest struct {
	matcherTest

	DatePart DatePart // Part of date to compare
	Zone     string   // Time zone offset (e.g., "+0500")
}

func (c CurrentDateTest) Check(ctx context.Context, rd *RuntimeData) (bool, error) {
	datePart, ok := expandDatePart(rd, c.DatePart)
	if !ok {
		return false, nil
	}

	// RFC 5260 §5: "The :count value of a currentdate test is always 1."
	if c.isCount() {
		return c.countMatches(rd, 1), nil
	}

	// The current time, in the :zone or the local zone
	t := time.Now().In(zoneLocation(rd, c.Zone))

	// Extract the date part
	partValue, err := extractDatePart(t, datePart)
	if err != nil {
		return false, err
	}

	// Match against keys
	return c.matcherTest.tryMatch(ctx, rd, partValue)
}
