package interp

import (
	"github.com/foxcpp/go-sieve/parser"
)

// loadMailboxExistsTest loads the mailboxexists test
// Usage: mailboxexists <mailbox-names: string-list>
func loadMailboxExistsTest(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("mailbox") {
		return nil, parser.ErrorAt(test.Position, "missing require 'mailbox'")
	}

	t := MailboxExistsTest{}

	err := LoadSpec(s, &Spec{
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MatchStr: func(val []string) {
					t.Mailboxes = val
				},
			},
		},
	}, test.Position, test.Args, nil, nil)

	if err != nil {
		return nil, err
	}

	return t, nil
}
