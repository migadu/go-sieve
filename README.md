go-sieve
====================

Sieve email filtering language ([RFC 5228])
implementation in Go.

**Note:** This is a hard fork of [foxcpp/go-sieve](https://github.com/foxcpp/go-sieve).

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
- regex (draft-murchison-sieve-regex)
- date ([RFC 5260])
- index ([RFC 5260])
- editheader ([RFC 5293])
- mailbox ([RFC 5490])
- subaddress ([RFC 5233])
- body ([RFC 5173])

## Supported comparators

- `i;octet`
- `i;ascii-casemap`
- `i;ascii-numeric`
- `i;unicode-casemap`

## Example

See ./cmd/sieve-run.

[RFC 5228]: https://datatracker.ietf.org/doc/html/rfc5228
[RFC 5229]: https://datatracker.ietf.org/doc/html/rfc5229
[RFC 5230]: https://datatracker.ietf.org/doc/html/rfc5230
[RFC 5231]: https://datatracker.ietf.org/doc/html/rfc5231
[RFC 5232]: https://datatracker.ietf.org/doc/html/rfc5232
[RFC 3894]: https://datatracker.ietf.org/doc/html/rfc3894
[RFC 5260]: https://datatracker.ietf.org/doc/html/rfc5260
[RFC 5293]: https://datatracker.ietf.org/doc/html/rfc5293
[RFC 5490]: https://datatracker.ietf.org/doc/html/rfc5490
[RFC 5233]: https://datatracker.ietf.org/doc/html/rfc5233
[RFC 5173]: https://datatracker.ietf.org/doc/html/rfc5173
