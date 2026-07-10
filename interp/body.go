package interp

import (
	"bufio"
	"bytes"
	"context"
	"html"
	"io"
	"mime"
	"net/textproto"
	"regexp"
	"strings"

	"github.com/emersion/go-message"
)

var (
	htmlTagRe   = regexp.MustCompile(`(?s)<[^>]*>`)
	htmlSpaceRe = regexp.MustCompile(`[\s\p{Zs}]+`)
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

			// Split by boundary
			dashBoundary := []byte("\n--" + boundary)
			dashBoundary2 := []byte("\r\n--" + boundary)

			// Find boundaries
			var parts [][]byte
			current := b
			// A message without a MIME preamble starts directly with the
			// first delimiter, with no preceding CRLF to search for.
			if bytes.HasPrefix(current, []byte("--"+boundary)) {
				parts = append(parts, nil)
				current = current[len(boundary)+2:]
			}
			for {
				idx := bytes.Index(current, dashBoundary2)
				if idx == -1 {
					idx = bytes.Index(current, dashBoundary)
					if idx == -1 {
						parts = append(parts, current)
						break
					} else {
						parts = append(parts, current[:idx])
						current = current[idx+len(dashBoundary):]
					}
				} else {
					parts = append(parts, current[:idx])
					current = current[idx+len(dashBoundary2):]
				}
			}

			// parts[0] is prologue
			prologue := parts[0]
			epilogue := []byte{}

			var nested [][]byte
			for i := 1; i < len(parts); i++ {
				p := parts[i]
				if bytes.HasPrefix(p, []byte("--")) {
					// End boundary
					epilogue = p[2:]
					// Skip leading newline in epilogue if present
					if bytes.HasPrefix(epilogue, []byte("\r\n")) {
						epilogue = epilogue[2:]
					} else if bytes.HasPrefix(epilogue, []byte("\n")) {
						epilogue = epilogue[1:]
					}
					break
				}
				// Skip leading newline from boundary match
				if bytes.HasPrefix(p, []byte("\r\n")) {
					p = p[2:]
				} else if bytes.HasPrefix(p, []byte("\n")) {
					p = p[1:]
				}
				nested = append(nested, p)
			}

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
				// Parse headers for nested part
				r := textproto.NewReader(bufio.NewReader(bytes.NewReader(p)))
				partHdr, err := r.ReadMIMEHeader()
				if err != nil && err != io.EOF {
					continue
				}

				// Read until the first blank line to find the body
				idx := bytes.Index(p, []byte("\r\n\r\n"))
				var partBody []byte
				if idx != -1 {
					partBody = p[idx+4:]
				} else {
					idx = bytes.Index(p, []byte("\n\n"))
					if idx != -1 {
						partBody = p[idx+2:]
					} else {
						partBody = nil
					}
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
			r := textproto.NewReader(bufio.NewReader(bytes.NewReader(b)))
			nestedHdr, err := r.ReadMIMEHeader()

			// Extract header bytes exactly as they appear
			var hdrBytes []byte
			idx := bytes.Index(b, []byte("\r\n\r\n"))
			var nestedBody []byte
			if idx != -1 {
				hdrBytes = b[:idx+2] // include the last \r\n of the header block but not the blank line
				nestedBody = b[idx+4:]
			} else {
				idx = bytes.Index(b, []byte("\n\n"))
				if idx != -1 {
					hdrBytes = b[:idx+1]
					nestedBody = b[idx+2:]
				} else {
					hdrBytes = b
					nestedBody = nil
				}
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

			if err == nil || err == io.EOF {
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
				// Text part
				// For text parts, we should decode transfer encoding if any
				// An unknown charset is not fatal: the part is still
				// readable and matching raw octets beats skipping it.
				entity, err := message.New(h, bytes.NewReader(b))
				if err != nil && !message.IsUnknownCharset(err) {
					return false, nil // RFC 5173: skip if text cannot be decoded
				}
				decodedBody, err := io.ReadAll(entity.Body)
				if err != nil {
					return false, nil
				}

				if t.text && (mediaType == "text/html" || mediaType == "application/xhtml+xml") {
					// Very simple HTML stripping just for Sieve tests
					stripped := htmlTagRe.ReplaceAllString(string(decodedBody), " ")

					// Decode entities before collapsing whitespace so that
					// &nbsp; (U+00A0) is normalized to a plain space too.
					stripped = html.UnescapeString(stripped)
					stripped = htmlSpaceRe.ReplaceAllString(stripped, " ")
					decodedBody = []byte(strings.TrimSpace(stripped))
				}

				if t.isCount() {
					count++
				} else {
					match, err := t.tryMatch(ctx, d, string(decodedBody))
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
