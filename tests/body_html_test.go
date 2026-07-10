package tests

import (
	"testing"
)

// The body test must decode the Content-Transfer-Encoding and charset of
// text parts before matching (RFC 5173, Section 5), including for
// single-part (non-multipart) messages.
func TestBodySinglePartEncoded(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "body";

test_set "message" text:
From: shop@example.com
To: steven@example.com
Subject: Order
Content-Type: text/html; charset=utf-8
Content-Transfer-Encoding: quoted-printable

<html><body><p>Bestellung best=C3=A4tigt</p></body></html>
.
;

test "Single-part quoted-printable HTML" {
	if not body :text :contains "Bestellung bestätigt" {
		test_fail "did not match quoted-printable HTML body with :text";
	}
	if not body :content "text/html" :contains "Bestellung" {
		test_fail "did not match quoted-printable HTML body with :content";
	}
}

test_set "message" text:
From: shop@example.com
To: steven@example.com
Subject: Order
Content-Type: text/html; charset=utf-8
Content-Transfer-Encoding: base64

PGh0bWw+PGJvZHk+PHA+QmVzdGVsbHVuZyBiZXN0w6R0aWd0PC9wPjwvYm9keT48L2h0bWw+
.
;

test "Single-part base64 HTML" {
	if not body :text :contains "Bestellung bestätigt" {
		test_fail "did not match base64 HTML body with :text";
	}
}

test_set "message" text:
From: shop@example.com
To: steven@example.com
Subject: Order
Content-Type: text/plain; charset=utf-8
Content-Transfer-Encoding: quoted-printable

Bestellung best=C3=A4tigt
.
;

test "Single-part quoted-printable plain text" {
	if not body :text :contains "Bestellung bestätigt" {
		test_fail "did not match quoted-printable plain text body";
	}
}
`)
}

// Text parts in charsets other than UTF-8/US-ASCII must be converted to
// UTF-8 before matching, not silently skipped.
func TestBodyLegacyCharset(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "body";

test_set "message" text:
From: shop@example.com
To: steven@example.com
Subject: Order
Content-Type: text/html; charset=iso-8859-1
Content-Transfer-Encoding: quoted-printable

<html><body>Bestellung best=E4tigt</body></html>
.
;

test "ISO-8859-1 HTML part" {
	if not body :text :contains "Bestellung bestätigt" {
		test_fail "did not match ISO-8859-1 HTML body";
	}
}

test_set "message" text:
From: shop@example.com
To: steven@example.com
Subject: Order
Content-Type: text/html; charset=windows-1252
Content-Transfer-Encoding: quoted-printable

<html><body>Bestellung best=E4tigt =96 danke</body></html>
.
;

test "Windows-1252 HTML part" {
	if not body :text :contains "Bestellung bestätigt" {
		test_fail "did not match windows-1252 HTML body";
	}
}
`)
}

// :text extracts the text content of HTML: entities must be decoded before
// whitespace normalization so that &nbsp; behaves as a space.
func TestBodyHTMLEntities(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "body";

test_set "message" text:
From: shop@example.com
To: steven@example.com
Subject: Order
Content-Type: text/html; charset=utf-8

<html><body><p>Bestellung&nbsp;best&auml;tigt</p></body></html>
.
;

test "HTML entities and nbsp" {
	if not body :text :contains "Bestellung bestätigt" {
		test_fail "did not match text containing &nbsp; and &auml; entities";
	}
}

test_set "message" text:
From: shop@example.com
To: steven@example.com
Subject: Order
Content-Type: text/html; charset=utf-8

<html><body><p>Bestellung <b>best</b>ätigt</p></body></html>
.
;

test "Tags inside words" {
	if not body :text :contains "Bestellung best ätigt" {
		test_fail "tag replacement should insert whitespace";
	}
}
`)
}

// Multipart messages without a MIME preamble start the body directly with
// the first boundary line; the first part must still be found.
func TestBodyMultipartNoPreamble(t *testing.T) {
	RunDovecotTestInline(t, "", `
require "vnd.dovecot.testsuite";
require "body";

test_set "message" text:
From: shop@example.com
To: steven@example.com
Subject: Order
Content-Type: multipart/alternative; boundary=frontier

--frontier
Content-Type: text/plain; charset=utf-8

Plain variant text
--frontier
Content-Type: text/html; charset=utf-8
Content-Transfer-Encoding: quoted-printable

<html><body>HTML variant best=C3=A4tigt</body></html>
--frontier--
.
;

test "First part without preamble" {
	if not body :text :contains "Plain variant text" {
		test_fail "did not match first part of multipart without preamble";
	}
}

test "HTML alternative part" {
	if not body :text :contains "HTML variant bestätigt" {
		test_fail "did not match quoted-printable HTML part in multipart";
	}
}
`)
}
