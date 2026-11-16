package interp

import (
	"context"
	"errors"
	"net/textproto"
	"reflect"
	"testing"
)

// Cancellation after one matching occurrence must roll back only this command,
// retaining edits staged by earlier actions and stopping before another match.
func TestDeleteHeaderRollsBackOnMidCommandCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	c := CmdDeleteHeader{
		FieldName: "X-Foo", ValuePatterns: []string{"*"},
		matcherTest: matcherTest{
			comparator: DefaultComparator, match: MatchMatches, key: []string{"*"},
			keyCompiled: []CompiledMatcher{func(context.Context, string) (bool, []string, error) {
				calls++
				cancel()
				return true, []string{"first", "first"}, nil
			}},
		},
	}
	prior := []HeaderEdit{{Action: "add", FieldName: "X-Prior", Value: "saved"}}
	d := NewRuntimeData(&Script{}, DummyPolicy{}, EnvelopeStatic{}, MessageStatic{
		Header: textproto.MIMEHeader{"X-Foo": {"first", "second", "third"}},
	})
	d.HeaderEdits = append([]HeaderEdit(nil), prior...)
	if err := c.Execute(ctx, d); !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute error = %v, want cancellation", err)
	}
	if calls != 1 {
		t.Fatalf("matcher ran %d times after cancellation, want once", calls)
	}
	if !reflect.DeepEqual(d.HeaderEdits, prior) {
		t.Fatalf("edits = %+v, want earlier edits %+v", d.HeaderEdits, prior)
	}
	got, err := GetHeaderWithEdits(d, "X-Foo")
	if err != nil || !reflect.DeepEqual(got, []string{"first", "second", "third"}) {
		t.Fatalf("headers = %q, error %v, want all originals", got, err)
	}
}

func TestDeleteHeaderDirectConstructionUsesDefaultComparator(t *testing.T) {
	for _, tc := range []struct {
		name         string
		index        int
		last         bool
		values, want []string
	}{
		{"all", 0, false, []string{"drop", "KEEP", "Drop"}, []string{"KEEP"}},
		{"index", 1, false, []string{"Drop", "drop"}, []string{"drop"}},
		{"last", 1, true, []string{"drop", "Drop"}, []string{"drop"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := NewRuntimeData(&Script{}, DummyPolicy{}, EnvelopeStatic{}, MessageStatic{
				Header: textproto.MIMEHeader{"X-Foo": tc.values},
			})
			c := CmdDeleteHeader{FieldName: "X-Foo", ValuePatterns: []string{"missing", "DROP"}, Index: tc.index, Last: tc.last}
			if err := c.Execute(context.Background(), d); err != nil {
				t.Fatal(err)
			}
			got, err := GetHeaderWithEdits(d, "X-Foo")
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("headers = %q, error %v, want %q", got, err, tc.want)
			}
		})
	}
}

func TestDeleteHeaderCompiledAndCachedMatchersHonorInputLimit(t *testing.T) {
	for _, match := range []Match{MatchMatches, MatchRegex} {
		t.Run(string(match), func(t *testing.T) {
			s := &Script{}
			m := newMatcherTest()
			m.match = match
			pattern := "abc"
			if match == MatchRegex {
				pattern = "^abc$"
				s.extensions = map[string]struct{}{"regex": {}}
			}
			if err := m.setKey(s, []string{pattern}); err != nil {
				t.Fatal(err)
			}
			c := CmdDeleteHeader{FieldName: "X-Foo", ValuePatterns: []string{pattern}, matcherTest: m}
			d := NewRuntimeData(s, DummyPolicy{}, EnvelopeStatic{}, MessageStatic{
				Header: textproto.MIMEHeader{"X-Foo": {"abcX", "abcY"}},
			})
			ctx := ContextWithRegexLimits(context.Background(), RegexLimits{MaxInputLength: 3})
			if err := c.Execute(ctx, d); err != nil {
				t.Fatal(err)
			}
			got, err := GetHeaderWithEdits(d, "X-Foo")
			if err != nil || len(got) != 0 {
				t.Fatalf("headers = %q, error %v, want input-limited matches to delete both", got, err)
			}
			if match == MatchRegex && (len(d.regexes.matchers) != 1 || d.regexes.bytes > regexCacheMaxBytes || d.regexes.size > regexCacheMaxSize) {
				t.Fatalf("regex cache = %+v, want one bounded key reused for both values", d.regexes)
			}
		})
	}
}
