package interp

import (
	"fmt"

	"github.com/foxcpp/go-sieve/parser"
)

func loadAddressTest(s *Script, test parser.Test) (Test, error) {
	loaded := AddressTest{
		matcherTest: newMatcherTest(),
		AddressPart: All,
	}
	var key []string
	var useSubaddress bool
	err := LoadSpec(s, loaded.addSpecTags(&Spec{
		Tags: map[string]SpecTag{
			"all": {
				MatchBool: func() {
					loaded.AddressPart = All
					loaded.AddressPartCnt++
				},
			},
			"localpart": {
				MatchBool: func() {
					loaded.AddressPart = LocalPart
					loaded.AddressPartCnt++
				},
			},
			"domain": {
				MatchBool: func() {
					loaded.AddressPart = Domain
					loaded.AddressPartCnt++
				},
			},
			// RFC 5233 subaddress extension
			"user": {
				MatchBool: func() {
					loaded.AddressPart = User
					loaded.AddressPartCnt++
					useSubaddress = true
				},
			},
			"detail": {
				MatchBool: func() {
					loaded.AddressPart = Detail
					loaded.AddressPartCnt++
					useSubaddress = true
				},
			},
		},
		Pos: []SpecPosArg{
			{
				MatchStr: func(val []string) {
					loaded.Header = val
				},
				MinStrCount: 1,
			},
			{
				MatchStr: func(val []string) {
					key = val
				},
				MinStrCount: 1,
			},
		},
	}), test.Position, test.Args, test.Tests, nil)
	if err != nil {
		return nil, err
	}

	if err := loaded.setKey(s, key); err != nil {
		return nil, err
	}

	// Check for duplicate address parts
	if loaded.AddressPartCnt > 1 {
		return nil, fmt.Errorf("multiple address-parts are not allowed")
	}

	// Check for require "subaddress" when :user or :detail is used
	if useSubaddress && !s.RequiresExtension("subaddress") {
		return nil, parser.ErrorAt(test.Position, "missing require 'subaddress'")
	}

	return loaded, nil
}

func loadAllOfTest(s *Script, test parser.Test) (Test, error) {
	loaded := AllOfTest{}
	err := LoadSpec(s, &Spec{
		AddTest: func(t Test) {
			loaded.Tests = append(loaded.Tests, t)
		},
		MultipleTests: true,
	}, test.Position, test.Args, test.Tests, nil)
	return loaded, err
}

func loadAnyOfTest(s *Script, test parser.Test) (Test, error) {
	loaded := AnyOfTest{}
	err := LoadSpec(s, &Spec{
		AddTest: func(t Test) {
			loaded.Tests = append(loaded.Tests, t)
		},
		MultipleTests: true,
	}, test.Position, test.Args, test.Tests, nil)
	return loaded, err
}

func loadEnvelopeTest(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("envelope") {
		return nil, fmt.Errorf("missing require 'envelope'")
	}

	loaded := EnvelopeTest{
		matcherTest: newMatcherTest(),
		AddressPart: All,
	}
	var key []string
	var useSubaddress bool
	err := LoadSpec(s, loaded.addSpecTags(&Spec{
		Tags: map[string]SpecTag{
			"all": {
				MatchBool: func() {
					loaded.AddressPart = All
				},
			},
			"localpart": {
				MatchBool: func() {
					loaded.AddressPart = LocalPart
				},
			},
			"domain": {
				MatchBool: func() {
					loaded.AddressPart = Domain
				},
			},
			// RFC 5233 subaddress extension
			"user": {
				MatchBool: func() {
					loaded.AddressPart = User
					useSubaddress = true
				},
			},
			"detail": {
				MatchBool: func() {
					loaded.AddressPart = Detail
					useSubaddress = true
				},
			},
		},
		Pos: []SpecPosArg{
			{
				MatchStr: func(val []string) {
					loaded.Field = val
				},
				MinStrCount: 1,
			},
			{
				MatchStr: func(val []string) {
					key = val
				},
				MinStrCount: 1,
			},
		},
	}), test.Position, test.Args, test.Tests, nil)
	if err != nil {
		return nil, err
	}

	if err := loaded.setKey(s, key); err != nil {
		return nil, err
	}

	// Check for require "subaddress" when :user or :detail is used
	if useSubaddress && !s.RequiresExtension("subaddress") {
		return nil, parser.ErrorAt(test.Position, "missing require 'subaddress'")
	}

	return loaded, nil
}

func loadExistsTest(s *Script, test parser.Test) (Test, error) {
	loaded := ExistsTest{}
	err := LoadSpec(s, &Spec{
		Pos: []SpecPosArg{
			{
				MatchStr: func(val []string) {
					loaded.Fields = val
				},
				MinStrCount: 1,
			},
		},
	}, test.Position, test.Args, test.Tests, nil)
	return loaded, err
}

func loadFalseTest(s *Script, test parser.Test) (Test, error) {
	loaded := FalseTest{}
	err := LoadSpec(s, &Spec{}, test.Position, test.Args, test.Tests, nil)
	return loaded, err
}

func loadTrueTest(s *Script, test parser.Test) (Test, error) {
	loaded := TrueTest{}
	err := LoadSpec(s, &Spec{}, test.Position, test.Args, test.Tests, nil)
	return loaded, err
}

func loadHeaderTest(s *Script, test parser.Test) (Test, error) {
	loaded := HeaderTest{matcherTest: newMatcherTest()}
	var key []string
	err := LoadSpec(s, loaded.addSpecTags(&Spec{
		Pos: []SpecPosArg{
			{
				MatchStr: func(val []string) {
					loaded.Header = val
				},
				MinStrCount: 1,
			},
			{
				MatchStr: func(val []string) {
					key = val
				},
				MinStrCount: 1,
			},
		},
	}), test.Position, test.Args, test.Tests, nil)
	if err != nil {
		return nil, err
	}

	if err := loaded.setKey(s, key); err != nil {
		return nil, err
	}

	// Check if regex extension is required
	if loaded.match == MatchRegex && !s.RequiresExtension("regex") {
		return nil, fmt.Errorf("missing require 'regex'")
	}

	return loaded, nil
}

func loadNotTest(s *Script, test parser.Test) (Test, error) {
	loaded := NotTest{}
	err := LoadSpec(s, &Spec{
		AddTest: func(t Test) {
			loaded.Test = t
		},
	}, test.Position, test.Args, test.Tests, nil)
	return loaded, err
}

func loadSizeTest(s *Script, test parser.Test) (Test, error) {
	loaded := SizeTest{}
	err := LoadSpec(s, &Spec{
		Tags: map[string]SpecTag{
			"under": {
				MatchBool: func() { loaded.Under = true },
			},
			"over": {
				MatchBool: func() { loaded.Over = true },
			},
		},
		Pos: []SpecPosArg{
			{
				MatchNum: func(i int) {
					loaded.Size = i
				},
			},
		},
	}, test.Position, test.Args, test.Tests, nil)
	if loaded.Under == loaded.Over {
		return nil, fmt.Errorf("loadSizeTest: either under or over is required")
	}
	return loaded, err
}
