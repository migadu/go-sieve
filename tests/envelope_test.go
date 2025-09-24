package tests

import (
	"path/filepath"
	"testing"
)

func TestExtensionsEnvelope(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "extensions", "envelope.svtest"))
}
