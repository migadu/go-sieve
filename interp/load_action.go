package interp

import (
	"sort"
	"strings"

	"github.com/migadu/go-sieve/lexer"
	"github.com/migadu/go-sieve/parser"
)

type Flags []string

func canonicalFlags(src []string, remove Flags, aliases map[string]string) Flags {
	// This does four things
	// * Translate space delimited lists of flags into separate flags
	// * Handle flag aliases
	// * Deduplicate
	// * Sort
	// * (optionally) remove flags
	c := make(Flags, 0, len(src))
	fm := make(map[string]struct{})
	for _, fl := range src {
		for _, f := range flagNames(fl) {
			// RFC 3501: Flags are case-insensitive.
			f = strings.ToLower(f)
			if fc, ok := aliases[f]; ok {
				fm[fc] = struct{}{}
			} else {
				fm[f] = struct{}{}
			}
		}
	}
	for _, fl := range remove {
		for _, f := range flagNames(fl) {
			// RFC 3501: Flags are case-insensitive.
			f = strings.ToLower(f)
			if fc, ok := aliases[f]; ok {
				delete(fm, fc)
			} else {
				delete(fm, f)
			}
		}
	}
	for f := range fm {
		c = append(c, f)
	}
	sort.Strings(c)
	return c
}

// flagNames splits one string of a list-of-flags parameter. RFC 5232 §2: "A
// string containing a space-separated list of flag names is equivalent to a
// string list consisting of the flags.", empty strings are ignored, and
// "multiple spaces between flag names MUST be treated as a single space
// character, and leading and trailing spaces MUST be ignored."
func flagNames(list string) []string {
	return strings.FieldsFunc(list, func(r rune) bool { return r == ' ' })
}

// distinctFlags splits the flag lists in src like canonicalFlags and drops a
// flag that repeats an earlier one, but keeps each flag's spelling and order,
// as hasflag compares them with the script's comparator (RFC 5232 §4).
func distinctFlags(src []string) []string {
	var flags []string
	seen := make(map[string]struct{})
	for _, list := range src {
		for _, f := range flagNames(list) {
			// RFC 3501: Flags are case-insensitive.
			lf := strings.ToLower(f)
			if _, ok := seen[lf]; ok {
				continue
			}
			seen[lf] = struct{}{}
			flags = append(flags, f)
		}
	}
	return flags
}

func loadFileInto(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension("fileinto") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'fileinto")
	}
	cmd := CmdFileInto{}
	err := LoadSpec(s, &Spec{
		Tags: map[string]SpecTag{
			"flags": {
				NeedsValue:  true,
				MinStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Flags = canonicalFlags(val, nil, nil)
				},
			},
			"copy": {
				NeedsValue: false,
				MatchBool: func() {
					cmd.Copy = true
				},
			},
			"create": {
				NeedsValue: false,
				MatchBool: func() {
					cmd.Create = true
				},
			},
		},
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Mailbox = val[0]
				},
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}

	if !s.RequiresExtension("imap4flags") && cmd.Flags != nil {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'imap4flags")
	}

	if cmd.Copy && !s.RequiresExtension("copy") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'copy'")
	}

	if cmd.Create && !s.RequiresExtension("mailbox") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'mailbox'")
	}

	return cmd, nil
}

func loadRedirect(s *Script, pcmd parser.Cmd) (Cmd, error) {
	cmd := CmdRedirect{}
	err := LoadSpec(s, &Spec{
		Tags: map[string]SpecTag{
			"copy": {
				NeedsValue: false,
				MatchBool: func() {
					cmd.Copy = true
				},
			},
		},
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Addr = val[0]
				},
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}

	if cmd.Copy && !s.RequiresExtension("copy") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'copy'")
	}

	return cmd, nil
}

func loadKeep(s *Script, pcmd parser.Cmd) (Cmd, error) {
	cmd := CmdKeep{}
	err := LoadSpec(s, &Spec{
		Tags: map[string]SpecTag{
			"flags": {
				NeedsValue:  true,
				MinStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Flags = canonicalFlags(val, nil, nil)
				},
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}

	if !s.RequiresExtension("imap4flags") && cmd.Flags != nil {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'imap4flags")
	}

	return cmd, nil
}

func loadDiscard(s *Script, pcmd parser.Cmd) (Cmd, error) {
	cmd := CmdDiscard{}
	err := LoadSpec(s, &Spec{}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	return cmd, err
}

func loadSetFlag(s *Script, pcmd parser.Cmd) (Cmd, error) {
	variable, flags, err := loadFlagAction(s, pcmd)
	if err != nil {
		return nil, err
	}
	return CmdSetFlag{Variable: variable, Flags: flags}, nil
}

func loadAddFlag(s *Script, pcmd parser.Cmd) (Cmd, error) {
	variable, flags, err := loadFlagAction(s, pcmd)
	if err != nil {
		return nil, err
	}
	return CmdAddFlag{Variable: variable, Flags: flags}, nil
}

func loadRemoveFlag(s *Script, pcmd parser.Cmd) (Cmd, error) {
	variable, flags, err := loadFlagAction(s, pcmd)
	if err != nil {
		return nil, err
	}
	return CmdRemoveFlag{Variable: variable, Flags: flags}, nil
}

// loadFlagAction loads the arguments of setflag, addflag and removeflag (RFC
// 5232 §3): "[<variablename: string>] <list-of-flags: string-list>". The
// variable name is "" for the internal variable.
func loadFlagAction(s *Script, pcmd parser.Cmd) (string, Flags, error) {
	if !s.RequiresExtension("imap4flags") {
		return "", nil, parser.ErrorAt(pcmd.Position, "missing require 'imap4flags")
	}
	var args flagArgs
	err := LoadSpec(s, &Spec{Pos: args.spec()}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return "", nil, err
	}
	variables, flags, err := args.split(s, pcmd.Position, true)
	if err != nil {
		return "", nil, err
	}
	variable := ""
	if variables != nil {
		variable = variables[0]
	}
	return variable, canonicalFlags(flags, nil, nil), nil
}

// flagArgs collects the positional arguments of the imap4flags actions and
// hasflag: an optional variable name (RFC 5232 §3), or variable list for
// hasflag (§4), before the list of flags. Which of the two a lone argument is
// shows only once all are loaded, so split sorts them out after LoadSpec.
type flagArgs struct {
	first, second []string
}

func (a *flagArgs) spec() []SpecPosArg {
	return []SpecPosArg{
		{
			MinStrCount: 1,
			MatchStr:    func(val []string) { a.first = val },
		},
		{
			Optional:    true,
			MinStrCount: 1,
			MatchStr:    func(val []string) { a.second = val },
		},
	}
}

// split returns the variable names, lower-cased, or nil for the internal
// variable, and the list of flags. An action's variable is one name it can
// set; hasflag's are names it can read.
func (a *flagArgs) split(s *Script, pos lexer.Position, action bool) (variables, flags []string, err error) {
	if a.second == nil {
		return nil, a.first, nil
	}
	// RFC 5232 §3: "If the SIEVE interpreter doesn't support the "variables"
	// extension [VARIABLES], the presence of the variable name parameter will
	// cause a runtime error [SIEVE]." A script must require an extension to
	// use it, so it is refused when it loads, as Pigeonhole does.
	if !s.RequiresExtension("variables") {
		return nil, nil, parser.ErrorAt(pos, "a variable name needs require 'variables'")
	}
	if action && len(a.first) != 1 {
		return nil, nil, parser.ErrorAt(pos, "the variable name must be a single string")
	}
	variables = make([]string, len(a.first))
	for i, name := range a.first {
		usable, gettable := s.IsVarUsable(name)
		if !action {
			usable = gettable
		}
		if !usable {
			return nil, nil, parser.ErrorAt(pos, "%q is not a usable variable name", name)
		}
		variables[i] = strings.ToLower(name)
	}
	return variables, a.second, nil
}
