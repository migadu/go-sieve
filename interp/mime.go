package interp

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"mime"
	"net/textproto"
	"strings"

	"github.com/emersion/go-message"
)

// Bounds on the MIME tree built for the mime, foreverypart and extracttext
// extensions (RFC 5703 §11: part matching must not deny service to other
// users). Structure past either cap is kept as an opaque leaf.
const (
	maxMIMEDepth = 32
	maxMIMEParts = 1024
)

// mimePart is one node of a message's MIME structure. The root part is the
// message itself; its headers are read through the Message (so editheader
// changes apply) rather than stored here.
type mimePart struct {
	root     bool
	header   textproto.MIMEHeader // sub-parts only
	body     []byte               // raw, still transfer-encoded
	children []*mimePart
}

// subtree returns p and every part nested in it, depth first, which is the
// order foreverypart visits them (RFC 5703 §3).
func (p *mimePart) subtree() []*mimePart {
	out := []*mimePart{p}
	for _, c := range p.children {
		out = append(out, c.subtree()...)
	}
	return out
}

// headerValues returns the part's values for a header field, unfolded but not
// otherwise decoded.
func (p *mimePart) headerValues(d *RuntimeData, name string) ([]string, error) {
	if p.root {
		return GetHeaderWithEdits(d, name)
	}
	return p.header.Values(name), nil
}

func (p *mimePart) firstHeader(d *RuntimeData, name string) string {
	values, _ := p.headerValues(d, name)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// text returns the part's content as extracttext stores it (RFC 5703 §7):
// transfer encoding removed, text transcoded to UTF-8 and HTML reduced to its
// text, which is what Pigeonhole does. A part holding other parts yields "",
// as does content that cannot be decoded. At most limit octets are decoded.
func (p *mimePart) text(d *RuntimeData, limit int64) string {
	if len(p.children) > 0 {
		return ""
	}
	contentType := p.firstHeader(d, "Content-Type")
	if contentType == "" {
		contentType = "text/plain; charset=us-ascii"
	}
	var h message.Header
	h.Set("Content-Type", contentType)
	if cte := p.firstHeader(d, "Content-Transfer-Encoding"); cte != "" {
		h.Set("Content-Transfer-Encoding", cte)
	}
	entity, err := message.New(h, bytes.NewReader(p.body))
	if err != nil && !message.IsUnknownCharset(err) {
		return ""
	}
	decoded, err := io.ReadAll(io.LimitReader(entity.Body, limit))
	if err != nil {
		return ""
	}
	mediaType, _ := parseMediaTypeLenient(contentType)
	if mediaType == "text/html" || mediaType == "application/xhtml+xml" {
		return htmlToText(string(decoded))
	}
	return string(decoded)
}

// mimeTree parses the message's MIME structure once per evaluation. The body
// does not change during a run; the root's headers are not copied, so header
// edits are seen either way.
func (d *RuntimeData) mimeTree() (*mimePart, error) {
	if d.mimeRoot != nil {
		return d.mimeRoot, nil
	}
	root := &mimePart{root: true}
	body, hasBody, err := d.Msg.BodyRaw()
	if err != nil {
		return nil, err
	}
	if hasBody {
		root.body = body
		count := 1
		buildMIMEChildren(root, root.firstHeader(d, "Content-Type"), body, 1, &count)
	}
	d.mimeRoot = root
	return root, nil
}

// currentPart is the part a running foreverypart has reached, or nil outside
// every loop.
func (d *RuntimeData) currentPart() *mimePart {
	if n := len(d.partStack); n > 0 {
		return d.partStack[n-1]
	}
	return nil
}

func buildMIMEChildren(p *mimePart, contentType string, body []byte, depth int, count *int) {
	if depth >= maxMIMEDepth || *count >= maxMIMEParts {
		return
	}
	mediaType, params := parseMediaTypeLenient(contentType)
	switch {
	case strings.HasPrefix(mediaType, "multipart/"):
		boundary := params["boundary"]
		if boundary == "" {
			return
		}
		_, parts, _ := splitMultipart(body, boundary)
		for _, raw := range parts {
			if *count >= maxMIMEParts {
				return
			}
			hdr, partBody, err := parsePartHeader(raw)
			if err != nil {
				// Unparseable header: an opaque leaf holding everything.
				hdr, partBody = textproto.MIMEHeader{}, raw
			}
			child := &mimePart{header: hdr, body: partBody}
			p.children = append(p.children, child)
			*count++
			buildMIMEChildren(child, hdr.Get("Content-Type"), partBody, depth+1, count)
		}
	case mediaType == "message/rfc822":
		hdr, nested, err := parsePartHeader(body)
		if err != nil {
			return
		}
		child := &mimePart{header: hdr, body: nested}
		p.children = append(p.children, child)
		*count++
		buildMIMEChildren(child, hdr.Get("Content-Type"), nested, depth+1, count)
	}
}

// parseMediaTypeLenient parses a Content-Type or Content-Disposition value.
// Mail in the wild carries parameters the strict parser rejects (a trailing
// semicolon, a duplicated name); those fall back to the bare media type so the
// type itself still tests. The media type is lower-cased; "" means the value
// did not parse at all.
func parseMediaTypeLenient(v string) (string, map[string]string) {
	mediaType, params, err := mime.ParseMediaType(v)
	if err == nil {
		return mediaType, params
	}
	if trimmed := strings.TrimRight(strings.TrimSpace(v), "; \t"); trimmed != v {
		if mediaType, params, err = mime.ParseMediaType(trimmed); err == nil {
			return mediaType, params
		}
	}
	base, _, _ := strings.Cut(v, ";")
	return strings.ToLower(strings.TrimSpace(base)), map[string]string{}
}

// splitMultipart splits a multipart body at its boundary delimiters. It
// returns the prologue, each part (header and body, with the delimiter's line
// ending removed) and the epilogue.
func splitMultipart(b []byte, boundary string) (prologue []byte, parts [][]byte, epilogue []byte) {
	dashBoundary := []byte("\n--" + boundary)
	dashBoundary2 := []byte("\r\n--" + boundary)

	var chunks [][]byte
	current := b
	// A body without a MIME preamble starts directly with the first
	// delimiter, with no preceding line ending to search for.
	if bytes.HasPrefix(current, []byte("--"+boundary)) && delimiterEnds(current, len(boundary)+2) {
		chunks = append(chunks, nil)
		current = current[len(boundary)+2:]
	}
	for {
		idx := indexDelimiter(current, dashBoundary2)
		if idx == -1 {
			idx = indexDelimiter(current, dashBoundary)
			if idx == -1 {
				chunks = append(chunks, current)
				break
			}
			chunks = append(chunks, current[:idx])
			current = current[idx+len(dashBoundary):]
		} else {
			chunks = append(chunks, current[:idx])
			current = current[idx+len(dashBoundary2):]
		}
	}

	prologue = chunks[0]
	epilogue = []byte{}
	for i := 1; i < len(chunks); i++ {
		p := chunks[i]
		if bytes.HasPrefix(p, []byte("--")) {
			// Closing delimiter; what follows is the epilogue.
			epilogue = p[2:]
			if bytes.HasPrefix(epilogue, []byte("\r\n")) {
				epilogue = epilogue[2:]
			} else if bytes.HasPrefix(epilogue, []byte("\n")) {
				epilogue = epilogue[1:]
			}
			break
		}
		if bytes.HasPrefix(p, []byte("\r\n")) {
			p = p[2:]
		} else if bytes.HasPrefix(p, []byte("\n")) {
			p = p[1:]
		}
		parts = append(parts, p)
	}
	return prologue, parts, epilogue
}

// indexDelimiter finds delim ("\n--boundary") where it ends a delimiter line
// (RFC 2046 §5.1.1): a boundary that is a prefix of another part's boundary
// must not match that part's lines.
func indexDelimiter(b, delim []byte) int {
	off := 0
	for {
		i := bytes.Index(b[off:], delim)
		if i < 0 {
			return -1
		}
		if delimiterEnds(b, off+i+len(delim)) {
			return off + i
		}
		off += i + 1
	}
}

// delimiterEnds reports whether what follows a boundary at offset end can end
// a delimiter line: the line ending, the closing "--", trailing whitespace, or
// the end of the body.
func delimiterEnds(b []byte, end int) bool {
	if end >= len(b) {
		return true
	}
	switch b[end] {
	case '\r', '\n', '-', ' ', '\t':
		return true
	}
	return false
}

// parsePartHeader splits a MIME part into its header and body. The body is
// nil when the part has no blank line.
func parsePartHeader(p []byte) (textproto.MIMEHeader, []byte, error) {
	r := textproto.NewReader(bufio.NewReader(bytes.NewReader(p)))
	hdr, err := r.ReadMIMEHeader()
	if err != nil && err != io.EOF {
		return nil, nil, err
	}

	var body []byte
	if idx := bytes.Index(p, []byte("\r\n\r\n")); idx != -1 {
		body = p[idx+4:]
	} else if idx := bytes.Index(p, []byte("\n\n")); idx != -1 {
		body = p[idx+2:]
	}
	return hdr, body, nil
}

// decodeInputLimit is how many octets of a decoded part are read for matching
// or extraction: the bound the matcher already applies to its input, so a
// large attachment is neither decoded nor copied in full.
func decodeInputLimit(ctx context.Context) int64 {
	if limits, ok := regexLimitsFromContext(ctx); ok {
		return int64(EffectiveRegexLimits(limits).MaxInputLength)
	}
	return int64(DefaultRegexLimits.MaxInputLength)
}
