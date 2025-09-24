package interp

import (
	"github.com/foxcpp/go-sieve/parser"
)

// loadVacation loads the vacation command as defined in RFC 5230.
// The vacation command has the following syntax:
//
//	vacation [":days" number] [":subject" string]
//	         [":from" string] [":addresses" string-list]
//	         [":mime"] [":handle" string] <reason: string>
func loadVacation(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension("vacation") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'vacation'")
	}

	cmd := CmdVacation{
		Days: 7, // Default value as per RFC 5230
	}
	err := LoadSpec(s, &Spec{
		Tags: map[string]SpecTag{
			"days": {
				NeedsValue: true,
				MatchNum: func(val int) {
					cmd.Days = val
				},
			},
			"subject": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Subject = val[0]
				},
			},
			"from": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.From = val[0]
				},
			},
			"addresses": {
				NeedsValue:  true,
				MinStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Addresses = val
				},
			},
			"mime": {
				NeedsValue: false,
				MatchBool: func() {
					cmd.Mime = true
				},
			},
			"handle": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Handle = val[0]
				},
			},
		},
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Reason = val[0]
				},
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}

	return cmd, nil
}
