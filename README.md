go-sieve
====================

Sieve email filtering language ([RFC 5228])
implementation in Go.

## Supported extensions

- envelope ([RFC 5228])
- fileinto ([RFC 5228])
- redirect ([RFC 5228])
- encoded-character ([RFC 5228])
- imap4flags ([RFC 5232])
- variables ([RFC 5229])
- relational ([RFC 5231])
- vacation ([RFC 5230])
- copy ([RFC 3894]) - `:copy` modifier for `redirect` and `fileinto` commands

## Example

See ./cmd/sieve-run.

## Known issues

- Some invalid scripts are accepted as valid (see tests/compile_test.go)
- Comments in addresses are not ignored when testing equality, etc.
- Source routes in addresses are not ignored when testing equality, etc.

[RFC 5228]: https://datatracker.ietf.org/doc/html/rfc5228
[RFC 5229]: https://datatracker.ietf.org/doc/html/rfc5229
[RFC 5230]: https://datatracker.ietf.org/doc/html/rfc5230
[RFC 5231]: https://datatracker.ietf.org/doc/html/rfc5231
[RFC 5232]: https://datatracker.ietf.org/doc/html/rfc5232
[RFC 3894]: https://datatracker.ietf.org/doc/html/rfc3894
