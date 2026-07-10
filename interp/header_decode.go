package interp

import (
	"fmt"
	"io"
	"mime"
	"strings"

	"github.com/emersion/go-message"

	// Sets message.CharsetReader so that both RFC 2047 encoded-words in
	// headers and text body parts in legacy charsets (ISO-8859-*,
	// windows-125x, ...) are converted to UTF-8.
	_ "github.com/emersion/go-message/charset"
)

var headerWordDecoder = mime.WordDecoder{
	CharsetReader: func(charset string, input io.Reader) (io.Reader, error) {
		if message.CharsetReader != nil {
			return message.CharsetReader(strings.ToLower(charset), input)
		}
		return nil, fmt.Errorf("unhandled charset %q", charset)
	},
}

// decodeHeaderValue unfolds a header value and decodes RFC 2047
// encoded-words into UTF-8 so that comparisons operate on the decoded text
// (RFC 5228, Section 2.7.2). Values that fail to decode are returned
// unfolded but otherwise unchanged.
func decodeHeaderValue(raw string) string {
	if strings.ContainsAny(raw, "\r\n") {
		raw = strings.NewReplacer("\r", "", "\n", "").Replace(raw)
	}
	if !strings.Contains(raw, "=?") {
		return raw
	}
	decoded, err := headerWordDecoder.DecodeHeader(raw)
	if err != nil {
		return raw
	}
	return decoded
}
