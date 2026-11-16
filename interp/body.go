package interp

import (
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/emersion/go-message"
)

type TestBody struct {
	matcherTest

	raw     bool
	text    bool
	content []string
}

func (t *TestBody) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	savedVars := d.MatchVariables
	defer func() {
		d.MatchVariables = savedVars
	}()

	rawBody, hasBody, err := d.Msg.BodyRaw()
	if err != nil {
		return false, err
	}

	if !hasBody {
		return false, nil
	}

	if t.raw {
		// For :raw, the whole raw body is treated as a single string, matched
		// as it is read rather than copied.
		if t.isCount() {
			return t.countMatches(d, 1), nil
		}
		return t.matchStream(ctx, d, bytes.NewReader(rawBody))
	}

	// For :text and :content, we need to parse the MIME structure. The
	// message and each part are read by their first Content-Type and
	// Content-Transfer-Encoding fields, as the mime extension reads them
	// (mimePart.firstHeader), so a repeated field does not give the body test
	// one view of a part and the :mime tests and extracttext another.
	contentType, cte := "text/plain; charset=us-ascii", ""
	if vals, err := d.Msg.HeaderGet("Content-Type"); err == nil && len(vals) > 0 {
		contentType = vals[0]
	}
	// Single-part messages carry their transfer encoding in the top-level
	// header; without it the body would be matched still encoded.
	if vals, err := d.Msg.HeaderGet("Content-Transfer-Encoding"); err == nil && len(vals) > 0 {
		cte = vals[0]
	}

	// RFC 5173 §6: "variable references in the key strings or content type
	// strings are evaluated as described in this document."
	contentTypes := expandVarsList(d, t.content)

	count := uint64(0)
	var walk func(contentType, cte string, b []byte) (bool, error)
	walk = func(contentType, cte string, b []byte) (bool, error) {
		// Honour the script execution deadline while descending the MIME tree.
		if err := ctx.Err(); err != nil {
			return false, err
		}

		if contentType == "" {
			contentType = "text/plain; charset=us-ascii"
		}
		// Parsed as the MIME tree parses it (buildMIMEChildren), so the body
		// test and the mime extension read the same type and boundary.
		mediaType, params := parseMediaTypeLenient(contentType)
		if mediaType == "" {
			mediaType = "text/plain"
		}

		// Check if the current part's content-type matches
		process := false
		if t.text {
			// RFC 5173: Invalid content types (e.g. multiple slashes) don't match
			if strings.Count(mediaType, "/") == 1 && (strings.HasPrefix(mediaType, "text/") || mediaType == "application/xhtml+xml") {
				process = true
			}
		} else if len(contentTypes) > 0 {
			for _, ct := range contentTypes {
				ct = strings.ToLower(strings.TrimSpace(ct))
				if ct == "" {
					process = true
					break
				}
				if strings.HasPrefix(ct, "/") || strings.HasSuffix(ct, "/") || strings.Count(ct, "/") > 1 {
					continue // Matches no content types
				}
				if ct == mediaType || strings.HasPrefix(mediaType, ct+"/") {
					process = true
					break
				}
			}
		}

		if strings.HasPrefix(mediaType, "multipart/") {
			boundary := params["boundary"]
			if boundary == "" {
				// Treat as text/plain if no boundary
				if process {
					if t.isCount() {
						count++
					} else {
						match, err := t.tryMatch(ctx, d, string(b))
						if err != nil {
							return false, err
						}
						if match {
							return true, nil
						}
					}
				}
				return false, nil
			}

			prologue, nested, epilogue := splitMultipart(b, boundary)

			if process {
				// Search prologue and epilogue. A body whose boundary never
				// appears is one big prologue; it streams rather than being
				// copied into a string.
				if t.isCount() {
					count += 2
				} else {
					for _, part := range [][]byte{prologue, epilogue} {
						match, err := t.matchStream(ctx, d, bytes.NewReader(part))
						if err != nil {
							return false, err
						}
						if match {
							return true, nil
						}
					}
				}
			}

			// Descend into nested parts
			for _, p := range nested {
				partHdr, partBody, err := parsePartHeader(p)
				if err != nil {
					continue
				}

				match, err := walk(partHdr.Get("Content-Type"), partHdr.Get("Content-Transfer-Encoding"), partBody)
				if err != nil {
					return false, err
				}
				if match {
					return true, nil
				}
			}
		} else if mediaType == "message/rfc822" {
			// RFC 5173: match against the header of the nested message
			nestedHdr, nestedBody, err := parsePartHeader(b)

			// The header block as it appears, without the blank line.
			hdrBytes := b
			if nestedBody != nil {
				hdrBytes = bytes.TrimSuffix(bytes.TrimSuffix(b[:len(b)-len(nestedBody)], []byte("\n")), []byte("\r"))
			}

			if process {
				if t.isCount() {
					count++
				} else {
					match, err := t.tryMatch(ctx, d, string(hdrBytes))
					if err != nil {
						return false, err
					}
					if match {
						return true, nil
					}
				}
			}

			if err == nil {
				match, err := walk(nestedHdr.Get("Content-Type"), nestedHdr.Get("Content-Transfer-Encoding"), nestedBody)
				if err != nil {
					return false, err
				}
				if match {
					return true, nil
				}
			}
		} else {
			if process {
				// The part is decoded by the two fields the walk read.
				h := decodingHeader(contentType, cte)
				if t.isCount() {
					// The content is not needed, but a part that cannot be
					// decoded is not counted, as it is not matched.
					if _, err := newBodyEntity(h, cte, b); err != nil && !message.IsUnknownCharset(err) {
						return false, nil
					}
					count++
				} else {
					// The part is matched as it decodes (transfer encoding and
					// charset, then HTML to text for :text), never held whole.
					// An unknown charset is not fatal: the part is still
					// readable and matching raw octets beats skipping it.
					entity, err := newBodyEntity(h, cte, b)
					if err != nil && !message.IsUnknownCharset(err) {
						return false, nil // RFC 5173: skip if text cannot be decoded
					}
					var content io.Reader = entity.Body
					if t.text && (mediaType == "text/html" || mediaType == "application/xhtml+xml") {
						content = newHTMLTextReader(content)
					}
					match, err := t.matchStream(ctx, d, content)
					if err != nil {
						return false, err
					}
					if match {
						return true, nil
					}
				}
			}
		}

		return false, nil
	}

	match, err := walk(contentType, cte, rawBody)
	if err != nil {
		return false, err
	}
	if match {
		return true, nil
	}

	if t.isCount() {
		return t.countMatches(d, count), nil
	}

	return false, nil
}

// newBodyEntity retains transfer-decoding errors even when the charset is also
// unknown. go-message reports only the charset error in that case; tolerating
// it would otherwise match and count a part whose transfer encoding cannot be
// decoded. Check that encoding alone before applying the raw-charset fallback.
func newBodyEntity(h message.Header, cte string, body []byte) (*message.Entity, error) {
	entity, err := message.New(h, bytes.NewReader(body))
	if message.IsUnknownCharset(err) {
		var transferHeader message.Header
		transferHeader.Set("Content-Transfer-Encoding", cte)
		if _, transferErr := message.New(transferHeader, bytes.NewReader(nil)); transferErr != nil {
			return entity, transferErr
		}
	}
	return entity, err
}
