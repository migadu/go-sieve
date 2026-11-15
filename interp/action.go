package interp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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
	d.storeOrder++
	if d.MailboxStoreOrder == nil {
		d.MailboxStoreOrder = make(map[string]uint64)
	}
	d.MailboxStoreOrder[mailbox] = d.storeOrder

	// A fileinto naming a mailbox already filed into is performed all the same,
	// so its :create and its cancelling of the implicit keep apply; only the
	// message is not stored there twice (RFC 5228 §2.10.3).

	// RFC3894: If :copy is specified, do not set ImplicitKeep to false
	if !c.Copy {
		d.ImplicitKeep = false
	}

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

	for _, m := range d.Mailboxes {
		if m == mailbox {
			return nil
		}
	}
	d.Mailboxes = append(d.Mailboxes, mailbox)
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
	d.storeOrder++
	d.KeepStoreOrder = d.storeOrder
	return nil
}

type CmdDiscard struct{}

func (c CmdDiscard) Execute(_ context.Context, d *RuntimeData) error {
	d.ImplicitKeep = false
	return nil
}

type CmdSetFlag struct {
	Variable string // the variable named (RFC 5232 §3), "" for the internal one
	Flags    Flags
}

func (c CmdSetFlag) Execute(_ context.Context, d *RuntimeData) error {
	return d.setFlagVar(c.Variable, canonicalFlags(expandVarsList(d, c.Flags), nil, d.FlagAliases))
}

type CmdAddFlag struct {
	Variable string // the variable named (RFC 5232 §3), "" for the internal one
	Flags    Flags
}

func (c CmdAddFlag) Execute(_ context.Context, d *RuntimeData) error {
	flags, err := d.flagVar(c.Variable)
	if err != nil {
		return err
	}
	// Every addflag canonicalises: a variable can expand to a space-separated
	// list of flags (RFC 5232 §2), and canonicalFlags also lower-cases,
	// applies aliases and removes duplicates.
	return d.setFlagVar(c.Variable, canonicalFlags(append(flags, expandVarsList(d, c.Flags)...), nil, d.FlagAliases))
}

type CmdRemoveFlag struct {
	Variable string // the variable named (RFC 5232 §3), "" for the internal one
	Flags    Flags
}

func (c CmdRemoveFlag) Execute(_ context.Context, d *RuntimeData) error {
	flags, err := d.flagVar(c.Variable)
	if err != nil {
		return err
	}
	return d.setFlagVar(c.Variable, canonicalFlags(flags, expandVarsList(d, c.Flags), d.FlagAliases))
}

// flagVar returns the flags of the variable an imap4flags action or test names,
// or of the internal variable for "".
func (d *RuntimeData) flagVar(name string) ([]string, error) {
	if name == "" {
		return d.Flags, nil
	}
	if n, err := strconv.Atoi(name); err == nil && n >= 0 {
		return []string{d.MatchVariable(n)}, nil
	}
	value, err := d.Var(name)
	if err != nil {
		return nil, err
	}
	return []string{value}, nil
}

// setFlagVar stores flags in the variable an imap4flags action names, or in the
// internal variable for "". RFC 5232 §3: "On variable substitution, a set of
// flags is represented as a string containing a space-separated list of flag
// names."
func (d *RuntimeData) setFlagVar(name string, flags Flags) error {
	if name == "" {
		d.Flags = flags
		return nil
	}
	return d.SetVar(name, strings.Join(flags, " "))
}
