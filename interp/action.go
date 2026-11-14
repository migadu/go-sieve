package interp

import (
	"context"
	"fmt"
)

type CmdStop struct{}

func (c CmdStop) Execute(_ context.Context, _ *RuntimeData) error {
	return ErrStop
}

type CmdFileInto struct {
	Mailbox string
	Flags   Flags
	Copy    bool // RFC3894 - :copy modifier
	Create  bool // RFC5490 - :create modifier (mailbox extension)
}

func (c CmdFileInto) Execute(_ context.Context, d *RuntimeData) error {
	mailbox := expandVars(d, c.Mailbox)
	// A mailbox filed into again still gets one copy, and RFC 5232 §3: "the
	// last flag list value MUST win".
	if d.MailboxFlags == nil {
		d.MailboxFlags = make(map[string][]string)
	}
	d.MailboxFlags[mailbox] = copyFlags(d, c.Flags)

	found := false
	for _, m := range d.Mailboxes {
		if m == mailbox {
			found = true
		}
	}
	if found {
		return nil
	}
	d.Mailboxes = append(d.Mailboxes, mailbox)

	// RFC 5490: Track mailboxes that should be created
	if c.Create {
		createFound := false
		for _, m := range d.MailboxesCreate {
			if m == mailbox {
				createFound = true
				break
			}
		}
		if !createFound {
			d.MailboxesCreate = append(d.MailboxesCreate, mailbox)
		}
	}

	// RFC3894: If :copy is specified, do not set ImplicitKeep to false
	if !c.Copy {
		d.ImplicitKeep = false
	}
	return nil
}

// copyFlags returns the flags the copy a keep or fileinto stores is given (RFC
// 5232 §5): "The copy of the message filed into the mailbox will have only flags
// listed after the :flags tagged argument", and without :flags (§3) "the current
// value of the internal variable is used instead". Neither changes the internal
// variable, d.Flags.
func copyFlags(d *RuntimeData, flags Flags) []string {
	if flags != nil {
		return canonicalFlags(expandVarsList(d, flags), nil, d.FlagAliases)
	}
	return append([]string(nil), d.Flags...)
}

type CmdRedirect struct {
	Addr string
	Copy bool // RFC3894 - :copy modifier
}

func (c CmdRedirect) Execute(ctx context.Context, d *RuntimeData) error {
	addr := expandVars(d, c.Addr)

	ok, err := d.Policy.RedirectAllowed(ctx, d, addr)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	d.RedirectAddr = append(d.RedirectAddr, addr)

	// RFC3894: If :copy is specified, do not set ImplicitKeep to false
	if !c.Copy {
		d.ImplicitKeep = false
	}

	if len(d.RedirectAddr) > d.Script.opts.MaxRedirects {
		return fmt.Errorf("too many actions")
	}
	return nil
}

type CmdKeep struct {
	Flags Flags
}

func (c CmdKeep) Execute(_ context.Context, d *RuntimeData) error {
	d.Keep = true
	// keep is a non-terminating action - it does NOT cancel implicit keep
	// Kept twice, the message still gets one copy, with the last flag list
	// (RFC 5232 §3).
	d.KeepFlags = copyFlags(d, c.Flags)
	return nil
}

type CmdDiscard struct{}

func (c CmdDiscard) Execute(_ context.Context, d *RuntimeData) error {
	d.ImplicitKeep = false
	return nil
}

type CmdSetFlag struct {
	Flags Flags
}

func (c CmdSetFlag) Execute(_ context.Context, d *RuntimeData) error {
	if c.Flags != nil {
		d.Flags = canonicalFlags(expandVarsList(d, c.Flags), nil, d.FlagAliases)
	}
	return nil
}

type CmdAddFlag struct {
	Flags Flags
}

func (c CmdAddFlag) Execute(_ context.Context, d *RuntimeData) error {
	if c.Flags != nil {
		flags := expandVarsList(d, c.Flags)

		if d.Flags == nil {
			d.Flags = make([]string, len(flags))
			copy(d.Flags, flags)
		} else {
			// Use canonicalFlags to remove duplicates
			d.Flags = canonicalFlags(append(d.Flags, flags...), nil, d.FlagAliases)
		}
	}
	return nil
}

type CmdRemoveFlag struct {
	Flags Flags
}

func (c CmdRemoveFlag) Execute(_ context.Context, d *RuntimeData) error {
	if c.Flags != nil {
		// Use canonicalFlags to remove duplicates
		d.Flags = canonicalFlags(d.Flags, expandVarsList(d, c.Flags), d.FlagAliases)
	}
	return nil
}
