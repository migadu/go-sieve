package interp

import (
	"github.com/foxcpp/go-sieve/parser"
)

func loadBodyTest(s *Script, ptest parser.Test) (Test, error) {
	if !s.RequiresExtension("body") {
		return nil, parser.ErrorAt(ptest.Position, "missing require 'body'")
	}

	test := &TestBody{
		matcherTest: newMatcherTest(),
	}

	spec := test.matcherTest.addSpecTags(&Spec{})

	// Track which transform is used to ensure only one is specified
	transformCount := 0

	spec.Tags["raw"] = SpecTag{
		MatchBool: func() {
			test.raw = true
			transformCount++
		},
	}
	spec.Tags["text"] = SpecTag{
		MatchBool: func() {
			test.text = true
			transformCount++
		},
	}
	spec.Tags["content"] = SpecTag{
		NeedsValue:  true,
		MinStrCount: 1,
		MatchStr: func(val []string) {
			test.content = val
			transformCount++
		},
	}

	spec.Pos = []SpecPosArg{
		{
			MinStrCount: 1,
			MatchStr: func(val []string) {
				test.matcherTest.setKey(s, val)
			},
		},
	}

	err := LoadSpec(s, spec, ptest.Position, ptest.Args, ptest.Tests, nil)
	if err != nil {
		return nil, err
	}

	if transformCount > 1 {
		return nil, parser.ErrorAt(ptest.Position, "only one of :raw, :text, or :content is allowed")
	}

	// Default to :text if no transform is specified
	if transformCount == 0 {
		test.text = true
	}

	err = test.matcherTest.setKey(s, test.matcherTest.key)
	if err != nil {
		return nil, parser.ErrorAt(ptest.Position, err.Error())
	}

	return test, nil
}
