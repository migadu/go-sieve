package interp

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"
)

// VacationResponse represents an autoresponse to be sent.
type VacationResponse struct {
	// From is the address to be used in the From header of the autoresponse.
	From string

	// Subject is the subject to be used in the autoresponse: the :subject
	// argument, or by default "Auto: " followed by the original subject, or
	// "Automated reply" when the message has none (RFC 5230 section 5.3).
	Subject string

	// Body is the message body to be used in the autoresponse.
	Body string

	// IsMime indicates that the body is a MIME-formatted message.
	IsMime bool

	// Handle is a handle that uniquely identifies this vacation action.
	Handle string

	// ResponseIdentity is an opaque, stable key for RFC 5230 section 4.2
	// suppression tracking. It uses the explicit handle, or the subject, from,
	// mime and reason arguments when no handle was supplied. Arguments are
	// identified before variable expansion; rendered fields above remain expanded.
	ResponseIdentity string

	// Days specifies the minimum number of days between autoresponses to the same sender.
	Days int

	// Addresses is the :addresses argument after variable expansion: addresses
	// that belong to the recipient in addition to those the implementation knows
	// of, for the personal message check of RFC 5230 section 4.5.
	Addresses []string
}

// CmdVacation represents the vacation command as defined in RFC 5230.
type CmdVacation struct {
	// Days specifies the minimum number of days between autoresponses to the same sender.
	// Default is 7 days if not specified.
	Days int

	// Subject specifies the subject to be used in the autoresponse.
	// Default is "Auto: " and the original subject if not specified.
	Subject string

	// From specifies the address to be used in the From header of the autoresponse.
	// If not specified, the implementation should choose a sensible default.
	From string

	// Addresses specifies additional addresses that are considered "my" addresses.
	// These addresses will not trigger an autoresponse.
	Addresses []string

	// Mime indicates that the reason string is a MIME-formatted message.
	Mime bool

	// Handle specifies a handle that uniquely identifies this vacation action.
	// This can be used to manage multiple vacation responses.
	Handle string

	// HandleSet distinguishes an explicit empty handle from an omitted handle.
	HandleSet bool

	// Reason is the message body to be used in the autoresponse.
	Reason string
}

// Execute implements the vacation command as defined in RFC 5230.
func (c CmdVacation) Execute(ctx context.Context, d *RuntimeData) error {
	// The action has executed even when it sends nothing below, and conflicts
	// with reject all the same (RFC 5429 §2.4).
	d.vacationExecuted = true

	// Expand variables in all string fields
	subject := expandVars(d, c.Subject)
	if subject == "" {
		subject = defaultVacationSubject(d)
	}

	from := expandVars(d, c.From)
	reason := expandVars(d, c.Reason)
	handle := expandVars(d, c.Handle)

	addresses := expandVarsList(d, c.Addresses)

	// The response goes to the envelope sender. A null reverse path (a bounce,
	// DSN or MDN) gets none: "Responders MUST NOT generate any response for which
	// the destination of that response would be a null address" (RFC 3834 §2).
	// That is no error, and the rest of the script still runs.
	sender := d.Envelope.EnvelopeFrom()
	if sender == "" {
		return nil
	}

	// Check if the sender is in the list of "my" addresses
	for _, addr := range addresses {
		if addr == sender {
			// Don't send autoresponse to our own addresses
			return nil
		}
	}

	// In a real implementation, we would check if we've already sent an autoresponse
	// to this sender recently, and we would send the autoresponse if allowed.
	// For now, we'll just add the autoresponse to the runtime data.

	// Add the autoresponse to the runtime data
	if d.VacationResponses == nil {
		d.VacationResponses = make(map[string]VacationResponse)
	}

	d.VacationResponses[sender] = VacationResponse{
		From:             from,
		Subject:          subject,
		Body:             reason,
		IsMime:           c.Mime,
		Handle:           handle,
		ResponseIdentity: c.responseIdentity(),
		Days:             c.Days,
		Addresses:        addresses,
	}

	// Per RFC 5230 Section 4: "The vacation action does not cancel the implicit keep."
	// Therefore, we do NOT set d.ImplicitKeep = false here.

	return nil
}

// defaultVacationSubject is the subject of a reply without :subject (RFC 5230
// section 5.3): "The subject is set to the characters "Auto: " followed by the
// original subject. An appropriate fixed Subject, such as "Automated reply",
// SHOULD be used in the event that :subject isn't specified and the original
// message doesn't contain a Subject field." The original is decoded (RFC 2047),
// as the reply's subject is given unencoded.
func defaultVacationSubject(d *RuntimeData) string {
	if values, err := d.Msg.HeaderGet("Subject"); err == nil && len(values) > 0 {
		if original := strings.TrimSpace(decodeHeaderValue(values[0])); original != "" {
			return "Auto: " + original
		}
	}
	return "Automated reply"
}

func (c CmdVacation) responseIdentity() string {
	// Length prefixes preserve tuple boundaries, including empty arguments.
	// The two domains keep an explicit handle separate from synthesized keys.
	h := sha256.New()
	write := func(value string) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		h.Write(size[:])
		h.Write([]byte(value))
	}
	if c.HandleSet || c.Handle != "" {
		write(c.Handle)
		return fmt.Sprintf("v1:h:%x", h.Sum(nil))
	}
	write(c.Subject)
	write(c.From)
	if c.Mime {
		write("1")
	} else {
		write("0")
	}
	write(c.Reason)
	return fmt.Sprintf("v1:r:%x", h.Sum(nil))
}
