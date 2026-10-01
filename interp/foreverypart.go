package interp

import (
	"context"
	"errors"
)

// CmdForEveryPart runs its block once per MIME part (RFC 5703 §3). At the top
// level it visits the message itself and then every part nested in it, depth
// first; nested in another loop it visits the parts nested in that loop's
// current part, so it does nothing for a leaf.
type CmdForEveryPart struct {
	Name  string
	Block []Cmd
}

// breakSignal is what break returns. The loop it names, or the closest loop
// when it has no name, ends and swallows it; every other error, including
// ErrStop, passes through the loops unchanged.
type breakSignal struct {
	name string
}

func (b *breakSignal) Error() string {
	return "interpreter: break called"
}

func (c *CmdForEveryPart) Execute(ctx context.Context, d *RuntimeData) error {
	root, err := d.mimeTree()
	if err != nil {
		return err
	}
	var parts []*mimePart
	if cur := d.currentPart(); cur == nil {
		parts = root.subtree()
	} else {
		parts = cur.subtree()[1:]
	}

	for _, p := range parts {
		if err := ctx.Err(); err != nil {
			return err
		}
		d.partStack = append(d.partStack, p)
		err := runBlock(ctx, d, c.Block)
		d.partStack = d.partStack[:len(d.partStack)-1]
		if err != nil {
			var b *breakSignal
			if errors.As(err, &b) && (b.name == "" || b.name == c.Name) {
				return nil
			}
			return err
		}
	}
	return nil
}

func runBlock(ctx context.Context, d *RuntimeData, cmds []Cmd) error {
	for _, c := range cmds {
		if err := c.Execute(ctx, d); err != nil {
			return err
		}
	}
	return nil
}

type CmdBreak struct {
	Name string
}

func (c CmdBreak) Execute(_ context.Context, _ *RuntimeData) error {
	return &breakSignal{name: c.Name}
}

// CmdExtractText stores the current part's text in a variable (RFC 5703 §7).
// Outside a loop, which the loader already rejects, it stores "".
type CmdExtractText struct {
	Name   string
	First  int // characters to keep; -1 keeps everything
	Modify func(string) string
}

func (c CmdExtractText) Execute(ctx context.Context, d *RuntimeData) error {
	text := ""
	if p := d.currentPart(); p != nil {
		text = p.text(d, decodeInputLimit(ctx))
	}
	if c.First >= 0 {
		// RFC 5703 counts characters, not octets.
		if r := []rune(text); len(r) > c.First {
			text = string(r[:c.First])
		}
	}
	return d.SetVar(c.Name, c.Modify(text))
}
