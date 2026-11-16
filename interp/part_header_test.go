package interp

import (
	"net/textproto"
	"slices"
	"strings"
	"testing"
)

// TestPartHeaderRFC5322Fields: a MIME part's header is read with RFC 5322's
// field syntax, which part headers share (RFC 2045 §3 MIME-part-headers),
// not HTTP's. A field name is "1*ftext", "ftext = %d33-57 / %d59-126"
// (RFC 5322 §3.6.8), so "(", "/" and "@" are name octets; white space may
// stand before the colon ("obs-optional = field-name *WSP ":" ...", §4.5.8);
// and a value may hold control octets (obs-utext, §4.1). §4: these obsolete
// forms "MUST be accepted and parsed by a conformant receiver". A part with
// such a field kept its text from the body test and its fields and children
// from the :mime tests, as if it had no header at all.
func TestPartHeaderRFC5322Fields(t *testing.T) {
	hdr := textproto.MIMEHeader{"Content-Type": {`multipart/mixed; boundary="b"`}}
	body := multipart("b",
		"Content-Type: text/plain\r\nX(a: 1\r\nx(A: 2\r\n\r\nsecret words",
		"Content-Type: multipart/alternative; boundary=\"c\"\r\nX/b: 1\r\n\r\n"+
			multipart("c", "Content-Type: text/plain\r\n\r\nplain alternative", "Content-Type: text/html\r\n\r\n<p>html alternative</p>"),
		"Content-Type: message/rfc822\r\n\r\nSubject: inner\r\nX@c: 1\r\nContent-Type: text/plain\r\n\r\nnested words",
		"Content-Type : text/calendar\r\n\r\nBEGIN:VCALENDAR",
		"Content-Type: text/plain\r\nSubject: a\x01b\r\n\r\ncontrol words",
	)

	for _, tc := range []struct {
		name, script string
		want         string // mailboxes filed into, comma separated
	}{
		{"body of a part with ( in a field name",
			`if body :text :contains "secret words" { fileinto "hit"; }`, "hit"},
		{"header of a part with ( in a field name",
			`if header :mime :anychild :contains "content-type" "text/plain" { fileinto "hit"; }`, "hit"},
		{"field name with ( found under another ASCII case",
			`if exists :mime :anychild "x(A" { fileinto "hit"; }`, "hit"},
		{"both fields of a name i;ascii-casemap folds to one",
			`if header :mime :anychild :is "X(A" "1" { fileinto "one"; }
			 if header :mime :anychild :is "x(a" "2" { fileinto "two"; }`, "one,two"},
		{"values of a folded name in message order",
			`if header :mime :anychild :matches "x(a" "*" { fileinto "${1}"; }`, "1"},
		{"children of a part with / in a field name",
			`if header :mime :anychild :contenttype "Content-Type" "text/html" { fileinto "hit"; }`, "hit"},
		{"body of a child of a part with / in a field name",
			`if body :text :contains "html alternative" { fileinto "hit"; }`, "hit"},
		{"header of a nested message with @ in a field name",
			`if header :mime :anychild :is "Subject" "inner" { fileinto "hit"; }`, "hit"},
		{"body of a nested message with @ in a field name",
			`if body :text :contains "nested words" { fileinto "hit"; }`, "hit"},
		{"white space before the colon",
			`if header :mime :anychild :contenttype "Content-Type" "text/calendar" { fileinto "hit"; }`, "hit"},
		{"control octet in a field value",
			`if body :text :contains "control words" { fileinto "hit"; }`, "hit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := runOn(t, `require ["body", "mime", "fileinto", "variables"]; `+tc.script, hdr, body)
			if got := strings.Join(d.Mailboxes, ","); got != tc.want {
				t.Errorf("%s\n  filed into %q, want %q", tc.script, got, tc.want)
			}
		})
	}
}

func TestParsePartHeader(t *testing.T) {
	for _, tc := range []struct {
		name   string
		in     string
		fields map[string][]string // looked up by name
		body   *string             // nil: no blank line, so no body
		err    bool
	}{
		{name: "CRLF", in: "Content-Type: text/plain\r\nX-A: 1\r\n\r\nbody\r\n",
			fields: map[string][]string{"content-type": {"text/plain"}, "X-A": {"1"}}, body: ptr("body\r\n")},
		{name: "bare LF", in: "Content-Type: text/plain\n\nbody",
			fields: map[string][]string{"Content-Type": {"text/plain"}}, body: ptr("body")},
		{name: "folded value", in: "Subject: a\r\n b \r\n\tc\r\n\r\n",
			fields: map[string][]string{"subject": {"a b c"}}, body: ptr("")},
		{name: "empty value", in: "X-A:\r\n\r\nx",
			fields: map[string][]string{"x-a": {""}}, body: ptr("x")},
		{name: "no header fields", in: "\r\nbody",
			fields: map[string][]string{"content-type": nil}, body: ptr("body")},
		{name: "no blank line", in: "Content-Type: text/plain\r\n",
			fields: map[string][]string{"content-type": {"text/plain"}}},
		{name: "no line ending", in: "Content-Type: text/plain",
			fields: map[string][]string{"content-type": {"text/plain"}}},
		{name: "empty part", in: ""},
		{name: "non-token name octets", in: "X(a: 1\r\nx(A: 2\r\nX/b@c[]{}: 3\r\n\r\n",
			fields: map[string][]string{"x(a": {"1", "2"}, "X(A": {"1", "2"}, "x/B@C[]{}": {"3"}}, body: ptr("")},
		{name: "white space before the colon", in: "Content-Type \t: text/plain\r\n\r\n",
			fields: map[string][]string{"content-type": {"text/plain"}}, body: ptr("")},
		{name: "control octets in a value", in: "Subject: a\x01b\rc\x7f\r\n\r\n",
			fields: map[string][]string{"subject": {"a\x01b\rc\x7f"}}, body: ptr("")},
		{name: "folded name is ASCII only", in: "Key: 1\r\n\r\n",
			fields: map[string][]string{"KEY": {"1"}, "\u212Aey": nil}, body: ptr("")},
		{name: "field without a name", in: ": x\r\nX-A: 1\r\n\r\n",
			fields: map[string][]string{"": nil, "x-a": {"1"}}, body: ptr("")},
		// Not RFC 5322 names, but kept rather than losing the part's header.
		{name: "name octets outside ftext", in: "X A: 1\r\nX-\xc3\xa9: 2\r\n\r\n",
			fields: map[string][]string{"x a": {"1"}, "x-\xc3\xa9": {"2"}}, body: ptr("")},
		{name: "first line continues nothing", in: " X-A: 1\r\n\r\n", err: true},
		{name: "line without a colon", in: "X-A: 1\r\nnot a field\r\n\r\n", err: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hdr, body, err := parsePartHeader([]byte(tc.in))
			if tc.err {
				if err == nil {
					t.Fatalf("%q: no error", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("%q: %v", tc.in, err)
			}
			for name, want := range tc.fields {
				if got := hdr.Values(name); !slices.Equal(got, want) {
					t.Errorf("%q: %q = %q, want %q", tc.in, name, got, want)
				}
			}
			switch {
			case tc.body == nil && body != nil:
				t.Errorf("%q: body %q, want none", tc.in, body)
			case tc.body != nil && body == nil:
				t.Errorf("%q: no body, want %q", tc.in, *tc.body)
			case tc.body != nil && string(body) != *tc.body:
				t.Errorf("%q: body %q, want %q", tc.in, body, *tc.body)
			}
		})
	}
}

func ptr(s string) *string { return &s }
