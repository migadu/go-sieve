package tests

import (
	"bytes"
	"context"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foxcpp/go-sieve"
	"github.com/foxcpp/go-sieve/interp"
)

func RunDovecotTestInline(t *testing.T, baseDir string, scriptText string) {
	opts := sieve.DefaultOptions()
	opts.Lexer.Filename = "inline"
	opts.Interp.T = t
	// Enable all extensions for Dovecot tests
	opts.EnabledExtensions = []string{
		"fileinto", "envelope", "encoded-character",
		"comparator-i;octet", "comparator-i;ascii-casemap",
		"comparator-i;ascii-numeric", "comparator-i;unicode-casemap",
		"imap4flags", "variables", "relational", "vacation", "copy", "regex",
	}

	script, err := sieve.Load(strings.NewReader(scriptText), opts)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Create data with proper envelope and message for vacation tests
	env := interp.EnvelopeStatic{
		From: "sender@example.com",
		To:   "recipient@example.com",
	}
	
	msg := interp.MessageStatic{
		Header: make(textproto.MIMEHeader),
		Size:   100,
	}
	
	data := sieve.NewRuntimeData(script, interp.DummyPolicy{}, env, msg)

	if baseDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		data.Namespace = os.DirFS(wd)
	} else {
		data.Namespace = os.DirFS(baseDir)
	}

	err = script.Execute(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
}

func RunDovecotTestWithout(t *testing.T, path string, disabledTests []string) {
	svScript, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	opts := sieve.DefaultOptions()
	opts.Lexer.Filename = filepath.Base(path)
	opts.Interp.T = t
	opts.Interp.DisabledTests = disabledTests
	// Enable all extensions for Dovecot tests
	opts.EnabledExtensions = []string{
		"fileinto", "envelope", "encoded-character",
		"comparator-i;octet", "comparator-i;ascii-casemap",
		"comparator-i;ascii-numeric", "comparator-i;unicode-casemap",
		"imap4flags", "variables", "relational", "vacation", "copy", "regex",
	}

	script, err := sieve.Load(bytes.NewReader(svScript), opts)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Empty data.
	data := sieve.NewRuntimeData(script, interp.DummyPolicy{},
		interp.EnvelopeStatic{}, interp.MessageStatic{})
	data.Namespace = os.DirFS(filepath.Dir(path))

	err = script.Execute(ctx, data)
	if err != nil {
		t.Fatal(err)
	}
}

func RunDovecotTest(t *testing.T, path string) {
	RunDovecotTestWithout(t, path, nil)
}
