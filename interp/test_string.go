package interp

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type Match string

const (
	MatchContains Match = "contains"
	MatchIs       Match = "is"
	MatchMatches  Match = "matches"
	MatchValue    Match = "value"
	MatchCount    Match = "count"
	MatchRegex    Match = "regex"
)

type Comparator string

const (
	ComparatorOctet          Comparator = "i;octet"
	ComparatorASCIICaseMap   Comparator = "i;ascii-casemap"
	ComparatorASCIINumeric   Comparator = "i;ascii-numeric"
	ComparatorUnicodeCaseMap Comparator = "i;unicode-casemap"

	DefaultComparator = ComparatorASCIICaseMap
)

type AddressPart string

const (
	LocalPart AddressPart = "localpart"
	Domain    AddressPart = "domain"
	All       AddressPart = "all"
	// RFC 5233 subaddress extension
	User   AddressPart = "user"
	Detail AddressPart = "detail"
)

// SubaddressSeparator is the character sequence that separates user from detail
// in subaddresses. The default is "+" but can be configured.
var SubaddressSeparator = "+"

func split(addr string) (mailbox, domain string, err error) {
	if strings.EqualFold(addr, "postmaster") {
		return addr, "", nil
	}

	indx := strings.LastIndexByte(addr, '@')
	if indx == -1 {
		return "", "", errors.New("address: missing at-sign")
	}
	mailbox = addr[:indx]
	domain = addr[indx+1:]
	if mailbox == "" {
		return "", "", errors.New("address: empty local-part")
	}
	if domain == "" {
		return "", "", errors.New("address: empty domain")
	}
	return
}

var ErrComparatorMatchUnsupported = fmt.Errorf("match-comparator combination not supported")

func numericValue(s string) *uint64 {
	// https://www.rfc-editor.org/rfc/rfc4790.html#section-9.1

	if len(s) == 0 {
		return nil
	}
	runes := []rune(s)
	if !unicode.IsDigit(runes[0]) {
		return nil
	}
	var sl string
	for i, r := range runes {
		if !unicode.IsDigit(r) {
			sl = string(runes[:i])
			break
		}
	}
	if sl == "" {
		sl = s
	}
	digit, err := strconv.ParseUint(sl, 10, 64)
	if err != nil {
		return nil
	}
	return &digit
}

func testString(comparator Comparator, match Match, rel Relational, value, key string) (bool, []string, error) {
	switch comparator {
	case ComparatorOctet:
		switch match {
		case MatchContains:
			return strings.Contains(value, key), nil, nil
		case MatchIs:
			return value == key, nil, nil
		case MatchMatches:
			return matchOctet(key, value, false)
		case MatchRegex:
			return matchRegex(key, value)
		case MatchValue:
			return rel.CompareString(value, key), nil, nil
		case MatchCount:
			panic("testString should not be used with MatchCount")
		}
	case ComparatorASCIINumeric:
		switch match {
		case MatchContains:
			return false, nil, ErrComparatorMatchUnsupported
		case MatchIs:
			lhsNum := numericValue(value)
			rhsNum := numericValue(key)
			return RelEqual.CompareNumericValue(lhsNum, rhsNum), nil, nil
		case MatchMatches:
			return false, nil, ErrComparatorMatchUnsupported
		case MatchRegex:
			return false, nil, ErrComparatorMatchUnsupported
		case MatchValue:
			lhsNum := numericValue(value)
			rhsNum := numericValue(key)
			return rel.CompareNumericValue(lhsNum, rhsNum), nil, nil
		case MatchCount:
			panic("testString should not be used with MatchCount")
		}
	case ComparatorASCIICaseMap:
		switch match {
		case MatchContains:
			value = toLowerASCII(value)
			key = toLowerASCII(key)
			return strings.Contains(value, key), nil, nil
		case MatchIs:
			value = toLowerASCII(value)
			key = toLowerASCII(key)
			return value == key, nil, nil
		case MatchMatches:
			return matchOctet(key, value, true)
		case MatchRegex:
			// For case-insensitive regex, normalize value but not pattern
			value = toLowerASCII(value)
			return matchRegex(key, value)
		case MatchValue:
			value = toLowerASCII(value)
			key = toLowerASCII(key)
			return rel.CompareString(value, key), nil, nil
		case MatchCount:
			panic("testString should not be used with MatchCount")
		}
	case ComparatorUnicodeCaseMap:
		switch match {
		case MatchContains:
			value = strings.ToLower(value)
			key = strings.ToLower(key)
			return strings.Contains(value, key), nil, nil
		case MatchIs:
			return strings.EqualFold(value, key), nil, nil
		case MatchMatches:
			return matchUnicode(key, value, true)
		case MatchRegex:
			// For Unicode case-insensitive regex, normalize value but not pattern
			value = strings.ToLower(value)
			return matchRegex(key, value)
		case MatchValue:
			value = toLowerASCII(value)
			key = toLowerASCII(key)
			return rel.CompareString(value, key), nil, nil
		case MatchCount:
			panic("testString should not be used with MatchCount")
		}
	}
	return false, nil, nil
}

// splitSubaddress splits a local-part into user and detail parts
// using the SubaddressSeparator. If no separator is found, user is the
// entire local-part and detail is empty.
func splitSubaddress(localPart string) (user, detail string) {
	idx := strings.Index(localPart, SubaddressSeparator)
	if idx == -1 {
		// No separator found - entire local-part is the user
		return localPart, ""
	}
	return localPart[:idx], localPart[idx+len(SubaddressSeparator):]
}

func testAddress(d *RuntimeData, matcher matcherTest, part AddressPart, address string) (bool, error) {
	if address == "<>" {
		address = ""
	}

	var valueToCompare string
	if address != "" {
		switch part {
		case LocalPart:
			localPart, _, err := split(address)
			if err != nil {
				return false, nil
			}
			valueToCompare = localPart
		case Domain:
			_, domain, err := split(address)
			if err != nil {
				return false, nil
			}
			valueToCompare = domain
		case All:
			valueToCompare = address
		case User:
			// RFC 5233: :user is the user sub-part of the local-part
			localPart, _, err := split(address)
			if err != nil {
				return false, nil
			}
			user, _ := splitSubaddress(localPart)
			valueToCompare = user
		case Detail:
			// RFC 5233: :detail is the detail sub-part of the local-part
			// If no detail exists, the address fails to match any key
			localPart, _, err := split(address)
			if err != nil {
				return false, nil
			}
			_, detail := splitSubaddress(localPart)
			if detail == "" && !strings.Contains(localPart, SubaddressSeparator) {
				// No separator found - fail to match (RFC 5233 Section 4)
				return false, nil
			}
			valueToCompare = detail
		}
	}

	ok, err := matcher.tryMatch(d, valueToCompare)
	if err != nil {
		return false, err
	}
	return ok, nil
}

func toLowerASCII(s string) string {
	hasUpper := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		hasUpper = hasUpper || ('A' <= c && c <= 'Z')
	}
	if !hasUpper {
		return s
	}
	var (
		b   strings.Builder
		pos int
	)
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
			if pos < i {
				b.WriteString(s[pos:i])
			}
			b.WriteByte(c)
			pos = i + 1
		}
	}
	if pos < len(s) {
		b.WriteString(s[pos:])
	}
	return b.String()
}

// matchRegex performs safe regex matching and returns match result and capture groups
func matchRegex(pattern, value string) (bool, []string, error) {
	matcher, err := CompileSafeRegex(pattern, DefaultRegexLimits)
	if err != nil {
		return false, nil, err
	}

	matches, err := matcher.FindSubmatch(value)
	if err != nil {
		return false, nil, err
	}

	if matches == nil {
		return false, nil, nil
	}

	return true, matches, nil
}
