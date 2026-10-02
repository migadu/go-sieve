package interp

import (
	"bytes"
	"context"
	"io"
	"mime"
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
		// For :raw, the whole raw body is treated as a single string.
		if t.isCount() {
			return t.countMatches(d, 1), nil
		}
		return t.tryMatch(ctx, d, string(rawBody))
	}

	// For :text and :content, we need to parse the MIME structure.
	var hdr message.Header
	if vals, err := d.Msg.HeaderGet("Content-Type"); err == nil && len(vals) > 0 {
		for _, v := range vals {
			hdr.Add("Content-Type", v)
		}
	} else {
		hdr.Set("Content-Type", "text/plain; charset=us-ascii")
	}
	// Single-part messages carry their transfer encoding in the top-level
	// header; without it the body would be matched still encoded.
	if vals, err := d.Msg.HeaderGet("Content-Transfer-Encoding"); err == nil {
		for _, v := range vals {
			hdr.Add("Content-Transfer-Encoding", v)
		}
	}

	count := uint64(0)
	var walk func(h message.Header, b []byte) (bool, error)
	walk = func(h message.Header, b []byte) (bool, error) {
		// Honour the script execution deadline while descending the MIME tree.
		if err := ctx.Err(); err != nil {
			return false, err
		}

		contentType := h.Get("Content-Type")
		if contentType == "" {
			contentType = "text/plain; charset=us-ascii"
		}
		mediaType, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			mediaType = strings.TrimSpace(strings.Split(contentType, ";")[0])
		}
		if mediaType == "" {
			mediaType = "text/plain"
		}
		mediaType = strings.ToLower(strings.TrimSpace(mediaType))

		// Check if the current part's content-type matches
		process := false
		if t.text {
			// RFC 5173: Invalid content types (e.g. multiple slashes) don't match
			if strings.Count(mediaType, "/") == 1 && (strings.HasPrefix(mediaType, "text/") || mediaType == "application/xhtml+xml") {
				process = true
			}
		} else if len(t.content) > 0 {
			for _, ct := range t.content {
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
				// Search prologue and epilogue
				if t.isCount() {
					count += 2
				} else {
					match, err := t.tryMatch(ctx, d, string(prologue))
					if err != nil {
						return false, err
					}
					if match {
						return true, nil
					}
					match, err = t.tryMatch(ctx, d, string(epilogue))
					if err != nil {
						return false, err
					}
					if match {
						return true, nil
					}
				}
			}

			// Descend into nested parts
			for _, p := range nested {
				partHdr, partBody, err := parsePartHeader(p)
				if err != nil {
					continue
				}

				mh := message.Header{}
				for k, vv := range partHdr {
					for _, v := range vv {
						mh.Add(k, v)
					}
				}

				match, err := walk(mh, partBody)
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
				mh := message.Header{}
				for k, vv := range nestedHdr {
					for _, v := range vv {
						mh.Add(k, v)
					}
				}
				match, err := walk(mh, nestedBody)
				if err != nil {
					return false, err
				}
				if match {
					return true, nil
				}
			}
		} else {
			if process {
				if t.isCount() {
					count++ // the content itself is not needed
				} else {
					// The part is matched as it decodes (transfer encoding and
					// charset, then HTML to text for :text), never held whole.
					// An unknown charset is not fatal: the part is still
					// readable and matching raw octets beats skipping it.
					entity, err := message.New(h, bytes.NewReader(b))
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

	match, err := walk(hdr, rawBody)
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
