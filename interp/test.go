package interp

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/emersion/go-message/mail"
)

// stripRFC2822Comments removes RFC 2822 comments (text in parentheses) from address strings
// This allows parsing addresses like "tss(no spam)@fi.iki" -> "tss@fi.iki" 
func stripRFC2822Comments(addr string) string {
	// Simple regex to remove text in parentheses
	// This is a basic implementation - RFC 2822 comment parsing is complex
	// but this handles the common case in the test
	commentRegex := regexp.MustCompile(`\([^)]*\)`)
	return strings.TrimSpace(commentRegex.ReplaceAllString(addr, ""))
}

type Test interface {
	Check(ctx context.Context, d *RuntimeData) (bool, error)
}

type AddressTest struct {
	matcherTest

	AddressPart    AddressPart
	AddressPartCnt int // Counter to detect duplicate address parts
	Header         []string
}

var allowedAddrHeaders = map[string]struct{}{
	// Required by Sieve.
	"from":        {},
	"to":          {},
	"cc":          {},
	"bcc":         {},
	"sender":      {},
	"resent-from": {},
	"resent-to":   {},
	// Misc (RFC 2822)
	"reply-to":        {},
	"resent-reply-to": {},
	"resent-sender":   {},
	"resent-cc":       {},
	"resent-bcc":      {},
	// Non-standard (RFC 2076, draft-palme-mailext-headers-08.txt)
	"for-approval":                       {},
	"for-handling":                       {},
	"for-comment":                        {},
	"apparently-to":                      {},
	"errors-to":                          {},
	"delivered-to":                       {},
	"return-receipt-to":                  {},
	"x-admin":                            {},
	"read-receipt-to":                    {},
	"x-confirm-reading-to":               {},
	"return-receipt-requested":           {},
	"registered-mail-reply-requested-by": {},
	"mail-followup-to":                   {},
	"mail-reply-to":                      {},
	"abuse-reports-to":                   {},
	"x-complaints-to":                    {},
	"x-report-abuse-to":                  {},
	"x-beenthere":                        {},
	"x-original-from":                    {},
	"x-original-to":                      {},
}

func (a AddressTest) Check(_ context.Context, d *RuntimeData) (bool, error) {
	entryCount := uint64(0)
	for _, hdr := range a.Header {
		hdr = strings.ToLower(hdr)
		hdr = expandVars(d, hdr)

		if _, ok := allowedAddrHeaders[hdr]; !ok {
			continue
		}

		values, err := d.Msg.HeaderGet(hdr)
		if err != nil {
			return false, err
		}

		// Handle case where header exists but has no values (empty header)  
		if len(values) == 0 {
			if a.isCount() {
				// No addresses to count for this header
				continue
			}
			
			// Try to match against empty string for empty header
			ok, err := testAddress(d, a.matcherTest, a.AddressPart, "")
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
			continue
		}

		for _, value := range values {
			// Strip RFC 2822 comments before parsing
			cleanValue := stripRFC2822Comments(value)
			
			// Check for invalid angle bracket usage (bare angle brackets without display name)
			// Pattern like "<email@domain.com>" without preceding display name is invalid
			trimmed := strings.TrimSpace(cleanValue)
			hasBareAngleBrackets := strings.HasPrefix(trimmed, "<") && strings.HasSuffix(trimmed, ">") && 
			   strings.Count(trimmed, "<") == 1 && strings.Count(trimmed, ">") == 1
			
			if hasBareAngleBrackets {
				// Bare angle brackets are invalid for address parsing, but for :all we can match literally
				if a.isCount() {
					// For count mode, invalid addresses don't count
					continue
				}
				
				// Try literal matching against the invalid address format
				ok, err := testAddress(d, a.matcherTest, a.AddressPart, cleanValue)
				if err != nil {
					return false, err
				}
				if ok {
					return true, nil
				}
				continue
			}
			
			addrList, err := mail.ParseAddressList(cleanValue)
			if err != nil {
				// If parsing fails, try matching against the literal header value
				if a.isCount() {
					// For count mode, non-parseable addresses don't count
					continue
				}
				
				// For failed address parsing, match against the literal value
				ok, err := testAddress(d, a.matcherTest, a.AddressPart, cleanValue)
				if err != nil {
					return false, err
				}
				if ok {
					return true, nil
				}
				continue
			}

			// Handle empty address list (empty header value)
			if len(addrList) == 0 {
				if a.isCount() {
					// No addresses to count
					continue
				}
				
				// Try to match against empty string
				ok, err := testAddress(d, a.matcherTest, a.AddressPart, "")
				if err != nil {
					return false, err
				}
				if ok {
					return true, nil
				}
				continue
			}

			for _, addr := range addrList {
				if a.isCount() {
					entryCount++
					continue
				}

				ok, err := testAddress(d, a.matcherTest, a.AddressPart, addr.Address)
				if err != nil {
					return false, err
				}
				if ok {
					return true, nil
				}
			}
		}
	}

	if a.isCount() {
		return a.countMatches(d, entryCount), nil
	}

	return false, nil
}

type AllOfTest struct {
	Tests []Test
}

func (a AllOfTest) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	for _, t := range a.Tests {
		ok, err := t.Check(ctx, d)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

type AnyOfTest struct {
	Tests []Test
}

func (a AnyOfTest) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	for _, t := range a.Tests {
		ok, err := t.Check(ctx, d)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

type EnvelopeTest struct {
	matcherTest

	AddressPart AddressPart
	Field       []string
}

func (e EnvelopeTest) Check(_ context.Context, d *RuntimeData) (bool, error) {
	entryCount := uint64(0)
	for _, field := range e.Field {
		var value string
		switch strings.ToLower(expandVars(d, field)) {
		case "from":
			value = d.Envelope.EnvelopeFrom()
		case "to":
			value = d.Envelope.EnvelopeTo()
		case "auth":
			value = d.Envelope.AuthUsername()
		default:
			return false, fmt.Errorf("envelope: unsupported envelope-part: %v", field)
		}
		
		// For envelope addresses (from/to), we need to validate them first
		// If the address is syntactically invalid, envelope tests should not match
		// Note: auth is not an address, so don't validate it
		fieldName := strings.ToLower(expandVars(d, field))
		if value != "" && (fieldName == "from" || fieldName == "to") {
			// Try to parse as envelope address to check validity
			_, err := parseEnvelopeAddress(value)
			if err != nil {
				// Invalid envelope address - should not match anything
				continue
			}
		}
		
		if e.isCount() {
			if value != "" {
				entryCount++
			}
			continue
		}

		ok, err := testAddress(d, e.matcherTest, e.AddressPart, value)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	if e.isCount() {
		return e.countMatches(d, entryCount), nil
	}
	return false, nil
}

type ExistsTest struct {
	Fields []string
}

func (e ExistsTest) Check(_ context.Context, d *RuntimeData) (bool, error) {
	for _, field := range e.Fields {
		values, err := d.Msg.HeaderGet(expandVars(d, field))
		if err != nil {
			return false, err
		}
		if len(values) == 0 {
			return false, nil // Return false if ANY header is missing
		}
	}
	return true, nil // Return true only if ALL headers exist
}

type FalseTest struct{}

func (f FalseTest) Check(context.Context, *RuntimeData) (bool, error) {
	return false, nil
}

type TrueTest struct{}

func (t TrueTest) Check(context.Context, *RuntimeData) (bool, error) {
	return true, nil
}

type HeaderTest struct {
	matcherTest

	Header []string
}

func (h HeaderTest) Check(_ context.Context, d *RuntimeData) (bool, error) {
	entryCount := uint64(0)
	for _, hdr := range h.Header {
		values, err := d.Msg.HeaderGet(expandVars(d, hdr))
		if err != nil {
			return false, err
		}

		for _, value := range values {
			if h.isCount() {
				entryCount++
				continue
			}

			ok, err := h.matcherTest.tryMatch(d, value)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
	}

	if h.isCount() {
		return h.countMatches(d, entryCount), nil
	}

	return false, nil
}

type NotTest struct {
	Test Test
}

func (n NotTest) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	ok, err := n.Test.Check(ctx, d)
	if err != nil {
		return false, err
	}
	return !ok, nil
}

type SizeTest struct {
	Size  int
	Over  bool
	Under bool
}

func (s SizeTest) Check(_ context.Context, d *RuntimeData) (bool, error) {
	if s.Over && d.Msg.MessageSize() > s.Size {
		return true, nil
	}
	if s.Under && d.Msg.MessageSize() < s.Size {
		return true, nil
	}
	return false, nil
}
