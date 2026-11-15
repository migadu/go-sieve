package interp

import (
	"regexp/syntax"
	"unicode"
)

// These are conservative accounting units, not Go object-layout sizes. They
// cover instruction/capture metadata, rune backing arrays and one-pass lookup
// tables, text backing capacity, and fixed matcher/map overhead respectively.
// Cache admission bounds this estimated footprint, not total process memory or
// transient compilation/matching allocations.
const (
	regexCacheMaxBytes    = 5 << 20
	regexInstructionBytes = 256
	regexRuneBytes        = 32
	regexTextBytes        = 4
	regexEntryBytes       = 2048
)

// Counts saturate just above the admission budget. No large repeated program
// is constructed to estimate it, and neither repetition nor products of
// control/rune counts can overflow an int.
func regexBytesAdd(a, b int) int {
	if a > regexCacheMaxBytes || b > regexCacheMaxBytes-a {
		return regexCacheMaxBytes + 1
	}
	return a + b
}

func regexBytesMul(a, b int) int {
	if a == 0 || b == 0 {
		return 0
	}
	if a > regexCacheMaxBytes/b {
		return regexCacheMaxBytes + 1
	}
	return a * b
}

type regexStorage struct {
	instructions, runes, controls int
}

func (a regexStorage) add(b regexStorage) regexStorage {
	return regexStorage{regexBytesAdd(a.instructions, b.instructions), regexBytesAdd(a.runes, b.runes), regexBytesAdd(a.controls, b.controls)}
}

func (a regexStorage) times(n int) regexStorage {
	return regexStorage{regexBytesMul(a.instructions, n), regexBytesMul(a.runes, n), regexBytesMul(a.controls, n)}
}

// regexStorageFor counts the expanded program's instructions, rune-set ends
// and control instructions. A one-pass compiler can copy the union of the
// reachable rune sets through captures, anchors and branches; the product of
// all control instructions and the full rune envelope bounds those copies.
// Sharing is deliberately not discounted. Parse trees are discarded after
// charging the matcher.
func regexStorageFor(re *syntax.Regexp) regexStorage {
	switch re.Op {
	case syntax.OpLiteral:
		s := regexStorage{instructions: len(re.Rune)}
		for _, r := range re.Rune {
			ends := 2
			if re.Flags&syntax.FoldCase != 0 {
				for other := unicode.SimpleFold(r); other != r; other = unicode.SimpleFold(other) {
					ends += 2
				}
			}
			s.runes = regexBytesAdd(s.runes, ends)
		}
		return s
	case syntax.OpCharClass:
		return regexStorage{instructions: 1, runes: len(re.Rune)}
	case syntax.OpAnyChar:
		return regexStorage{instructions: 1, runes: 2}
	case syntax.OpAnyCharNotNL:
		return regexStorage{instructions: 1, runes: 4}
	case syntax.OpCapture, syntax.OpStar:
		return regexStorageFor(re.Sub[0]).add(regexStorage{instructions: 2, controls: 2})
	case syntax.OpPlus, syntax.OpQuest:
		return regexStorageFor(re.Sub[0]).add(regexStorage{instructions: 1, controls: 1})
	case syntax.OpConcat, syntax.OpAlternate:
		var s regexStorage
		for _, sub := range re.Sub {
			s = s.add(regexStorageFor(sub))
		}
		if re.Op == syntax.OpAlternate {
			s = s.add(regexStorage{instructions: len(re.Sub) - 1, controls: len(re.Sub) - 1})
		}
		return s
	case syntax.OpRepeat:
		s := regexStorageFor(re.Sub[0])
		switch {
		case re.Max == -1 && re.Min == 0:
			return s.add(regexStorage{instructions: 2, controls: 2})
		case re.Max == -1:
			return s.times(re.Min).add(regexStorage{instructions: 1, controls: 1})
		default:
			return s.times(re.Max).add(regexStorage{instructions: re.Max - re.Min, controls: re.Max - re.Min})
		}
	default:
		return regexStorage{instructions: 1, controls: 1}
	}
}

func regexCacheCost(expr, pattern string, parsed *syntax.Regexp) int {
	s := regexStorageFor(parsed)
	cost := regexBytesAdd(regexEntryBytes, regexBytesMul(regexBytesAdd(s.instructions, 2), regexInstructionBytes))
	// Charge leaf rune arrays and every potential control-lookahead copy,
	// including an allowance for the compiler's implicit terminal nodes.
	runes := regexBytesMul(regexBytesAdd(s.runes, 2), regexBytesAdd(s.controls, 3))
	cost = regexBytesAdd(cost, regexBytesMul(runes, regexRuneBytes))
	text := regexBytesAdd(len(expr), len(pattern))
	return regexBytesAdd(cost, regexBytesMul(text, regexTextBytes))
}
