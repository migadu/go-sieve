package interp

import (
	"context"
	"regexp"
	"strings"
)

// HeaderEdit represents a header modification (add or delete)
type HeaderEdit struct {
	Action    string // "add" or "delete"
	FieldName string
	Value     string
	Last      bool // for addheader: add at end; for deleteheader: count from end
	Index     int  // for deleteheader: specific index (0 means all)
}

// protectedHeaders are headers that MUST NOT be deleted per RFC 5293
var protectedHeaders = map[string]struct{}{
	"received":       {},
	"auto-submitted": {},
}

// isValidHeaderName checks if a header field name is valid according to RFC 2822
// field-name = 1*ftext
// ftext = %d33-57 / %d59-126 ; Any character except controls, SP, and ":"
func isValidHeaderName(name string) bool {
	if len(name) == 0 {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		// Valid characters: 33-57 (! to 9) and 59-126 (; to ~)
		// Excludes: 0-32 (controls and SP), 58 (:)
		if c < 33 || c > 126 || c == ':' {
			return false
		}
	}
	return true
}

// isProtectedHeader checks if a header is protected from deletion
func isProtectedHeader(name string) bool {
	_, ok := protectedHeaders[strings.ToLower(name)]
	return ok
}

// CmdAddHeader represents the addheader action
type CmdAddHeader struct {
	FieldName string
	Value     string
	Last      bool
}

func (c CmdAddHeader) Execute(_ context.Context, d *RuntimeData) error {
	fieldName := expandVars(d, c.FieldName)
	value := expandVars(d, c.Value)

	// Validate field name
	if !isValidHeaderName(fieldName) {
		// Per RFC 5293: implementation MUST flag an error
		// However, we'll silently ignore per Section 6 recommendation
		return nil
	}

	// Check if protected header that cannot be added (optional, not required by RFC)
	// RFC only requires Subject to be allowed

	d.HeaderEdits = append(d.HeaderEdits, HeaderEdit{
		Action:    "add",
		FieldName: fieldName,
		Value:     value,
		Last:      c.Last,
	})

	return nil
}

// CmdDeleteHeader represents the deleteheader action
type CmdDeleteHeader struct {
	matcherTest
	FieldName     string
	ValuePatterns []string
	Index         int
	Last          bool
}

func (c CmdDeleteHeader) Execute(_ context.Context, d *RuntimeData) error {
	fieldName := expandVars(d, c.FieldName)

	// Validate field name
	if !isValidHeaderName(fieldName) {
		return nil
	}

	// Check if protected header
	if isProtectedHeader(fieldName) {
		// Silently ignore per RFC 5293 Section 6
		return nil
	}

	// Expand value patterns
	valuePatterns := expandVarsList(d, c.ValuePatterns)

	// If no value patterns, delete all matching headers (or specific index)
	if len(valuePatterns) == 0 {
		d.HeaderEdits = append(d.HeaderEdits, HeaderEdit{
			Action:    "delete",
			FieldName: fieldName,
			Index:     c.Index,
			Last:      c.Last,
		})
		return nil
	}

	// Get current header values to find which ones match
	values, err := d.Msg.HeaderGet(fieldName)
	if err != nil {
		return nil
	}

	// Apply existing edits to get the current state
	values = applyHeaderEditsToValues(d, fieldName, values)

	if len(values) == 0 {
		return nil
	}

	// If :index is specified, only check that specific occurrence
	if c.Index > 0 {
		idx := c.Index - 1 // Convert to 0-based
		if c.Last {
			idx = len(values) - c.Index
		}
		if idx < 0 || idx >= len(values) {
			return nil // Index out of range, nothing to delete
		}

		// Check if the value at this index matches any pattern
		matches, err := c.valueMatchesPatterns(d, values[idx], valuePatterns)
		if err != nil || !matches {
			return nil
		}

		// Delete only this specific occurrence
		d.HeaderEdits = append(d.HeaderEdits, HeaderEdit{
			Action:    "delete",
			FieldName: fieldName,
			Value:     values[idx],
			Index:     c.Index,
			Last:      c.Last,
		})
		return nil
	}

	// No :index, check all occurrences
	for _, val := range values {
		matches, err := c.valueMatchesPatterns(d, val, valuePatterns)
		if err != nil {
			continue
		}
		if matches {
			d.HeaderEdits = append(d.HeaderEdits, HeaderEdit{
				Action:    "delete",
				FieldName: fieldName,
				Value:     val,
			})
		}
	}

	return nil
}

func (c CmdDeleteHeader) valueMatchesPatterns(d *RuntimeData, value string, patterns []string) (bool, error) {
	// Trim leading/trailing whitespace as per RFC 5293
	value = strings.TrimSpace(value)

	for _, pattern := range patterns {
		ok, err := c.matcherTest.tryMatch(d, value)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
		// If matcherTest wasn't set up (no value-patterns parsing), do simple matching
		if c.matcherTest.match == "" {
			ok, _, err = testString(c.comparator, MatchIs, "", value, pattern)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
	}
	return false, nil
}

// applyHeaderEditsToValues applies header edits to get the current state of a header
func applyHeaderEditsToValues(d *RuntimeData, fieldName string, values []string) []string {
	if d.HeaderEdits == nil {
		return values
	}

	result := make([]string, len(values))
	copy(result, values)

	for _, edit := range d.HeaderEdits {
		if !strings.EqualFold(edit.FieldName, fieldName) {
			continue
		}

		switch edit.Action {
		case "add":
			if edit.Last {
				result = append(result, edit.Value)
			} else {
				result = append([]string{edit.Value}, result...)
			}
		case "delete":
			if edit.Index > 0 {
				// Delete specific index
				idx := edit.Index - 1
				if edit.Last {
					idx = len(result) - edit.Index
				}
				if idx >= 0 && idx < len(result) {
					result = append(result[:idx], result[idx+1:]...)
				}
			} else if edit.Value != "" {
				// Delete by value
				newResult := make([]string, 0, len(result))
				deleted := false
				for _, v := range result {
					if !deleted && v == edit.Value {
						deleted = true
						continue
					}
					newResult = append(newResult, v)
				}
				result = newResult
			} else {
				// Delete all occurrences
				result = nil
			}
		}
	}

	return result
}

// GetHeaderWithEdits retrieves header values with edits applied
func GetHeaderWithEdits(d *RuntimeData, fieldName string) ([]string, error) {
	values, err := d.Msg.HeaderGet(fieldName)
	if err != nil {
		return nil, err
	}
	return applyHeaderEditsToValues(d, fieldName, values), nil
}

// EditableMessage wraps a Message to apply header edits
type EditableMessage struct {
	Original Message
	Data     *RuntimeData
}

func (m EditableMessage) HeaderGet(key string) ([]string, error) {
	values, err := m.Original.HeaderGet(key)
	if err != nil {
		return nil, err
	}
	return applyHeaderEditsToValues(m.Data, key, values), nil
}

func (m EditableMessage) MessageSize() int {
	return m.Original.MessageSize()
}

// HeaderNameRegex validates header field name per RFC 5322
var HeaderNameRegex = regexp.MustCompile(`^[\x21-\x39\x3b-\x7e]+$`)
