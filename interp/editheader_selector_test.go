package interp

import (
	"net/textproto"
	"reflect"
	"testing"
)

// HeaderEdit predates ByValue: callers could already select one nonempty value.
// The explicit marker adds empty-value selection without changing those edits,
// unmarked delete-all edits, or the precedence of an index over either form.
func TestHeaderEditDeleteSelectorCompatibility(t *testing.T) {
	cases := []struct {
		name   string
		values []string
		edit   HeaderEdit
		want   []string
	}{
		{"legacy value selector", []string{"one", "two", "one"},
			HeaderEdit{Value: "one"}, []string{"two", "one"}},
		{"explicit empty value selector", []string{"", "keep", ""},
			HeaderEdit{ByValue: true}, []string{"keep", ""}},
		{"unmarked empty value deletes all", []string{"", "keep", ""},
			HeaderEdit{}, nil},
		{"index precedes legacy value", []string{"one", "two", "one"},
			HeaderEdit{Index: 2, Value: "one"}, []string{"one", "one"}},
		{"last index precedes explicit value", []string{"", "keep", "last"},
			HeaderEdit{Index: 1, Last: true, ByValue: true}, []string{"", "keep"}},
		{"missing index does not fall back to value", []string{"one", "two"},
			HeaderEdit{Index: 3, Value: "one", ByValue: true}, []string{"one", "two"}},
		{"missing value leaves fields", []string{"one", "two"},
			HeaderEdit{Value: "absent", ByValue: true}, []string{"one", "two"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.edit.Action, tc.edit.FieldName = "delete", "X-Foo"
			d := &RuntimeData{
				Msg:         MessageStatic{Header: textproto.MIMEHeader{"X-Foo": tc.values}},
				HeaderEdits: []HeaderEdit{tc.edit},
			}
			got, err := GetHeaderWithEdits(d, "X-Foo")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("fields left = %q, want %q", got, tc.want)
			}
		})
	}
}
