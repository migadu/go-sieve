package tests

import (
	"path/filepath"
	"testing"
)

func TestTestsuite(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "testsuite.svtest"))
}

func TestLexer(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "lexer.svtest"))
}

func TestControlIf(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "control-if.svtest"))
}

func TestControlStop(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "control-stop.svtest"))
}

func TestTestAddress(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "test-address.svtest"))
}

func TestTestAllof(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "test-allof.svtest"))
}

func TestTestAnyof(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "test-anyof.svtest"))
}

func TestTestExists(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "test-exists.svtest"))
}

func TestTestHeader(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "test-header.svtest"))
}

func TestTestSize(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "test-size.svtest"))
}
