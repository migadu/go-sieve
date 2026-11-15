package interp

import (
	"context"
	"strings"
	"testing"
)

func TestRegexCacheByteAccountingSaturates(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	if got := regexBytesAdd(maxInt, 1); got != regexCacheMaxBytes+1 {
		t.Fatalf("overflowing sum: %d", got)
	}
	if got := regexBytesMul(maxInt, maxInt); got != regexCacheMaxBytes+1 {
		t.Fatalf("overflowing product: %d", got)
	}
	if got := regexBytesMul(0, maxInt); got != 0 {
		t.Fatalf("zero product: %d", got)
	}
	if got := regexBytesAdd(regexCacheMaxBytes, 1); got != regexCacheMaxBytes+1 {
		t.Fatalf("budget boundary: %d", got)
	}
}

// A legal small program can retain a large one-pass rune lookahead on every
// capture instruction. It must still match, without entering the cache.
func TestRegexCacheDeclinesLargeCaptureLookahead(t *testing.T) {
	pattern := "^k001" + strings.Repeat("(", 400) + `\p{L}` + strings.Repeat(")", 400) + "$"
	var cache regexCache
	first, err := cache.compile(pattern, ComparatorASCIICaseMap)
	if err != nil {
		t.Fatal(err)
	}
	if first.size > DefaultRegexLimits.MaxProgramSize {
		t.Fatal("the key exceeded the existing program bound")
	}
	if first.cacheBytes <= regexCacheMaxBytes || len(cache.matchers) != 0 || cache.bytes != 0 {
		t.Fatalf("large key retained: charge %d, entries %d, bytes %d", first.cacheBytes, len(cache.matchers), cache.bytes)
	}
	second, err := cache.compile(pattern, ComparatorASCIICaseMap)
	if err != nil || first == second {
		t.Fatalf("uncached fallback: same matcher %v, err %v", first == second, err)
	}
	matches, err := second.FindSubmatch(context.Background(), "K001É")
	if err != nil || len(matches) != 401 || matches[0] != "K001É" || matches[1] != "É" {
		t.Fatalf("fallback captures %q, err %v", matches, err)
	}
}

func TestRegexCacheChargesExpandedUnicodeText(t *testing.T) {
	pattern := "^" + strings.Repeat(`\p{Lu}`, 20) + "$"
	var cache regexCache
	matcher, err := cache.compile(pattern, ComparatorASCIICaseMap)
	if err != nil {
		t.Fatal(err)
	}
	if matcher.cacheBytes <= len(pattern)+matcher.size*50 {
		t.Fatalf("charge omits Unicode storage: %d bytes, %d units", matcher.cacheBytes, matcher.size)
	}
	if cache.bytes > regexCacheMaxBytes {
		t.Fatalf("cache charged beyond budget: %d", cache.bytes)
	}
}
