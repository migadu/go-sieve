package tests

import (
	"bufio"
	"context"
	"fmt"
	"net/textproto"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/migadu/go-sieve"
	"github.com/migadu/go-sieve/interp"
)

var regexKeyExtensions = []string{
	"fileinto", "envelope", "variables", "body", "date", "imap4flags",
	"editheader", "regex", "comparator-i;unicode-casemap",
}

func loadWithRegexKeyExtensions(script string) (*sieve.Script, error) {
	opts := sieve.DefaultOptions()
	opts.EnabledExtensions = regexKeyExtensions
	return sieve.Load(strings.NewReader(script), opts)
}

// draft-murchison-sieve-regex-08 §3: "Implementations SHOULD reject regular
// expressions that are unsupported by this specification as a syntax error."
// A :regex key without variables is known when the script loads, so one that
// does not compile refuses the script there, as a :matches pattern that does
// not compile does, and as Pigeonhole does for a key that is a string literal
// ("invalid regular expression"). It used to load and fail its test at every
// message.
func TestRegexKeyRefusedAtLoad(t *testing.T) {
	tooLong := strings.Repeat("a", interp.DefaultRegexLimits.MaxPatternLength+1)
	tooLarge := "(?:" + strings.Repeat("a", 990) + "){1000}"
	refused := []struct {
		name   string
		script string
	}{
		{"header", `require "regex"; if header :regex "Subject" "(" { stop; }`},
		{"header i;octet", `require "regex"; if header :regex :comparator "i;octet" "Subject" "(" { stop; }`},
		{"header i;unicode-casemap", `require ["regex", "comparator-i;unicode-casemap"]; if header :regex :comparator "i;unicode-casemap" "Subject" "(" { stop; }`},
		{"one bad key of several", `require "regex"; if header :regex "Subject" ["^a", "[z-a]"] { stop; }`},
		{"pattern too long", `require "regex"; if header :regex "Subject" "` + tooLong + `" { stop; }`},
		{"program too large", `require "regex"; if header :regex "Subject" "` + tooLarge + `" { stop; }`},
		{"address", `require "regex"; if address :regex "from" "a{2,1}" { stop; }`},
		{"envelope", `require ["envelope", "regex"]; if envelope :regex "from" "(" { stop; }`},
		// The keys of Pigeonhole's tests/extensions/regex/errors/compile.sieve.
		{"Pigeonhole address", `require "regex"; if address :regex "from" "sirius(+\\+.*)?@friep\\.example\\.com" { stop; }`},
		{"Pigeonhole header", `require "regex"; if header :regex "from" "sirius(\\+.*)?@friep\\.ex[]ample.com" { stop; }`},
		{"Pigeonhole envelope", `require ["envelope", "regex"]; if envelope :regex "from" "sirius(\\+.*)?@friep\\.ex[]ample.com" { stop; }`},
		{"string", `require ["variables", "regex"]; if string :regex "abc" "(" { stop; }`},
		{"body", `require ["body", "regex"]; if body :regex "(" { stop; }`},
		{"date", `require ["date", "regex"]; if date :regex "date" "year" "(" { stop; }`},
		{"currentdate", `require ["date", "regex"]; if currentdate :regex "year" "(" { stop; }`},
		{"hasflag", `require ["imap4flags", "regex"]; if hasflag :regex "(" { stop; }`},
		{"deleteheader", `require ["editheader", "regex"]; deleteheader :regex "X-Foo" "(";`},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadWithRegexKeyExtensions(tc.script)
			if err == nil {
				t.Fatal("load succeeded, want an error")
			}
			if !strings.Contains(err.Error(), "malformed pattern") {
				t.Fatalf("error %q, want it to contain %q", err, "malformed pattern")
			}
		})
	}

	accepted := []string{
		`require "regex"; if header :regex "Subject" ["^a", "b(c)d$"] { stop; }`,
		`require ["body", "regex"]; if body :regex "^x" { stop; }`,
		`require ["imap4flags", "regex"]; if hasflag :regex "^\\\\Se" { stop; }`,
		// A key with a variable is known only when the test runs.
		`require ["variables", "regex"]; set "p" "("; if header :regex "Subject" "${p}" { stop; }`,
	}
	for _, script := range accepted {
		if _, err := loadWithRegexKeyExtensions(script); err != nil {
			t.Errorf("%s: %v", script, err)
		}
	}
}

// A :regex key is checked, not compiled, when the script loads. Counted
// repetition makes a key of MaxPatternLength bytes, such as "(?:a...a){1000}",
// compile to a program of some 45 MB in some 40 ms, so compiling and keeping
// every key of a script under ManageSieve's 16 KB limit made each load (each
// PUTSCRIPT, each script compiled for delivery) take half a second and hold
// close to 700 MB for as long as the script was kept.
func TestRegexKeyCheckAtLoadDoesNotCompile(t *testing.T) {
	key := `"(?:` + strings.Repeat("a", 990) + `){1000}"`
	keys := make([]string, 15)
	for i := range keys {
		keys[i] = key
	}
	script := `require "regex"; if header :regex "Subject" [` + strings.Join(keys, ", ") + `] { stop; }`

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	loaded, err := loadWithRegexKeyExtensions(script)
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	runtime.KeepAlive(loaded)
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 16<<20 {
		t.Fatalf("loading a %d-byte script allocated %d MB, want under 16 MB", len(script), allocated>>20)
	}
}

// runRegexKeyScript runs script against message and returns the mailboxes it
// files into.
func runRegexKeyScript(t *testing.T, script, message string) []string {
	t.Helper()
	loaded, err := loadWithRegexKeyExtensions(script)
	if err != nil {
		t.Fatalf("%s: %v", script, err)
	}
	hdr, err := textproto.NewReader(bufio.NewReader(strings.NewReader(message))).ReadMIMEHeader()
	if err != nil {
		t.Fatal(err)
	}
	data := sieve.NewRuntimeData(loaded, interp.DummyPolicy{},
		interp.EnvelopeStatic{From: "from@example.com", To: "to@example.com"},
		interp.MessageStatic{Size: len(message), Header: hdr})
	if err := loaded.Execute(context.Background(), data); err != nil {
		t.Fatalf("%s: %v", script, err)
	}
	return data.Mailboxes
}

// A :regex key checked when the script loads matches as the same key does
// when it is known only at run time, through a variable: the same result and
// the same match variables, under every comparator :regex runs with.
func TestRegexKeyCheckedAtLoadMatchesAsAtRunTime(t *testing.T) {
	const message = "Subject: I have a Present for you\r\n\r\nBody.\r\n"

	const octet = `require ["fileinto", "variables", "regex"]; if header :regex :comparator "i;octet" "Subject" "^I have a (.*) for you$" { fileinto "hit-${1}"; }`
	if got, want := runRegexKeyScript(t, octet, message), []string{"hit-Present"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("i;octet: filed into %q, want %q", got, want)
	}

	comparators := []string{"i;octet", "i;ascii-casemap", "i;unicode-casemap"}
	patterns := []string{"^I have a (.*) for you$", "(Present)", "(present)", "^(i) (h)", "absent"}
	for _, comparator := range comparators {
		for _, pattern := range patterns {
			constant := fmt.Sprintf(`require ["fileinto", "variables", "regex", "comparator-i;unicode-casemap"];
if header :regex :comparator %q "Subject" %q { fileinto "hit-${1}"; }`, comparator, pattern)
			variable := fmt.Sprintf(`require ["fileinto", "variables", "regex", "comparator-i;unicode-casemap"];
set "p" %q;
if header :regex :comparator %q "Subject" "${p}" { fileinto "hit-${1}"; }`, pattern, comparator)

			atLoad := runRegexKeyScript(t, constant, message)
			atRun := runRegexKeyScript(t, variable, message)
			if !reflect.DeepEqual(atLoad, atRun) {
				t.Errorf("%s %q: the constant key filed into %q, the key through a variable into %q", comparator, pattern, atLoad, atRun)
			}
		}
	}
}

// A :regex key known only when the test runs is refused then, as the same key
// written as a string is refused at load, when its program would be too large.
// It used to be compiled, to about a million instructions, for every value the
// test matched it against.
func TestRegexKeyTooLargeRefusedAtRunTime(t *testing.T) {
	script := `require ["fileinto", "variables", "regex"];
set "p" "(?:` + strings.Repeat("a", 990) + `){1000}";
if header :regex "Received" "${p}" { fileinto "hit"; }`
	loaded, err := loadWithRegexKeyExtensions(script)
	if err != nil {
		t.Fatal(err)
	}
	message := strings.Repeat("Received: from host.example.com by mx.example.org\r\n", 10) + "Subject: x\r\n\r\nBody.\r\n"
	hdr, err := textproto.NewReader(bufio.NewReader(strings.NewReader(message))).ReadMIMEHeader()
	if err != nil {
		t.Fatal(err)
	}
	data := sieve.NewRuntimeData(loaded, interp.DummyPolicy{},
		interp.EnvelopeStatic{From: "from@example.com", To: "to@example.com"},
		interp.MessageStatic{Size: len(message), Header: hdr})
	if err := loaded.Execute(context.Background(), data); err == nil || !strings.Contains(err.Error(), "too complex") {
		t.Fatalf("Execute: %v, want a too complex error", err)
	}
}

// A :regex key is compiled once, not once for every value the test matches it
// against: Pigeonhole likewise compiles each key once per match operation
// (mcht_regex_match_keys) and reuses it for every value. The key compiles to
// 8000 instructions, within the bound, so matching it against ten Received
// fields allocated ten times what matching it against one does.
func TestRegexKeyCompiledOncePerExecution(t *testing.T) {
	const key = "(?:abcdefgh){1000}"
	scripts := map[string]string{
		"constant": `require ["fileinto", "regex"];
if header :regex "Received" "` + key + `" { fileinto "hit"; }`,
		"variable": `require ["fileinto", "variables", "regex"];
set "p" "` + key + `";
if header :regex "Received" "${p}" { fileinto "hit"; }`,
	}
	for name, script := range scripts {
		t.Run(name, func(t *testing.T) {
			loaded, err := loadWithRegexKeyExtensions(script)
			if err != nil {
				t.Fatal(err)
			}
			execute := func(fields int) uint64 {
				message := strings.Repeat("Received: from host.example.com by mx.example.org\r\n", fields) + "Subject: x\r\n\r\nBody.\r\n"
				hdr, err := textproto.NewReader(bufio.NewReader(strings.NewReader(message))).ReadMIMEHeader()
				if err != nil {
					t.Fatal(err)
				}
				data := sieve.NewRuntimeData(loaded, interp.DummyPolicy{},
					interp.EnvelopeStatic{From: "from@example.com", To: "to@example.com"},
					interp.MessageStatic{Size: len(message), Header: hdr})
				var before, after runtime.MemStats
				runtime.ReadMemStats(&before)
				err = loaded.Execute(context.Background(), data)
				runtime.ReadMemStats(&after)
				if err != nil {
					t.Fatal(err)
				}
				if len(data.Mailboxes) != 0 {
					t.Fatalf("filed into %q", data.Mailboxes)
				}
				return after.TotalAlloc - before.TotalAlloc
			}
			one, ten := execute(1), execute(10)
			if ten > 2*one {
				t.Fatalf("matching the key against ten fields allocated %d KB, against one %d KB", ten>>10, one>>10)
			}
		})
	}
}
