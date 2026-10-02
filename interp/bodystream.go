package interp

import (
	"context"
	"io"
	"strings"
	"unicode/utf8"
)

// bodyReadChunk is how much decoded content the body test looks at per step.
const bodyReadChunk = 32 * 1024

// matchStream tests a part's decoded content as it streams, so the part is
// never held in memory whole. :contains and :is are substring and equality
// searches, which need only a window the size of the key, and the deadline
// is checked between chunks rather than only between parts. :matches and
// :regex need their whole input and already truncate it at the matcher's
// input bound, so only that much is read for them.
func (t *TestBody) matchStream(ctx context.Context, d *RuntimeData, r io.Reader) (bool, error) {
	switch {
	case t.match == MatchContains && t.comparatorFolds():
		return t.streamContains(ctx, d, r)
	case t.match == MatchIs && t.comparatorFolds():
		return t.streamIs(ctx, d, r)
	}
	buf, err := io.ReadAll(io.LimitReader(r, decodeInputLimit(ctx)))
	if err != nil {
		return false, nil // RFC 5173: a part that cannot be decoded is skipped
	}
	return t.tryMatch(ctx, d, string(buf))
}

// comparatorFolds reports whether the comparator is one whose :contains and
// :is are a plain search after case folding (the three the body test is used
// with; i;ascii-numeric rejects :contains at load time).
func (t *TestBody) comparatorFolds() bool {
	switch t.comparator {
	case ComparatorOctet, ComparatorASCIICaseMap, ComparatorUnicodeCaseMap:
		return true
	}
	return false
}

// fold applies the comparator's case folding, exactly as testString does.
func (t *TestBody) fold(s string) string {
	switch t.comparator {
	case ComparatorASCIICaseMap:
		return toLowerASCII(s)
	case ComparatorUnicodeCaseMap:
		return strings.ToLower(s)
	}
	return s
}

// streamContains is testString's :contains over a stream: the folded content
// is searched chunk by chunk for any folded key, carrying len(key)-1 bytes
// between chunks so a key split across two is still found.
func (t *TestBody) streamContains(ctx context.Context, d *RuntimeData, r io.Reader) (bool, error) {
	keys := make([]string, 0, len(t.key))
	carry := 0
	for _, k := range t.key {
		k = t.fold(expandVars(d, k))
		if k == "" {
			return true, nil // every string contains ""
		}
		keys = append(keys, k)
		if len(k)-1 > carry {
			carry = len(k) - 1
		}
	}

	var window string
	var raw []byte // undecoded tail: an incomplete rune at a chunk's end
	buf := make([]byte, bodyReadChunk)
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		n, err := r.Read(buf)
		raw = append(raw, buf[:n]...)
		eof := err == io.EOF
		if err != nil && !eof {
			return false, nil // RFC 5173: a part that cannot be decoded is skipped
		}

		// Fold only whole runes; a rune cut by the chunk waits for its rest.
		cut := len(raw)
		if !eof {
			cut = lastRuneBoundary(raw)
		}
		window += t.fold(string(raw[:cut]))
		raw = append(raw[:0], raw[cut:]...)

		for _, k := range keys {
			if strings.Contains(window, k) {
				return true, nil
			}
		}
		if eof {
			return false, nil
		}
		if len(window) > carry {
			// Keep the tail a key could start in, on a rune boundary.
			start := len(window) - carry
			for start > 0 && !utf8.RuneStart(window[start]) {
				start--
			}
			window = window[start:]
		}
	}
}

// streamIs is testString's :is over a stream. A value can only equal a key
// of n bytes if it has at most n runes, and a rune is at most four bytes, so
// 4n+1 bytes decide it without reading the rest.
func (t *TestBody) streamIs(ctx context.Context, d *RuntimeData, r io.Reader) (bool, error) {
	keys := make([]string, 0, len(t.key))
	longest := 0
	for _, k := range t.key {
		k = expandVars(d, k)
		keys = append(keys, k)
		if len(k) > longest {
			longest = len(k)
		}
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	limit := int64(4*longest + 1)
	buf, err := io.ReadAll(io.LimitReader(r, limit))
	if err != nil {
		return false, nil
	}
	if int64(len(buf)) >= limit {
		return false, nil // longer than any key could fold to
	}
	value := string(buf)
	for _, k := range keys {
		if t.comparator == ComparatorUnicodeCaseMap {
			if strings.EqualFold(value, k) {
				return true, nil
			}
		} else if t.fold(value) == t.fold(k) {
			return true, nil
		}
	}
	return false, nil
}

// lastRuneBoundary returns the length of the longest prefix of b that ends on
// a rune boundary, so a multi-byte rune cut by a chunk is not folded in halves.
func lastRuneBoundary(b []byte) int {
	for i := len(b) - 1; i >= 0 && i >= len(b)-utf8.UTFMax; i-- {
		if utf8.RuneStart(b[i]) {
			if utf8.FullRune(b[i:]) {
				return len(b)
			}
			return i
		}
	}
	return len(b)
}
