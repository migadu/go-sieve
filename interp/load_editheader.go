package interp

import (
	"github.com/migadu/go-sieve/parser"
)

// loadAddHeader loads the addheader command
// Usage: "addheader" [":last"] <field-name: string> <value: string>
func loadAddHeader(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension("editheader") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'editheader'")
	}

	cmd := CmdAddHeader{}

	err := LoadSpec(s, &Spec{
		Tags: map[string]SpecTag{
			"last": {
				NeedsValue: false,
				MatchBool: func() {
					cmd.Last = true
				},
			},
		},
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.FieldName = val[0]
				},
			},
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Value = val[0]
				},
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)

	if err != nil {
		return nil, err
	}

	return cmd, nil
}

// loadDeleteHeader loads the deleteheader command
// Usage: "deleteheader" [":index" <fieldno: number> [":last"]]
//
//	[COMPARATOR] [MATCH-TYPE]
//	<field-name: string>
//	[<value-patterns: string-list>]
func loadDeleteHeader(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension("editheader") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'editheader'")
	}

	cmd := CmdDeleteHeader{
		matcherTest: newMatcherTest(),
	}

	spec := cmd.matcherTest.addSpecTags(&Spec{
		Tags: map[string]SpecTag{
			"index": {
				NeedsValue: true,
				MatchNum: func(val int) {
					cmd.Index = val
				},
			},
			"last": {
				NeedsValue: false,
				MatchBool: func() {
					cmd.Last = true
				},
			},
		},
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.FieldName = val[0]
				},
			},
			{
				Optional:    true,
				MinStrCount: 1,
				MatchStr: func(val []string) {
					cmd.ValuePatterns = val
				},
			},
		},
	})

	err := LoadSpec(s, spec, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}

	// Per RFC 5293: :last MUST only be specified with :index
	if cmd.Last && cmd.Index == 0 {
		return nil, parser.ErrorAt(pcmd.Position, ":last can only be specified with :index")
	}

	// RFC 5293 §5: "If no value-patterns are specified, then the comparator
	// and match-type options are silently ignored." An option that is wrong
	// in itself is still an error, an unrequired comparator included (RFC 5228
	// §2.7.3); how the comparator fits the match type is checked only against
	// value patterns, as in Pigeonhole.
	if len(cmd.ValuePatterns) > 0 {
		err = cmd.matcherTest.setKey(s, cmd.ValuePatterns)
	} else {
		err = cmd.matcherTest.checkOptions(s)
	}
	if err != nil {
		return nil, parser.ErrorAt(pcmd.Position, "deleteheader: %v", err)
	}

	return cmd, nil
}
