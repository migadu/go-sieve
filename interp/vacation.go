package interp

import (
	"context"
	"fmt"
)

// VacationResponse represents an autoresponse to be sent.
type VacationResponse struct {
	// From is the address to be used in the From header of the autoresponse.
	From string

	// Subject is the subject to be used in the autoresponse.
	Subject string

	// Body is the message body to be used in the autoresponse.
	Body string

	// IsMime indicates that the body is a MIME-formatted message.
	IsMime bool

	// Handle is a handle that uniquely identifies this vacation action.
	Handle string

	// Days specifies the minimum number of days between autoresponses to the same sender.
	Days int
}

// CmdVacation represents the vacation command as defined in RFC 5230.
type CmdVacation struct {
	// Days specifies the minimum number of days between autoresponses to the same sender.
	// Default is 7 days if not specified.
	Days int

	// Subject specifies the subject to be used in the autoresponse.
	// Default is "Automated reply" if not specified.
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

	// Reason is the message body to be used in the autoresponse.
	Reason string
}

// Execute implements the vacation command as defined in RFC 5230.
func (c CmdVacation) Execute(ctx context.Context, d *RuntimeData) error {
	// Expand variables in all string fields
	subject := expandVars(d, c.Subject)
	if subject == "" {
		subject = "Automated reply"
	}

	from := expandVars(d, c.From)
	reason := expandVars(d, c.Reason)
	handle := expandVars(d, c.Handle)

	addresses := expandVarsList(d, c.Addresses)

	// Get the sender's address from the message
	// We'll use the envelope from address as the sender
	sender := d.Envelope.EnvelopeFrom()
	if sender == "" {
		return fmt.Errorf("vacation: failed to get sender from envelope")
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
		From:    from,
		Subject: subject,
		Body:    reason,
		IsMime:  c.Mime,
		Handle:  handle,
		Days:    c.Days,
	}

	d.ImplicitKeep = false

	return nil
}
