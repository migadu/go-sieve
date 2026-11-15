package tests

import (
	"context"
	"reflect"
	"testing"

	"github.com/migadu/go-sieve/interp"
)

// A delivery may equate INBOX with keep, or mailbox spellings with each other.
// Its duplicate elimination needs the latest action's flags (RFC 5232 section
// 3), which the first-occurrence Mailboxes list alone cannot determine.
func TestStoreOrderRecordsLastAction(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		mailboxes    map[string]uint64
		keep         uint64
	}{
		{"fileinto then keep", `fileinto :flags "first" "INBOX"; keep :flags "last";`, map[string]uint64{"INBOX": 1}, 2},
		{"keep then fileinto", `keep :flags "first"; fileinto :flags "last" "INBOX";`, map[string]uint64{"INBOX": 2}, 1},
		{"case variants and exact repeat", `fileinto :flags "first" "Archive"; fileinto :flags "second" "archive"; fileinto :flags "last" "Archive";`, map[string]uint64{"Archive": 3, "archive": 2}, 0},
		{"keep repeated", `keep; fileinto "Archive"; keep :flags "last";`, map[string]uint64{"Archive": 2}, 3},
		{"implicit keep has no order", `fileinto :copy "INBOX";`, map[string]uint64{"INBOX": 1}, 0},
		{"no store action", `setflag "work";`, nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := runFlags(t, `require ["fileinto", "imap4flags", "copy"]; `+tc.script)
			if !reflect.DeepEqual(d.MailboxStoreOrder, tc.mailboxes) || d.KeepStoreOrder != tc.keep {
				t.Fatalf("mailbox orders %v, keep order %d; want %v, %d", d.MailboxStoreOrder, d.KeepStoreOrder, tc.mailboxes, tc.keep)
			}
		})
	}
}

func TestStoreOrderCopyContinuesIndependently(t *testing.T) {
	d := runFlags(t, `require ["fileinto", "imap4flags"]; fileinto "INBOX"; keep;`)
	copied := d.Copy()
	if !reflect.DeepEqual(copied.MailboxStoreOrder, d.MailboxStoreOrder) || copied.KeepStoreOrder != d.KeepStoreOrder {
		t.Fatal("copy lost store order")
	}
	ctx := context.Background()
	if err := (interp.CmdFileInto{Mailbox: "inbox"}).Execute(ctx, copied); err != nil {
		t.Fatal(err)
	}
	if copied.MailboxStoreOrder["inbox"] != 3 {
		t.Fatalf("copied counter did not continue: %v", copied.MailboxStoreOrder)
	}
	if err := (interp.CmdKeep{}).Execute(ctx, copied); err != nil {
		t.Fatal(err)
	}
	if copied.KeepStoreOrder != 4 || d.KeepStoreOrder != 2 || len(d.MailboxStoreOrder) != 1 {
		t.Fatalf("copy shared store state: original %+v/%d copied %+v/%d", d.MailboxStoreOrder, d.KeepStoreOrder, copied.MailboxStoreOrder, copied.KeepStoreOrder)
	}
	if err := (interp.CmdFileInto{Mailbox: "INBOX"}).Execute(ctx, d); err != nil {
		t.Fatal(err)
	}
	if d.MailboxStoreOrder["INBOX"] != 3 || copied.MailboxStoreOrder["INBOX"] != 1 {
		t.Fatalf("original counter/map not independent: %v, %v", d.MailboxStoreOrder, copied.MailboxStoreOrder)
	}
}
