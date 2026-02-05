package interp

import (
	"fmt"
	"strings"

	"github.com/foxcpp/go-sieve/parser"
)

// loadDateTest loads the "date" test as defined in RFC 5260.
// The date test has the following syntax:
//
//	date [<":zone" <time-zone: string>> / ":originalzone"]
//	     [COMPARATOR] [MATCH-TYPE]
//	     <header-name: string> <date-part: string> <key-list: string-list>
func loadDateTest(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("date") {
		return nil, fmt.Errorf("missing require 'date'")
	}

	loaded := DateTest{
		matcherTest: newMatcherTest(),
	}

	var key []string
	var zoneCnt int

	spec := loaded.addSpecTags(&Spec{
		Tags: map[string]SpecTag{
			"zone": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					loaded.Zone = val[0]
					zoneCnt++
				},
			},
			"originalzone": {
				MatchBool: func() {
					loaded.OriginalZone = true
					zoneCnt++
				},
			},
			"index": {
				NeedsValue: true,
				MatchNum: func(val int) {
					loaded.Index = val
				},
			},
			"last": {
				MatchBool: func() {
					loaded.Last = true
				},
			},
		},
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					loaded.Header = val[0]
				},
			},
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					loaded.DatePart = DatePart(strings.ToLower(val[0]))
				},
			},
			{
				MinStrCount: 1,
				MatchStr: func(val []string) {
					key = val
				},
			},
		},
	})

	err := LoadSpec(s, spec, test.Position, test.Args, test.Tests, nil)
	if err != nil {
		return nil, err
	}

	// Validate zone arguments
	if zoneCnt > 1 {
		return nil, fmt.Errorf("date: cannot specify both :zone and :originalzone")
	}

	// Validate zone format if specified
	if loaded.Zone != "" {
		if _, err := parseZoneOffset(loaded.Zone); err != nil {
			return nil, fmt.Errorf("date: %v", err)
		}
	}

	// Validate date-part
	if _, ok := ValidDateParts[loaded.DatePart]; !ok {
		return nil, fmt.Errorf("date: invalid date-part: %s", loaded.DatePart)
	}

	// Validate :index and :last usage
	if loaded.Last && loaded.Index == 0 {
		return nil, fmt.Errorf("date: :last requires :index")
	}
	if loaded.Index > 0 && !s.RequiresExtension("index") {
		return nil, fmt.Errorf("date: missing require 'index' for :index argument")
	}

	if err := loaded.setKey(s, key); err != nil {
		return nil, err
	}

	return loaded, nil
}

// loadCurrentDateTest loads the "currentdate" test as defined in RFC 5260.
// The currentdate test has the following syntax:
//
//	currentdate [":zone" <time-zone: string>]
//	            [COMPARATOR] [MATCH-TYPE]
//	            <date-part: string> <key-list: string-list>
func loadCurrentDateTest(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("date") {
		return nil, fmt.Errorf("missing require 'date'")
	}

	loaded := CurrentDateTest{
		matcherTest: newMatcherTest(),
	}

	var key []string

	spec := loaded.addSpecTags(&Spec{
		Tags: map[string]SpecTag{
			"zone": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					loaded.Zone = val[0]
				},
			},
		},
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					loaded.DatePart = DatePart(strings.ToLower(val[0]))
				},
			},
			{
				MinStrCount: 1,
				MatchStr: func(val []string) {
					key = val
				},
			},
		},
	})

	err := LoadSpec(s, spec, test.Position, test.Args, test.Tests, nil)
	if err != nil {
		return nil, err
	}

	// Validate zone format if specified
	if loaded.Zone != "" {
		if _, err := parseZoneOffset(loaded.Zone); err != nil {
			return nil, fmt.Errorf("currentdate: %v", err)
		}
	}

	// Validate date-part
	if _, ok := ValidDateParts[loaded.DatePart]; !ok {
		return nil, fmt.Errorf("currentdate: invalid date-part: %s", loaded.DatePart)
	}

	if err := loaded.setKey(s, key); err != nil {
		return nil, err
	}

	return loaded, nil
}
