package tests

import (
	"path/filepath"
	"testing"
)

// TestExtensionsDateBasic is skipped because the Dovecot test file
// has an invalid message format (missing blank line between headers and body)
func TestExtensionsDateBasic(t *testing.T) {
	t.Skip("Skipped: Dovecot test file has invalid message format")
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "extensions", "date", "basic.svtest"))
}

func TestExtensionsDateParts(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "extensions", "date", "date-parts.svtest"))
}

// TestExtensionsDateZones is partially skipped because the "Local Zone Shift" test
// depends on the system's local timezone matching specific expectations
func TestExtensionsDateZones(t *testing.T) {
	RunDovecotTestWithout(t, filepath.Join("pigeonhole", "tests", "extensions", "date", "zones.svtest"),
		[]string{"Local Zone Shift"})
}
