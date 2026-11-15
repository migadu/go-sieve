package interp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/migadu/go-sieve/lexer"
	"github.com/migadu/go-sieve/parser"
)

// runCurrentDate runs script, which may use the date and variables extensions,
// on d's message after setting d up with set.
func runCurrentDate(t *testing.T, script string, set func(d *RuntimeData)) *RuntimeData {
	t.Helper()
	toks, err := lexer.Lex(strings.NewReader(script), &lexer.Options{MaxTokens: 5000})
	if err != nil {
		t.Fatal(err)
	}
	cmds, err := parser.Parse(lexer.NewStream(toks), &parser.Options{MaxBlockNesting: 15, MaxTestNesting: 15})
	if err != nil {
		t.Fatal(err)
	}
	s, err := LoadScript(cmds, &Options{MaxVariableCount: 128, MaxVariableNameLen: 32, MaxVariableLen: 4000},
		[]string{"date", "variables"})
	if err != nil {
		t.Fatal(err)
	}
	d := NewRuntimeData(s, DummyPolicy{}, EnvelopeStatic{}, MessageStatic{})
	if set != nil {
		set(d)
	}
	if err := s.Execute(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	return d
}

// The date-parts each currentdate test reads, in three zones.
const currentDateParts = `require ["date", "variables"];
if currentdate :zone "+0000" :matches "date" "*" { set "date" "${1}"; }
if currentdate :zone "+0000" :matches "time" "*" { set "time" "${1}"; }
if currentdate :zone "-0100" :matches "iso8601" "*" { set "iso8601" "${1}"; }
if currentdate :zone "+0200" :matches "std11" "*" { set "std11" "${1}"; }
`

// RFC 5260 §5: "All currentdate tests in a single Sieve script MUST refer to
// the same point in time during execution of the script." Each currentdate
// test read the clock itself, so tests on either side of a second, minute or
// day boundary saw different instants. Every test reads the instant of the
// execution; one that read the clock would see today, not 2001.
func TestCurrentDateOneInstant(t *testing.T) {
	instant := time.Date(2001, 2, 3, 23, 59, 59, 0, time.UTC)
	d := runCurrentDate(t, currentDateParts, func(d *RuntimeData) { d.currentTime = instant })

	for name, want := range map[string]string{
		"date":    "2001-02-03",
		"time":    "23:59:59",
		"iso8601": "2001-02-03T22:59:59-01:00",
		"std11":   "Sun, 04 Feb 2001 01:59:59 +0200",
	} {
		if got := d.Variables[name]; got != want {
			t.Errorf("currentdate %s = %q, want %q", name, got, want)
		}
	}
}

// The first currentdate test of an execution records the instant the others
// read, and a copy of the runtime data (test_script_run) keeps it.
func TestCurrentDateRecordsInstant(t *testing.T) {
	before := time.Now()
	d := runCurrentDate(t, currentDateParts, nil)
	after := time.Now()

	instant := d.currentTime
	if instant.Before(before) || instant.After(after) {
		t.Fatalf("recorded instant %v, want one between %v and %v", instant, before, after)
	}
	utc := instant.UTC()
	for name, want := range map[string]string{
		"date":    utc.Format("2006-01-02"),
		"time":    utc.Format("15:04:05"),
		"iso8601": instant.In(time.FixedZone("", -3600)).Format("2006-01-02T15:04:05Z07:00"),
		"std11":   instant.In(time.FixedZone("", 7200)).Format(time.RFC1123Z),
	} {
		if got := d.Variables[name]; got != want {
			t.Errorf("currentdate %s = %q, want %q (the recorded instant)", name, got, want)
		}
	}
	if c := d.Copy(); !c.currentTime.Equal(instant) {
		t.Errorf("copy has instant %v, want %v", c.currentTime, instant)
	}
}
