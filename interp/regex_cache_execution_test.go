package interp

import (
	"context"
	"errors"
	"fmt"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/migadu/go-sieve/lexer"
	"github.com/migadu/go-sieve/parser"
)

// Load through the public lexer, parser and interpreter entry points so these
// tests exercise real Sieve commands rather than invoking a matcher alone.
func loadRegexCacheExecutionScript(t *testing.T, source string) *Script {
	t.Helper()
	tokens, err := lexer.Lex(strings.NewReader(source), &lexer.Options{MaxTokens: 5000})
	if err != nil {
		t.Fatal(err)
	}
	commands, err := parser.Parse(lexer.NewStream(tokens), &parser.Options{MaxBlockNesting: 15, MaxTestNesting: 15})
	if err != nil {
		t.Fatal(err)
	}
	script, err := LoadScript(commands, &Options{
		MaxRedirects: 5, MaxVariableCount: 128, MaxVariableNameLen: 32, MaxVariableLen: 4000,
		RegexLimits: RegexLimits{MaxExecTime: 10 * time.Second},
	}, []string{"regex", "variables", "mime", "foreverypart", "comparator-i;unicode-casemap"})
	if err != nil {
		t.Fatal(err)
	}
	return script
}

// A short script can create many large, individually permitted keys from MIME
// headers. Running out of cache space must change retention, never matching.
func TestRegexCacheExecutionAfterByteBudget(t *testing.T) {
	const parts = 32
	letters := strings.Repeat("Ü", 10)
	const finalKey = `^(\pL{10})u-key-31$`
	for _, tc := range []struct {
		name       string
		comparator Comparator
		flag       string
		letter     string
		match      bool
	}{
		{"default ASCII", ComparatorASCIICaseMap, "", "U", true},
		{"octet match", ComparatorOctet, `:comparator "i;octet"`, "u", true},
		{"octet nonmatch", ComparatorOctet, `:comparator "i;octet"`, "U", false},
		{"Unicode", ComparatorUnicodeCaseMap, `:comparator "i;unicode-casemap"`, "U", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := letters + tc.letter + "-key-31"
			source := fmt.Sprintf(`
require ["regex", "variables", "mime", "foreverypart", "comparator-i;unicode-casemap"];
set "input" %s;
foreverypart {
	if header :mime :matches "X-Key" "*" {
		set "partkey" "${1}";
		set "key" "^(\\pL{10})u${partkey}$";
		if string :regex %s "${input}" "${key}" {
			set "dynamic" "${0}";
			set "letters" "${1}";
		} else {
			if not string "${1}" "${partkey}" { set "clobbered" "yes"; }
		}
	}
}
if string :regex %s "${input}" %s { set "static" "${0}"; }
`, strconv.Quote(input), tc.flag, tc.flag, strconv.Quote(finalKey))
			script := loadRegexCacheExecutionScript(t, source)
			var body strings.Builder
			for i := 0; i < parts; i++ {
				fmt.Fprintf(&body, "--b\r\nContent-Type: text/plain\r\nX-Key: -key-%02d\r\n\r\npart\r\n", i)
			}
			body.WriteString("--b--\r\n")
			header := textproto.MIMEHeader{"Content-Type": {"multipart/mixed; boundary=b"}}
			data := NewRuntimeData(script, DummyPolicy{}, EnvelopeStatic{}, MessageStatic{Header: header, Body: []byte(body.String()), HasBody: true})
			if err := script.Execute(context.Background(), data); err != nil {
				t.Fatal(err)
			}
			want := ""
			if tc.match {
				want = input
				if got := data.Variables["letters"]; got != letters {
					t.Errorf("capture changed original text: %q", got)
				}
			}
			if got := data.Variables["dynamic"]; got != want {
				t.Errorf("dynamic key matched %q, want %q", got, want)
			}
			if got := data.Variables["static"]; got != want {
				t.Errorf("static key matched %q, want %q", got, want)
			}
			if data.Variables["clobbered"] != "" {
				t.Error("a skipped nonmatch clobbered the previous capture")
			}
			cache := &data.regexes
			if cache.bytes <= 0 || cache.bytes > regexCacheMaxBytes {
				t.Fatalf("cache keeps %d charged bytes, limit %d", cache.bytes, regexCacheMaxBytes)
			}
			if len(cache.matchers) == 0 || len(cache.matchers) >= parts {
				t.Fatalf("fixture did not exhaust retention: %d keys", len(cache.matchers))
			}
			if _, ok := cache.matchers[regexCacheKey{comparator: tc.comparator, pattern: finalKey}]; ok {
				t.Fatal("last key is cached; fixture did not test uncached fallthrough")
			}
			first, err := cache.compile(finalKey, tc.comparator)
			if err != nil {
				t.Fatal(err)
			}
			again, err := cache.compile(finalKey, tc.comparator)
			if err != nil {
				t.Fatal(err)
			}
			if first == again {
				t.Error("a key past the byte budget was retained")
			}
			if len(cache.matchers)+1 >= regexCacheMaxKeys || cache.size+first.size > regexCacheMaxSize {
				t.Error("fixture exhausted a key/program budget instead of the byte budget")
			}
		})
	}
}

func TestRegexCacheExecutionComparatorKeys(t *testing.T) {
	script := loadRegexCacheExecutionScript(t, `
require ["regex", "variables", "comparator-i;unicode-casemap"];
if string :regex "u" "^(U)$" { set "ascii" "${1}"; }
if string :comparator "i;octet" :regex "u" "^(U)$" { set "wrong" "yes"; }
if string :comparator "i;unicode-casemap" :regex "É" "^(é)$" { set "unicode" "${1}"; }
if string :regex "É" "^(é)$" { set "wrong" "yes"; }
set "key" "^(U)$";
if string :regex "u" "${key}" { set "dynamic" "${1}"; }
`)
	data := NewRuntimeData(script, DummyPolicy{}, EnvelopeStatic{}, MessageStatic{})
	if err := script.Execute(context.Background(), data); err != nil {
		t.Fatal(err)
	}
	if data.Variables["ascii"] != "u" || data.Variables["dynamic"] != "u" || data.Variables["unicode"] != "É" || data.Variables["wrong"] != "" {
		t.Fatalf("comparator cache keys or original captures changed: %v", data.Variables)
	}
	if len(data.regexes.matchers) != 4 {
		t.Errorf("static and expanded identical keys did not share the same comparator cache: %d entries", len(data.regexes.matchers))
	}
}

func TestRegexCacheExecutionCopyIsolated(t *testing.T) {
	script := loadRegexCacheExecutionScript(t, `require ["regex", "variables"]; if string :regex "ABC" "^([a-z]+)$" { set "capture" "${1}"; }`)
	original := NewRuntimeData(script, DummyPolicy{}, EnvelopeStatic{}, MessageStatic{})
	if err := script.Execute(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	copied := original.Copy()
	if len(copied.regexes.matchers) != 0 || copied.regexes.bytes != 0 || copied.regexes.size != 0 {
		t.Fatal("RuntimeData.Copy retained the original execution's cache")
	}
	if err := script.Execute(context.Background(), copied); err != nil {
		t.Fatal(err)
	}
	key := regexCacheKey{comparator: ComparatorASCIICaseMap, pattern: "^([a-z]+)$"}
	if copied.regexes.matchers[key] == original.regexes.matchers[key] {
		t.Error("copied execution reuses the original cache entry")
	}
	if _, _, err := matchRegex(context.Background(), &copied.regexes, ComparatorASCIICaseMap, "^(new)$", "NEW"); err != nil {
		t.Fatal(err)
	}
	if len(original.regexes.matchers) != 1 || len(copied.regexes.matchers) != 2 {
		t.Error("copied and original executions share a mutable cache")
	}
	if copied.Variables["capture"] != "ABC" || original.Variables["capture"] != "ABC" {
		t.Error("copy changed captured source text")
	}
}

func TestRegexCacheExecutionOversizeEntryFallback(t *testing.T) {
	letters := strings.Repeat("Ü", 40)
	const pattern = `^(\pL{40})$`
	for _, comparator := range []Comparator{ComparatorOctet, ComparatorASCIICaseMap, ComparatorUnicodeCaseMap} {
		t.Run(string(comparator), func(t *testing.T) {
			flag := ":comparator " + strconv.Quote(string(comparator))
			script := loadRegexCacheExecutionScript(t, fmt.Sprintf(`
require ["regex", "variables", "comparator-i;unicode-casemap"];
set "input" %s;
set "key" %s;
if string :regex %s "${input}" %s { set "static" "${1}"; }
if string :regex %s "${input}" "${key}" { set "dynamic" "${1}"; }
`, strconv.Quote(letters), strconv.Quote(pattern), flag, strconv.Quote(pattern), flag))
			data := NewRuntimeData(script, DummyPolicy{}, EnvelopeStatic{}, MessageStatic{})
			if err := script.Execute(context.Background(), data); err != nil {
				t.Fatal(err)
			}
			if data.Variables["static"] != letters || data.Variables["dynamic"] != letters {
				t.Fatalf("large legal key did not retain original captures: %v", data.Variables)
			}
			if data.regexes.bytes != 0 || data.regexes.size != 0 || len(data.regexes.matchers) != 0 {
				t.Fatal("individually oversized cache entry was retained")
			}
		})
	}
}

func TestRegexCacheExecutionContextLimits(t *testing.T) {
	const pattern = "^(.+)$"
	var cache regexCache
	matcher, err := cache.compile(pattern, ComparatorASCIICaseMap)
	if err != nil {
		t.Fatal(err)
	}
	ctx := ContextWithRegexLimits(context.Background(), RegexLimits{MaxInputLength: 3, MaxExecTime: time.Second})
	ok, captures, err := matchRegex(ctx, &cache, ComparatorASCIICaseMap, pattern, "abcdef")
	if err != nil || !ok || len(captures) != 2 || captures[0] != "abc" || captures[1] != "abc" {
		t.Fatalf("cached matcher ignored context input cap: %v %q %v", ok, captures, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := matchRegex(cancelled, &cache, ComparatorASCIICaseMap, pattern, "abcdef"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cached matcher ignored cancellation: %v", err)
	}
	// Make the cached engine start reading after its deadline, independent
	// of host speed. The synchronous reader must report the stopped match.
	find := matcher.find
	matcher.find = func(value string, in *deadlineReader) []string {
		if in == nil {
			t.Error("large cached match did not use its deadline reader")
			return find(value, in)
		}
		select {
		case <-in.done:
		case <-time.After(time.Second):
			t.Error("cached match's deadline did not close")
		}
		return find(value, in)
	}
	defer func() { matcher.find = find }()
	ctx = ContextWithRegexLimits(context.Background(), RegexLimits{MaxInputLength: 4096, MaxExecTime: time.Millisecond})
	if _, _, err := matchRegex(ctx, &cache, ComparatorASCIICaseMap, pattern, strings.Repeat("a", 2048)); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("cached matcher ignored context deadline: %v", err)
	}
}
