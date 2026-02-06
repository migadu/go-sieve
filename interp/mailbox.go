package interp

import (
	"context"
)

// MailboxChecker is an interface that can be implemented to check mailbox existence
// If not implemented, mailboxexists will always return true (optimistic behavior)
type MailboxChecker interface {
	// MailboxExists checks if a mailbox exists and the user can deliver to it
	MailboxExists(ctx context.Context, mailbox string) (bool, error)
}

// MailboxCreator is an interface that can be implemented to create mailboxes
// If not implemented, :create will be a no-op (mailbox creation deferred to delivery)
type MailboxCreator interface {
	// CreateMailbox creates a mailbox if it doesn't exist
	CreateMailbox(ctx context.Context, mailbox string) error
}

// MailboxExistsTest tests if all specified mailboxes exist
type MailboxExistsTest struct {
	Mailboxes []string
}

func (m MailboxExistsTest) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	for _, mailbox := range m.Mailboxes {
		mailbox = expandVars(d, mailbox)

		// Check if the policy implements MailboxChecker
		if checker, ok := d.Policy.(MailboxChecker); ok {
			exists, err := checker.MailboxExists(ctx, mailbox)
			if err != nil {
				return false, err
			}
			if !exists {
				return false, nil
			}
		}
		// If MailboxChecker is not implemented, assume mailbox exists (optimistic)
		// This is consistent with how the Sieve script will typically be used
	}
	return true, nil
}
