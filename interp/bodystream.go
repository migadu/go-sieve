package interp

import (
	"bytes"
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
//
// A part whose decoding fails part-way is matched on what decoded before the
// failure; the rest cannot be examined. For :matches and :regex such a part
// is skipped, as the whole-part form always did.
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

// appendFolded appends chunk to dst with the comparator's case folding, the
// same folding testString applies to :contains (toLowerASCII, or
// strings.ToLower, whose []byte form bytes.ToLower is). TestStreamMatchesTestString
// holds the two to the same answers.
func (t *TestBody) appendFolded(dst, chunk []byte) []byte {
	switch t.comparator {
	case ComparatorASCIICaseMap:
		for _, c := range chunk {
			dst = append(dst, foldASCII(c))
		}
		return dst
	case ComparatorUnicodeCaseMap:
		return append(dst, bytes.ToLower(chunk)...)
	}
	return append(dst, chunk...)
}

// streamContains is testString's :contains over a stream: the folded content
// is searched chunk by chunk for any folded key, carrying len(key)-1 bytes
// between chunks so a key split across two is still found.
func (t *TestBody) streamContains(ctx context.Context, d *RuntimeData, r io.Reader) (bool, error) {
	keys := make([][]byte, 0, len(t.key))
	carry := 0
	for _, k := range t.key {
		folded := t.appendFolded(nil, []byte(expandVars(d, k)))
		if len(folded) == 0 {
			return true, nil // every string contains ""
		}
		keys = append(keys, folded)
		if len(folded)-1 > carry {
			carry = len(folded) - 1
		}
	}

	var window []byte
	var raw []byte // undecoded tail: a rune cut by the chunk's end
	buf := make([]byte, bodyReadChunk)
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		n, err := r.Read(buf)
		raw = append(raw, buf[:n]...)
		// At the end, or at a decoding error, everything decoded so far is
		// searched; after an error nothing more can be.
		done := err != nil

		// Fold only whole runes; a rune cut by the chunk waits for its rest.
		cut := len(raw)
		if !done {
			cut = lastRuneBoundary(raw)
		}
		window = t.appendFolded(window, raw[:cut])
		raw = append(raw[:0], raw[cut:]...)

		for _, k := range keys {
			if bytes.Contains(window, k) {
				return true, nil
			}
		}
		if done {
			return false, nil
		}
		if len(window) > carry {
			// Keep the tail a key could start in, on a rune boundary.
			start := len(window) - carry
			for start > 0 && start < len(window) && !utf8.RuneStart(window[start]) {
				start--
			}
			kept := copy(window, window[start:])
			window = window[:kept]
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
		} else if string(t.appendFolded(nil, []byte(value))) == string(t.appendFolded(nil, []byte(k))) {
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
