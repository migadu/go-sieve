package interp

import (
	"bytes"
	"html"
	"io"
	"unicode"
	"unicode/utf8"
)

// htmlTextReader reduces HTML to the text a reader sees, as a stream: script
// and style elements are dropped with their content, comments are dropped,
// every other tag becomes a space, character references are decoded, and
// whitespace is collapsed and trimmed. It holds only the bytes of the
// construct it is inside, so a part of any size streams through it in
// constant memory. This is the one HTML-to-text used by the body test's :text
// and by extracttext.
//
// What counts as a tag follows the HTML tokenizer: "<" opens one only when an
// ASCII letter, "/", "!" or "?" follows, so "5 < 10" is text; a tag with no
// ">" runs to the end of the input, as does a comment with no "-->".
//
// Whitespace is handled as the regular-expression form of this filter did:
// runs of ASCII whitespace and Unicode space separators (so U+00A0 from
// &nbsp; too) collapse to one space in the middle of the text, and any
// Unicode white space is trimmed from both ends.
type htmlTextReader struct {
	src   io.Reader
	buf   []byte // read buffer, reused
	in    []byte // unprocessed input
	out   []byte // processed text not yet returned
	eof   bool
	state htmlState

	// skipEnd is the closing tag that ends a script or style element.
	skipEnd []byte
	// tag collects a tag's leading name so script/style can be told apart; its
	// cap is the one thing a hostile tag can inflate.
	tag []byte
	// entity collects a character reference from "&" until it ends.
	entity []byte

	// emitted: some text has been produced, so whitespace may now follow it.
	emitted bool
	// pending is the whitespace seen since the last visible text, in order,
	// with runs already collapsed. It is emitted before the next visible
	// text and dropped at the end, which trims the tail.
	pending []byte
}

type htmlState int

const (
	htmlText    htmlState = iota
	htmlTag               // after "<", collecting the name
	htmlTagRest           // inside a tag, name decided, waiting for ">"
	htmlComment           // inside "<!-- ... -->"
	htmlSkip              // inside a script or style element
	htmlEntity            // after "&", collecting a character reference
)

const (
	htmlTagNameMax = 16
	htmlEntityMax  = 32
	htmlReadChunk  = 32 * 1024
)

func newHTMLTextReader(src io.Reader) *htmlTextReader {
	return &htmlTextReader{src: src}
}

func (r *htmlTextReader) Read(p []byte) (int, error) {
	for len(r.out) == 0 {
		if r.eof {
			r.finish()
			if len(r.out) == 0 {
				return 0, io.EOF
			}
			break
		}
		if r.buf == nil {
			r.buf = make([]byte, htmlReadChunk)
		}
		n, err := r.src.Read(r.buf)
		r.in = append(r.in, r.buf[:n]...)
		if err == io.EOF {
			r.eof = true
		} else if err != nil {
			return 0, err
		}
		r.process()
	}
	n := copy(p, r.out)
	r.out = r.out[n:]
	return n, nil
}

// process consumes as much of r.in as can be decided without more input.
func (r *htmlTextReader) process() {
	for len(r.in) > 0 {
		switch r.state {
		case htmlText:
			c := r.in[0]
			switch {
			case c == '<':
				// A tag needs a letter, "/", "!" or "?" next; anything else
				// makes the "<" ordinary text.
				if len(r.in) < 2 && !r.eof {
					return
				}
				if len(r.in) >= 2 && opensTag(r.in[1]) {
					r.in = r.in[1:]
					r.tag = r.tag[:0]
					r.state = htmlTag
					continue
				}
				r.in = r.in[1:]
				r.visible('<')
			case c == '&':
				r.in = r.in[1:]
				r.entity = append(r.entity[:0], '&')
				r.state = htmlEntity
			default:
				if !utf8.FullRune(r.in) && !r.eof {
					return
				}
				ch, size := utf8.DecodeRune(r.in)
				r.in = r.in[size:]
				if ch == utf8.RuneError && size == 1 {
					// Not UTF-8: pass the octet through, as text/plain
					// parts are matched on their raw octets too.
					r.visibleByte(c)
					continue
				}
				r.visible(ch)
			}
		case htmlTag:
			c := r.in[0]
			if len(r.tag) == 0 && c == '!' {
				// "<!--" opens a comment; decide only once enough has arrived.
				if len(r.in) < 3 && !r.eof {
					return
				}
				if bytes.HasPrefix(r.in, []byte("!--")) {
					r.in = r.in[3:]
					r.state = htmlComment
					continue
				}
			}
			if isTagNameByte(c) && len(r.tag) < htmlTagNameMax {
				r.tag = append(r.tag, c)
				r.in = r.in[1:]
				continue
			}
			// The name is complete.
			if bytes.EqualFold(r.tag, []byte("script")) {
				r.skipEnd = []byte("</script")
			} else if bytes.EqualFold(r.tag, []byte("style")) {
				r.skipEnd = []byte("</style")
			}
			r.state = htmlTagRest
		case htmlTagRest:
			idx := bytes.IndexByte(r.in, '>')
			if idx < 0 {
				r.in = r.in[:0] // attributes are never text
				return
			}
			r.in = r.in[idx+1:]
			r.space()
			if r.skipEnd != nil {
				r.state = htmlSkip
			} else {
				r.state = htmlText
			}
		case htmlComment:
			idx := bytes.Index(r.in, []byte("-->"))
			if idx < 0 {
				r.keepTail(2)
				return
			}
			r.in = r.in[idx+3:]
			r.space()
			r.state = htmlText
		case htmlSkip:
			idx, decided := r.findSkipEnd()
			if !decided {
				return
			}
			if idx < 0 {
				r.keepTail(len(r.skipEnd))
				return
			}
			r.in = r.in[idx+len(r.skipEnd):]
			r.skipEnd = nil
			r.state = htmlTagRest // the rest of "</script ...>"
		case htmlEntity:
			c := r.in[0]
			if c == ';' {
				r.entity = append(r.entity, ';')
				r.in = r.in[1:]
				r.emitEntity()
				continue
			}
			if isEntityByte(c) && len(r.entity) < htmlEntityMax {
				r.entity = append(r.entity, c)
				r.in = r.in[1:]
				continue
			}
			// Not a reference after all, or one without its ";".
			r.emitEntity()
		}
	}
}

// findSkipEnd looks for the closing tag that ends the script or style element
// at a place a tag can end ("</script>", "</script >", "</script/>"), so
// "</scripts>" does not end it. decided is false when the answer needs more
// input.
func (r *htmlTextReader) findSkipEnd() (idx int, decided bool) {
	off := 0
	for {
		i := indexFold(r.in[off:], r.skipEnd)
		if i < 0 {
			return -1, true
		}
		i += off
		end := i + len(r.skipEnd)
		if end >= len(r.in) {
			if r.eof {
				return i, true
			}
			r.in = append(r.in[:0], r.in[i:]...)
			return -1, false
		}
		if c := r.in[end]; c == '>' || c == '/' || isSpaceByte(c) {
			return i, true
		}
		off = i + 1
	}
}

// keepTail drops consumed input but keeps the last n bytes, which may be the
// start of the construct's terminator.
func (r *htmlTextReader) keepTail(n int) {
	if r.eof {
		r.in = r.in[:0]
		return
	}
	if len(r.in) > n {
		r.in = append(r.in[:0], r.in[len(r.in)-n:]...)
	}
}

// finish is called at the end of input: an open character reference is
// decoded as it stands; an open tag, comment or script element is dropped,
// as the HTML tokenizer drops them; pending whitespace is the trailing
// whitespace and goes.
func (r *htmlTextReader) finish() {
	if r.state == htmlEntity {
		r.emitEntity()
	}
	r.state = htmlText
	r.in = r.in[:0]
	r.pending = r.pending[:0]
}

func (r *htmlTextReader) emitEntity() {
	for _, ch := range html.UnescapeString(string(r.entity)) {
		r.visible(ch)
	}
	r.entity = r.entity[:0]
	r.state = htmlText
}

// visible emits one character of text, with the whitespace rules applied.
func (r *htmlTextReader) visible(ch rune) {
	switch {
	case isCollapsedSpace(ch):
		r.space()
	case unicode.IsSpace(ch):
		// Other white space is kept in the middle of the text but trimmed
		// from the ends, so it waits for the next visible character.
		if r.emitted {
			r.pending = utf8.AppendRune(r.pending, ch)
		}
	default:
		r.flushPending()
		r.out = utf8.AppendRune(r.out, ch)
		r.emitted = true
	}
}

// visibleByte emits one octet that is not UTF-8.
func (r *htmlTextReader) visibleByte(c byte) {
	r.flushPending()
	r.out = append(r.out, c)
	r.emitted = true
}

func (r *htmlTextReader) flushPending() {
	if len(r.pending) > 0 {
		r.out = append(r.out, r.pending...)
		r.pending = r.pending[:0]
	}
}

// space records collapsible whitespace (a tag counts as one): at most one
// space in a row, none before the first visible character.
func (r *htmlTextReader) space() {
	if r.emitted && (len(r.pending) == 0 || r.pending[len(r.pending)-1] != ' ') {
		r.pending = append(r.pending, ' ')
	}
}

// isCollapsedSpace is the whitespace that collapses to one space: ASCII
// whitespace as regexp's \s sees it, and Unicode space separators.
func isCollapsedSpace(ch rune) bool {
	switch ch {
	case ' ', '\t', '\n', '\f', '\r':
		return true
	}
	return unicode.Is(unicode.Zs, ch)
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\f' || c == '\r'
}

// opensTag reports whether the byte after "<" makes it a tag.
func opensTag(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '/' || c == '!' || c == '?'
}

// isTagNameByte is a word character, so "<script_x>" is not a script element
// while "<script-x>" is, exactly as the \b of the previous form decided.
func isTagNameByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

func isEntityByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '#'
}

// indexFold finds the ASCII-case-insensitive needle in b.
func indexFold(b, needle []byte) int {
	for i := 0; i+len(needle) <= len(b); i++ {
		if bytes.EqualFold(b[i:i+len(needle)], needle) {
			return i
		}
	}
	return -1
}
