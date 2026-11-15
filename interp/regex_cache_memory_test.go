package interp

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// Rune ranges cost memory as well as instructions. Forty small programs with
// large Unicode classes retain more than 20 MB if only instructions are counted.
// Stop retaining keys within the cache's memory budget, but keep matching them.
func TestRegexCacheBoundsUnicodeTables(t *testing.T) {
	for _, comparator := range []Comparator{ComparatorOctet, ComparatorASCIICaseMap, ComparatorUnicodeCaseMap} {
		t.Run(string(comparator), func(t *testing.T) {
			var c regexCache
			var last string
			for i := 0; i < 40; i++ {
				last = strings.Repeat(`\p{L}`, 100) + fmt.Sprintf("x%03d", i)
				if _, err := c.compile(last, comparator); err != nil {
					t.Fatal(err)
				}
			}
			if len(c.matchers) > 20 {
				t.Fatalf("retained %d Unicode-heavy keys; instruction count alone misses their rune tables", len(c.matchers))
			}
			if c.bytes > regexCacheMaxBytes {
				t.Fatalf("cache charge %d exceeds %d bytes", c.bytes, regexCacheMaxBytes)
			}
			a, err := c.compile(last, comparator)
			if err != nil {
				t.Fatal(err)
			}
			b, _ := c.compile(last, comparator)
			if a == b {
				t.Fatal("a key past the memory budget is retained")
			}
			if ok, err := a.Match(context.Background(), strings.Repeat("A", 100)+"x039"); !ok || err != nil {
				t.Fatalf("uncached Unicode key: match %v, %v", ok, err)
			}
		})
	}
}
