package tests

import (
	"path/filepath"
	"testing"
)

func TestExtensionsBodyBasic(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "extensions", "body", "basic.svtest"))
}

func TestExtensionsBodyErrors(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "extensions", "body", "errors.svtest"))
}

func TestExtensionsBodyContent(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "extensions", "body", "content.svtest"))
}

func TestExtensionsBodyRaw(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "extensions", "body", "raw.svtest"))
}

func TestExtensionsBodyText(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "extensions", "body", "text.svtest"))
}

func TestExtensionsBodyMatchValues(t *testing.T) {
	RunDovecotTest(t, filepath.Join("pigeonhole", "tests", "extensions", "body", "match-values.svtest"))
}
