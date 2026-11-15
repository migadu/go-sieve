package interp

import (
	"github.com/migadu/go-sieve/lexer"
	"github.com/migadu/go-sieve/parser"
)

// fieldIndex holds the arguments the index extension adds to the header,
// address and date tests (RFC 5260 §6):
//
//	[":index" <fieldno: number> [":last"]]
type fieldIndex struct {
	Index int  // the field tested, counted from 1; 0 tests every field
	Last  bool // count Index backwards from the last field

	indexed bool // :index was given
}

func (f *fieldIndex) addIndexTags(tags map[string]SpecTag) {
	tags["index"] = SpecTag{
		NeedsValue: true,
		MatchNum: func(val int) {
			f.Index = val
			f.indexed = true
		},
	}
	tags["last"] = SpecTag{
		MatchBool: func() {
			f.Last = true
		},
	}
}

func (f *fieldIndex) validateIndex(s *Script, pos lexer.Position) error {
	if !f.indexed && !f.Last {
		return nil
	}
	if !s.RequiresExtension("index") {
		return parser.ErrorAt(pos, "missing require 'index'")
	}
	if !f.indexed {
		return parser.ErrorAt(pos, ":last requires :index")
	}
	// Fields are counted "beginning at 1, the first named header field".
	if f.Index < 1 {
		return parser.ErrorAt(pos, ":index must be 1 or more")
	}
	return nil
}

// selectFields returns the fields of each named header a test examines: all
// of them, or with :index only the one it selects. get returns the fields of
// one header. RFC 5260 §6: "If more than one header field name is specified,
// all the named header fields are counted in the order specified by the
// header-list."
func (f fieldIndex) selectFields(names []string, get func(name string) ([]string, error)) ([][]string, error) {
	byName := make([][]string, len(names))
	total := 0
	for i, name := range names {
		values, err := get(name)
		if err != nil {
			return nil, err
		}
		byName[i] = values
		total += len(values)
	}
	if f.Index == 0 {
		return byName, nil
	}

	// pos is the selected field's place among all the fields, from 0.
	pos := f.Index - 1
	if f.Last {
		pos = total - f.Index
	}
	for i, values := range byName {
		if pos >= 0 && pos < len(values) {
			byName[i] = values[pos : pos+1]
		} else {
			byName[i] = nil
		}
		pos -= len(values)
	}
	return byName, nil
}
