package interp

import (
	"github.com/migadu/go-sieve/lexer"
	"github.com/migadu/go-sieve/parser"
)

// requireDovecotTestsuite refuses a vnd.dovecot.testsuite command or test
// unless the script required the extension, which loadRequire allows only in
// a testing environment. RFC 5228 section 2.10.5: "If an extension is not
// enabled with "require", implementations MUST treat it as if they did not
// support it at all."
func requireDovecotTestsuite(s *Script, pos lexer.Position, name string) error {
	if !s.RequiresExtension(DovecotTestExtension) || s.opts.T == nil {
		return parser.ErrorAt(pos, "%s: missing require '%s'", name, DovecotTestExtension)
	}
	return nil
}

// loadDovecotNoop loads a testsuite command that go-sieve has nothing to do
// for, such as test_binary_save (no binary form) or test_config_reload
// (test_config_set applies at once).
func loadDovecotNoop(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if err := requireDovecotTestsuite(s, pcmd.Position, pcmd.Id); err != nil {
		return nil, err
	}
	return CmdNoop{}, nil
}

func loadDovecotTestSet(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if err := requireDovecotTestsuite(s, pcmd.Position, pcmd.Id); err != nil {
		return nil, err
	}
	cmd := CmdDovecotTestSet{}
	err := LoadSpec(s, &Spec{
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.VariableName = val[0]
				},
				NoVariables: true,
			},
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.VariableValue = val[0]
				},
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

func loadDovecotTestFail(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if err := requireDovecotTestsuite(s, pcmd.Position, pcmd.Id); err != nil {
		return nil, err
	}
	cmd := CmdDovecotTestFail{}
	err := LoadSpec(s, &Spec{
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Message = val[0]
				},
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	cmd.At = pcmd.Position
	if err != nil {
		return nil, err
	}

	if !usedVarsAreValid(s, cmd.Message) {
		return nil, parser.ErrorAt(pcmd.Position, "invalid variable used: %v", cmd.Message)
	}

	return cmd, nil
}

func loadDovecotTest(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if err := requireDovecotTestsuite(s, pcmd.Position, pcmd.Id); err != nil {
		return nil, err
	}
	cmd := CmdDovecotTest{}
	err := LoadSpec(s, &Spec{
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.TestName = val[0]
				},
			},
		},
		AddBlock: func(cmds []Cmd) {
			cmd.Cmds = cmds
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	return cmd, err
}

func loadDovecotCompile(s *Script, test parser.Test) (Test, error) {
	if err := requireDovecotTestsuite(s, test.Position, test.Id); err != nil {
		return nil, err
	}
	loaded := TestDovecotCompile{}
	err := LoadSpec(s, &Spec{
		Pos: []SpecPosArg{
			{
				MatchStr: func(val []string) {
					loaded.ScriptPath = val[0]
				},
				MinStrCount: 1,
				MaxStrCount: 1,
			},
		},
	}, test.Position, test.Args, test.Tests, nil)
	return loaded, err
}

func loadDovecotConfigSet(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if err := requireDovecotTestsuite(s, pcmd.Position, pcmd.Id); err != nil {
		return nil, err
	}
	loaded := CmdDovecotConfigSet{}
	err := LoadSpec(s, &Spec{
		Pos: []SpecPosArg{
			{
				MatchStr: func(val []string) {
					loaded.Key = val[0]
				},
				MinStrCount: 1,
				MaxStrCount: 1,
			},
			{
				MatchStr: func(val []string) {
					loaded.Value = val[0]
				},
				MinStrCount: 1,
				MaxStrCount: 1,
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	return loaded, err
}

func loadDovecotConfigUnset(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if err := requireDovecotTestsuite(s, pcmd.Position, pcmd.Id); err != nil {
		return nil, err
	}
	loaded := CmdDovecotConfigSet{
		Unset: true,
	}
	err := LoadSpec(s, &Spec{
		Pos: []SpecPosArg{
			{
				MatchStr: func(val []string) {
					loaded.Key = val[0]
				},
				MinStrCount: 1,
				MaxStrCount: 1,
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	return loaded, err
}

func loadDovecotRun(s *Script, test parser.Test) (Test, error) {
	if err := requireDovecotTestsuite(s, test.Position, test.Id); err != nil {
		return nil, err
	}
	loaded := TestDovecotRun{}
	err := LoadSpec(s, &Spec{}, test.Position, test.Args, test.Tests, nil)
	return loaded, err
}

func loadDovecotError(s *Script, test parser.Test) (Test, error) {
	if err := requireDovecotTestsuite(s, test.Position, test.Id); err != nil {
		return nil, err
	}
	loaded := TestDovecotTestError{matcherTest: newMatcherTest()}
	err := LoadSpec(s, loaded.addSpecTags(&Spec{
		Tags: map[string]SpecTag{
			"index": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				NoVariables: true,
				MatchNum:    func(val int) {},
			},
		},
		Pos: []SpecPosArg{
			{
				MatchStr:    func(val []string) {},
				MinStrCount: 1,
			},
		},
	}), test.Position, test.Args, test.Tests, nil)
	return loaded, err
}
