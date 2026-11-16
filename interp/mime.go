package interp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
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
	header   partHeader // sub-parts only
	body     []byte     // raw, still transfer-encoded
	children []*mimePart

	// extracttext's view of the part, computed once per evaluation.
	textCached bool
	textValue  string
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
// text. A part holding other parts yields "", as does one whose charset is
// unknown (§7: "an empty string will result"). A non-text leaf is stored
// decoded, as Pigeonhole does: its decoder only returns raw binary when asked
// to, and Pigeonhole does not ask, so the content goes through its UTF-8
// translation instead. At most limit octets are decoded.
func (p *mimePart) text(d *RuntimeData, limit int64) string {
	if !p.textCached {
		p.textValue = p.decodeText(d, limit)
		p.textCached = true
	}
	return p.textValue
}

func (p *mimePart) decodeText(d *RuntimeData, limit int64) string {
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
	if err != nil {
		return ""
	}
	var content io.Reader = io.LimitReader(entity.Body, limit)
	mediaType, _ := parseMediaTypeLenient(contentType)
	if mediaType == "text/html" || mediaType == "application/xhtml+xml" {
		content = newHTMLTextReader(content)
	}
	decoded, err := io.ReadAll(content)
	if err != nil {
		return ""
	}
	return strings.ToValidUTF8(string(decoded), "\uFFFD")
}

// mimeTree parses the message's MIME structure once per message. The tree is
// kept for as long as the Message hands out the same body; a caller that
// swaps d.Msg gets a fresh parse. The root's headers are not copied, so
// header edits are seen either way.
func (d *RuntimeData) mimeTree() (*mimePart, error) {
	body, hasBody, err := d.Msg.BodyRaw()
	if err != nil {
		return nil, err
	}
	if d.mimeRoot != nil && sameBytes(d.mimeRoot.body, body) {
		return d.mimeRoot, nil
	}
	root := &mimePart{root: true}
	if hasBody {
		root.body = body
		count := 1
		buildMIMEChildren(root, root.firstHeader(d, "Content-Type"), body, 1, &count)
	}
	d.mimeRoot = root
	return root, nil
}

// sameBytes reports whether a and b are the same slice of the same array.
func sameBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	return len(a) == 0 || &a[0] == &b[0]
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
			hdr, partBody := readBodyPart(raw)
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
	// One scan for the LF form; a CR before it belongs to the delimiter too.
	// Scanning for the CRLF form first would skip an earlier bare-LF
	// delimiter in a body with mixed line endings.
	dashBoundary := []byte("\n--" + boundary)

	var chunks [][]byte
	current := b
	// A body without a MIME preamble starts directly with the first
	// delimiter, with no preceding line ending to search for.
	if bytes.HasPrefix(current, []byte("--"+boundary)) && delimiterEnds(current, len(boundary)+2) {
		chunks = append(chunks, nil)
		current = current[len(boundary)+2:]
	}
	for {
		idx := indexDelimiter(current, dashBoundary)
		if idx == -1 {
			chunks = append(chunks, current)
			break
		}
		chunk := current[:idx]
		if len(chunk) > 0 && chunk[len(chunk)-1] == '\r' {
			chunk = chunk[:len(chunk)-1]
		}
		chunks = append(chunks, chunk)
		current = current[idx+len(dashBoundary):]
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
// the end of the body. A single "-" cannot: "x-y" is a different boundary.
func delimiterEnds(b []byte, end int) bool {
	if end >= len(b) {
		return true
	}
	switch b[end] {
	case '\r', '\n', ' ', '\t':
		return true
	case '-':
		return end+1 < len(b) && b[end+1] == '-'
	}
	return false
}

// partHeader holds a MIME part's header fields: their values in the order the
// fields appear, keyed by the field name folded under i;ascii-casemap, which
// header names always compare with (RFC 5228 §2.7). "X(a" and "x(A" name one
// field.
type partHeader map[string][]string

// Values returns the values of the fields named name.
func (h partHeader) Values(name string) []string {
	return h[lowerASCII(name)]
}

// Get returns the value of the first field named name, or "".
func (h partHeader) Get(name string) string {
	if values := h.Values(name); len(values) > 0 {
		return values[0]
	}
	return ""
}

// readBodyPart splits one body part of a multipart into its header and body.
// A part whose header does not parse is still a part: the body test searches
// "All MIME parts with matching types" (RFC 5173 §5.2) and foreverypart
// "walks though every MIME part of a message" (RFC 5703 §3). It is read as a
// leaf with no header fields holding the whole part, header lines included,
// so its type is the text/plain default (RFC 2045 §5.2). The body test and
// the MIME tree both read parts here, so they see the same parts.
func readBodyPart(raw []byte) (partHeader, []byte) {
	hdr, body, err := parsePartHeader(raw)
	if err != nil {
		return nil, raw
	}
	return hdr, body
}

var errMalformedPartHeader = errors.New("malformed MIME part header")

// parsePartHeader splits a MIME part into its header and body. The body is
// what follows the blank line that ends the header, so a part with no header
// fields (RFC 2046 §5.1 allows one) still yields its body; it is nil when the
// part has no blank line at all.
//
// The fields are read as RFC 5322 defines them, which MIME part headers share
// (RFC 2045 §3), and not as net/textproto reads HTTP's: a name may hold any
// ftext octet ("(", "/", "@", ..., §3.6.8), white space may stand before the
// colon (§4.5.8), and a value may hold control octets (§4.1). The name is
// otherwise taken as written, and a field with an empty name is dropped. Only a
// line with no colon, or folding white space before the first field, makes the
// header malformed.
func parsePartHeader(p []byte) (partHeader, []byte, error) {
	hdr := partHeader{}
	var (
		name    string // the field being read, "" for one without a name
		value   []byte // its value so far, unfolded
		inField bool
	)
	endField := func() {
		if name != "" {
			hdr[name] = append(hdr[name], string(value))
		}
	}
	for rest := p; len(rest) > 0; {
		line := rest
		if i := bytes.IndexByte(rest, '\n'); i >= 0 {
			line, rest = rest[:i], rest[i+1:]
		} else {
			rest = nil
		}
		line = bytes.TrimSuffix(line, []byte("\r"))
		switch {
		case len(line) == 0:
			endField()
			return hdr, rest, nil
		case line[0] == ' ' || line[0] == '\t':
			if !inField {
				return nil, nil, errMalformedPartHeader
			}
			value = appendUnfolded(value, line)
		default:
			endField()
			colon := bytes.IndexByte(line, ':')
			if colon < 0 {
				return nil, nil, errMalformedPartHeader
			}
			name = lowerASCII(string(bytes.TrimRight(line[:colon], " \t")))
			value = appendUnfolded(value[:0], line[colon+1:])
			inField = true
		}
	}
	endField()
	return hdr, nil, nil
}

// appendUnfolded appends one line of a field's value to value as net/textproto
// and go-message join the lines of a folded value: white space around the line
// is dropped and one space separates it from the line before.
func appendUnfolded(value, line []byte) []byte {
	line = bytes.Trim(line, " \t")
	if len(value) > 0 {
		value = append(value, ' ')
	}
	return append(value, line...)
}

// lowerASCII lowers the US-ASCII letters of s and leaves every other octet as
// it is: i;ascii-casemap "treats US-ASCII letters case-insensitively", and
// "letters outside ASCII are not treated case-insensitively" (RFC 4790
// §9.2.1).
func lowerASCII(s string) string {
	for i := 0; i < len(s); i++ {
		if 'A' <= s[i] && s[i] <= 'Z' {
			b := []byte(s)
			for ; i < len(b); i++ {
				b[i] = foldASCII(b[i])
			}
			return string(b)
		}
	}
	return s
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
