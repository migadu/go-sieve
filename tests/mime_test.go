package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/migadu/go-sieve"
)

// RFC 5703 (mime, foreverypart, extracttext) against Pigeonhole's own suite.

func TestExtensionsMimeHeader(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "extensions", "mime", "header.svtest"))
}

func TestExtensionsMimeAddress(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "extensions", "mime", "address.svtest"))
}

func TestExtensionsMimeExists(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "extensions", "mime", "exists.svtest"))
}

func TestExtensionsMimeContentHeader(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "extensions", "mime", "content-header.svtest"))
}

func TestExtensionsMimeCalendarExample(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "extensions", "mime", "calendar-example.svtest"))
}

func TestExtensionsMimeExtracttext(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "extensions", "mime", "extracttext.svtest"))
}

func TestExtensionsMimeForeverypart(t *testing.T) {
	// A copy without the include-based test; see the file's header.
	RunDovecotTest(t, filepath.Join("testdata", "mime", "foreverypart.svtest"))
}

// errors.svtest is not run: it asserts Pigeonhole's exact compile error
// counts with test_error, and go-sieve stops at the first error. Its scripts
// are compiled by TestExtensionsMimeErrorScripts instead, and the rules they
// cover one at a time by TestExtensionsMimeCompileRules.

// mimeTestExtensions is the set the scripts under test may require.
var mimeTestExtensions = []string{"mime", "foreverypart", "extracttext", "variables", "relational",
	"comparator-i;ascii-numeric", "fileinto", "editheader"}

func compileScript(t *testing.T, path string) error {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	opts := sieve.DefaultOptions()
	opts.Lexer.Filename = filepath.Base(path)
	opts.EnabledExtensions = mimeTestExtensions
	_, err = sieve.Load(bytes.NewReader(src), opts)
	return err
}

// TestExtensionsMimeErrorScripts compiles each of the suite's error scripts
// with the extensions enabled. errors.svtest only shows that they fail to
// compile, and in this harness test_script_compile compiles with no
// extensions enabled, so every require fails there whatever the script does.
func TestExtensionsMimeErrorScripts(t *testing.T) {
	dir := filepath.Join("pigeonhole", "tests", "extensions", "mime", "errors")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == "limits-include.sieve" {
			continue // needs the include extension
		}
		t.Run(e.Name(), func(t *testing.T) {
			if err := compileScript(t, filepath.Join(dir, e.Name())); err == nil {
				t.Fatalf("%s compiled; Pigeonhole rejects it", e.Name())
			}
		})
	}
}

// TestExtensionsMimeExecuteScripts compiles the suite's execute scripts, which
// execute.svtest runs through test_result_execute, a testsuite command go-sieve
// does not have.
func TestExtensionsMimeExecuteScripts(t *testing.T) {
	dir := filepath.Join("pigeonhole", "tests", "extensions", "mime", "execute")
	for _, name := range []string{"foreverypart.sieve", "mime.sieve"} {
		t.Run(name, func(t *testing.T) {
			if err := compileScript(t, filepath.Join(dir, name)); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		})
	}
}

// TestExtensionsMimeCompileRules pins the compile-time rules the error scripts
// exercise one at a time, so a failure names the rule rather than the file.
func TestExtensionsMimeCompileRules(t *testing.T) {
	cases := []struct {
		name   string
		script string
		ok     bool
	}{
		{"mime tags", `require "mime"; if header :mime :anychild :contenttype "Content-Type" "text/html" { stop; }`, true},
		{"param needs list", `require "mime"; if header :mime :param ["filename"] :matches "Content-Disposition" "*.exe" { stop; }`, true},
		{"anychild without mime", `require "mime"; if header :anychild "Content-Type" "x" { stop; }`, false},
		{"type without mime", `require "mime"; if header :type "Content-Type" "x" { stop; }`, false},
		{"two mimeopts", `require "mime"; if header :mime :type :subtype "Content-Type" "x" { stop; }`, false},
		{"param on address", `require "mime"; if address :mime :param "x" "To" "x" { stop; }`, false},
		{"type on exists", `require "mime"; if exists :mime :type "To" { stop; }`, false},
		{"mime without require", `if header :mime "Content-Type" "x" { stop; }`, false},
		{"loop", `require "foreverypart"; foreverypart :name "a" { foreverypart { break :name "a"; } }`, true},
		{"break outside loop", `require "foreverypart"; break;`, false},
		{"break unknown name", `require "foreverypart"; foreverypart :name "a" { break :name "b"; }`, false},
		{"break name of sibling loop", `require "foreverypart"; foreverypart :name "a" { } foreverypart { break :name "a"; }`, false},
		{"nesting at the limit", `require "foreverypart"; foreverypart { foreverypart { foreverypart { foreverypart { stop; } } } }`, true},
		{"nesting past the limit", `require "foreverypart"; foreverypart { foreverypart { foreverypart { foreverypart { foreverypart { stop; } } } } }`, false},
		{"extracttext", `require ["foreverypart", "variables", "extracttext"]; foreverypart { extracttext :first 10 :lower "x"; }`, true},
		{"extracttext outside loop", `require ["foreverypart", "variables", "extracttext"]; extracttext "x";`, false},
		{"extracttext without variables", `require ["foreverypart", "extracttext"]; foreverypart { extracttext "x"; }`, false},
		{"extracttext without foreverypart", `require ["variables", "extracttext"]; keep;`, false},
		{"extracttext match variable", `require ["foreverypart", "variables", "extracttext"]; foreverypart { extracttext "0"; }`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := sieve.DefaultOptions()
			opts.EnabledExtensions = mimeTestExtensions
			_, err := sieve.Load(strings.NewReader(tc.script), opts)
			if tc.ok && err != nil {
				t.Fatalf("did not compile: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("compiled; want an error")
			}
		})
	}
}
