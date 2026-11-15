package interp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/migadu/go-sieve/lexer"
	"github.com/migadu/go-sieve/parser"
)

// A real message callback can take an execution across a second boundary.
// This exercises the interpreter without a test-only clock field or hook.
type currentDateBoundaryMessage struct{ MessageStatic }

func (m currentDateBoundaryMessage) HeaderGet(key string) ([]string, error) {
	if strings.EqualFold(key, "X-Wait") {
		until := time.Now().Truncate(time.Second).Add(time.Second + 20*time.Millisecond)
		time.Sleep(time.Until(until))
		return []string{"yes"}, nil
	}
	return m.MessageStatic.HeaderGet(key)
}

func TestCurrentDateExecutionLifetimeAcrossCallback(t *testing.T) {
	script := `require ["date", "variables"];
 if currentdate :zone "+0000" :matches "iso8601" "*" { set "first" "${1}"; }
 if header :is "X-Wait" "yes" { set "waited" "yes"; }
 if currentdate :zone "+0000" :matches "iso8601" "*" { set "second" "${1}"; }
 `
	toks, err := lexer.Lex(strings.NewReader(script), &lexer.Options{MaxTokens: 5000})
	if err != nil {
		t.Fatal(err)
	}
	cmds, err := parser.Parse(lexer.NewStream(toks), &parser.Options{MaxBlockNesting: 15, MaxTestNesting: 15})
	if err != nil {
		t.Fatal(err)
	}
	s, err := LoadScript(cmds, &Options{MaxVariableCount: 128, MaxVariableNameLen: 32, MaxVariableLen: 4000}, []string{"date", "variables"})
	if err != nil {
		t.Fatal(err)
	}
	run := func(d *RuntimeData) string {
		t.Helper()
		if err := s.Execute(context.Background(), d); err != nil {
			t.Fatal(err)
		}
		if d.Variables["waited"] != "yes" {
			t.Fatal("message callback was not evaluated")
		}
		first, second := d.Variables["first"], d.Variables["second"]
		if first == "" || first != second {
			t.Fatalf("one execution read different instants across a callback: %q -> %q", first, second)
		}
		return first
	}
	d := NewRuntimeData(s, DummyPolicy{}, EnvelopeStatic{}, currentDateBoundaryMessage{})
	first := run(d)
	if got := run(d.Copy()); got != first {
		t.Fatalf("runtime copy lost execution instant: %q -> %q", first, got)
	}
	fresh := NewRuntimeData(s, DummyPolicy{}, EnvelopeStatic{}, currentDateBoundaryMessage{})
	if got := run(fresh); got == first {
		t.Fatalf("new execution reused the previous instant: %q", got)
	}
}
