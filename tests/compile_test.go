package tests

import (
	"path/filepath"
	"testing"
)

func TestCompile(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "compile", "compile.svtest"))
}

// go-sieve has more simple error handling, but we still run
// tests to check whether any invalid scripts are not loaded as valid.

func TestCompileErrors(t *testing.T) {
	// Stripped test_error calls from errors.svtest.
	RunDovecotTestInline(t, filepath.Join("pigeonhole", "tests", "compile"), `
require "vnd.dovecot.testsuite";
test "Lexer errors" {
	if test_script_compile "errors/lexer.sieve" {
		test_fail "compile should have failed.";
	}
}
test "Parser errors" {
	if test_script_compile "errors/parser.sieve" {
		test_fail "compile should have failed.";
	}
}
test "Header errors" {
	if test_script_compile "errors/header.sieve" {
		test_fail "compile should have failed.";
	}
}
test "Address errors" {
	if test_script_compile "errors/address.sieve" {
		test_fail "compile should have failed.";
	}
}
test "If errors" {
	if test_script_compile "errors/if.sieve" {
		test_fail "compile should have failed.";
	}
}
test "Require errors" {
	if test_script_compile "errors/require.sieve" {
		test_fail "compile should have failed.";
	}
}
test "Size errors" {
	if test_script_compile "errors/size.sieve" {
		test_fail "compile should have failed.";
	}
}
test "Envelope errors" {
	if test_script_compile "errors/envelope.sieve" {
		test_fail "compile should have failed.";
	}
}
test "Stop errors" {
	if test_script_compile "errors/stop.sieve" {
		test_fail "compile should have failed.";
	}
}
test "Keep errors" {
	if test_script_compile "errors/keep.sieve" {
		test_fail "compile should have failed.";
	}
}
test "Fileinto errors" {
	if test_script_compile "errors/fileinto.sieve" {
		test_fail "compile should have failed.";
	}
}
test "COMPARATOR errors" {
	if test_script_compile "errors/comparator.sieve" {
		test_fail "compile should have failed.";
	}
}
test "ADDRESS-PART errors" {
	if test_script_compile "errors/address-part.sieve" {
		test_fail "compile should have failed.";
	}
}
test "MATCH-TYPE errors" {
	if test_script_compile "errors/match-type.sieve" {
		test_fail "compile should have failed.";
	}
}
test "Encoded-character errors" {
	if test_script_compile "errors/encoded-character.sieve" {
		test_fail "compile should have failed.";
	}
}
test "Outgoing address errors" {
	if test_script_compile "errors/out-address.sieve" {
		test_fail "compile should have failed.";
	}
}
test "Tagged argument errors" {
	if test_script_compile "errors/tag.sieve" {
		test_fail "compile should have failed.";
	}
}
test "Typos" {
	if test_script_compile "errors/typos.sieve" {
		test_fail "compile should have failed.";
	}
}
test "Unsupported language features" {
	if test_script_compile "errors/unsupported.sieve" {
		test_fail "compile should have failed.";
	}
}
`)
	//RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "compile", "errors.svtest"))
}

func TestCompileRecover(t *testing.T) {
	RunDovecotTestInline(t, filepath.Join("pigeonhole", "tests", "compile"), `
require "vnd.dovecot.testsuite";
test "Missing semicolons" {
	if test_script_compile "recover/commands-semicolon.sieve" {
		test_fail "compile should have failed.";
	}
}
test "Missing semicolon at end of block" {
	if test_script_compile "recover/commands-endblock.sieve" {
		test_fail "compile should have failed.";
	}
}
test "Spurious comma at end of test list" {
	if test_script_compile "recover/tests-endcomma.sieve" {
		test_fail "compile should have failed.";
	}
}
`)
}

func TestCompileWarnings(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "compile", "warnings.svtest"))
}
