package interp

import (
	"context"
	"fmt"

	"github.com/migadu/go-sieve/parser"
)

// CmdReject is the RFC 5429 "reject" or "ereject" action. It cancels the
// implicit keep and records the refusal; how the message is refused (a 5xx
// over SMTP/LMTP, a DSN, an MDN, or nothing at all) is the caller's decision,
// read from RuntimeData.Rejected, RejectReason and RejectExtended.
type CmdReject struct {
	Reason   string
	Extended bool // ereject
}

func (c CmdReject) Execute(_ context.Context, d *RuntimeData) error {
	// RFC 5429 §2.4: "Implementations MUST prohibit the execution of more
	// than one reject in a Sieve script."
	if d.Rejected {
		return fmt.Errorf("duplicate reject/ereject action not allowed")
	}
	d.Rejected = true
	d.RejectReason = expandVars(d, c.Reason)
	d.RejectExtended = c.Extended
	d.ImplicitKeep = false
	return nil
}

// checkRejectConflicts enforces RFC 5429 §2.4 once the script has run, so the
// order of the actions does not matter: a reject is incompatible with
// vacation (MUST), and, as in Pigeonhole, with any action that delivers the
// message (keep, fileinto, redirect; "NOT RECOMMENDED" by the RFC).
func checkRejectConflicts(d *RuntimeData) error {
	if !d.Rejected {
		return nil
	}
	conflict := ""
	switch {
	case d.Keep:
		conflict = "keep"
	case len(d.Mailboxes) > 0:
		conflict = "fileinto"
	case len(d.RedirectAddr) > 0:
		conflict = "redirect"
	case len(d.VacationResponses) > 0:
		conflict = "vacation"
	default:
		return nil
	}
	return fmt.Errorf("reject/ereject action conflicts with the %s action", conflict)
}

func loadReject(s *Script, pcmd parser.Cmd) (Cmd, error) {
	return loadRejectCmd(s, pcmd, "reject", false)
}

func loadEreject(s *Script, pcmd parser.Cmd) (Cmd, error) {
	return loadRejectCmd(s, pcmd, "ereject", true)
}

func loadRejectCmd(s *Script, pcmd parser.Cmd, ext string, extended bool) (Cmd, error) {
	if !s.RequiresExtension(ext) {
		return nil, parser.ErrorAt(pcmd.Position, "missing require '%s'", ext)
	}
	cmd := CmdReject{Extended: extended}
	err := LoadSpec(s, &Spec{
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
