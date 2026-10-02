package interp

import (
	"bytes"
	"html"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

// htmlTextReader reduces HTML to the text a reader sees, as a stream: script
// and style elements are dropped with their content, comments are dropped,
// every other tag becomes a space, character references are decoded, and runs
// of whitespace collapse to one space with none at either end. It holds only
// the bytes of the construct it is inside, so a part of any size streams
// through it in constant memory. This is the one HTML-to-text used by the
// body test's :text and by extracttext.
type htmlTextReader struct {
	src   io.Reader
	in    []byte // unprocessed input; after an EOF, whatever could not complete
	out   []byte // processed text not yet returned
	eof   bool
	state htmlState

	// skipEnd is the closing tag that ends a script or style element.
	skipEnd []byte
	// tag collects a tag's leading name so script/style can be told apart; cap
	// is the one thing a hostile tag can inflate.
	tag []byte
	// entity collects a character reference from "&" until it ends.
	entity []byte

	emitted      bool // some text has been produced: a space may now follow
	pendingSpace bool // whitespace seen since the last text, not yet emitted
}

type htmlState int

const (
	htmlText    htmlState = iota
	htmlTag               // inside "<...>", collecting the name
	htmlTagRest           // inside "<...>", name decided, waiting for ">"
	htmlComment           // inside "<!-- ... -->"
	htmlSkip              // inside a script or style element
	htmlEntity            // inside "&...;"
)

const (
	htmlTagNameMax = 16
	htmlEntityMax  = 32
	htmlReadChunk  = 32 * 1024
)

func newHTMLTextReader(src io.Reader) *htmlTextReader {
	return &htmlTextReader{src: src}
}

// htmlToText reduces s to its text; see htmlTextReader.
func htmlToText(s string) string {
	b, _ := io.ReadAll(newHTMLTextReader(strings.NewReader(s)))
	return string(b)
}

func (r *htmlTextReader) Read(p []byte) (int, error) {
	for len(r.out) == 0 {
		if r.eof {
			r.flush()
			if len(r.out) == 0 {
				return 0, io.EOF
			}
			break
		}
		if len(r.in) < htmlReadChunk {
			buf := make([]byte, htmlReadChunk)
			n, err := r.src.Read(buf)
			r.in = append(r.in, buf[:n]...)
			if err == io.EOF {
				r.eof = true
			} else if err != nil {
				return 0, err
			}
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
				r.in = r.in[1:]
				r.tag = r.tag[:0]
				r.state = htmlTag
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
				r.text(ch, r.in[:0], size)
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
			// The name is complete (or not a name at all).
			if len(r.tag) > 0 && !isTagNameByte(c) {
				if bytes.EqualFold(r.tag, []byte("script")) {
					r.skipEnd = []byte("</script")
				} else if bytes.EqualFold(r.tag, []byte("style")) {
					r.skipEnd = []byte("</style")
				}
			}
			r.state = htmlTagRest
		case htmlTagRest:
			idx := bytes.IndexByte(r.in, '>')
			if idx < 0 {
				if r.eof {
					// An unterminated tag runs to the end, as the "<[^>]*>"
					// view of it never matched; nothing after it is text.
					r.in = r.in[:0]
					return
				}
				r.in = r.in[:0] // the tag's attributes are never text
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
				// Keep the tail that could start "-->" next time.
				keep := len(r.in)
				if keep > 2 {
					keep = 2
				}
				r.in = append(r.in[:0], r.in[len(r.in)-keep:]...)
				if r.eof {
					r.in = r.in[:0]
				}
				return
			}
			r.in = r.in[idx+3:]
			r.space()
			r.state = htmlText
		case htmlSkip:
			idx := indexFold(r.in, r.skipEnd)
			if idx < 0 {
				keep := len(r.in)
				if keep > len(r.skipEnd)-1 {
					keep = len(r.skipEnd) - 1
				}
				r.in = append(r.in[:0], r.in[len(r.in)-keep:]...)
				if r.eof {
					r.in = r.in[:0]
				}
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

// flush is called at the end of input: whatever construct was open is closed
// as the string form of this filter would have closed it.
func (r *htmlTextReader) flush() {
	switch r.state {
	case htmlEntity:
		r.emitEntity()
	case htmlTag:
		r.state = htmlTagRest
	}
	r.in = r.in[:0]
}

func (r *htmlTextReader) emitEntity() {
	decoded := html.UnescapeString(string(r.entity))
	for _, ch := range decoded {
		r.text(ch, nil, utf8.RuneLen(ch))
	}
	r.entity = r.entity[:0]
	r.state = htmlText
}

// text emits one character of visible text, collapsing whitespace.
func (r *htmlTextReader) text(ch rune, _ []byte, _ int) {
	if isHTMLSpace(ch) {
		r.space()
		return
	}
	if r.pendingSpace {
		r.out = append(r.out, ' ')
		r.pendingSpace = false
	}
	r.out = utf8.AppendRune(r.out, ch)
	r.emitted = true
}

// space records whitespace; it is emitted only once text follows, so runs
// collapse and nothing leads or trails.
func (r *htmlTextReader) space() {
	if r.emitted {
		r.pendingSpace = true
	}
}

// isHTMLSpace is the whitespace the text collapses: ASCII whitespace as
// regexp's \s sees it, and Unicode space separators (so U+00A0 from &nbsp;
// becomes a plain space).
func isHTMLSpace(ch rune) bool {
	switch ch {
	case ' ', '\t', '\n', '\f', '\r':
		return true
	}
	return unicode.Is(unicode.Zs, ch)
}

func isTagNameByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func isEntityByte(c byte) bool {
	return isTagNameByte(c) || c == '#'
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
