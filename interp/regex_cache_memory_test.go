package interp

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// Rune ranges cost memory as well as instructions. Unicode-heavy programs must
// count their rune ranges. Retain only within the weighted cache budget while
// continuing to match keys that do not fit.
func TestRegexCacheBoundsUnicodeTables(t *testing.T) {
	for _, comparator := range []Comparator{ComparatorOctet, ComparatorASCIICaseMap, ComparatorUnicodeCaseMap} {
		t.Run(string(comparator), func(t *testing.T) {
			var c regexCache
			var last string
			for i := 0; i < 40; i++ {
				last = strings.Repeat(`\p{L}`, 50) + fmt.Sprintf("x%03d", i)
				if _, err := c.compile(last, comparator); err != nil {
					t.Fatal(err)
				}
			}
			if len(c.matchers) > 20 {
				t.Fatalf("retained %d Unicode-heavy keys; instruction count alone misses their rune tables", len(c.matchers))
			}
			if c.size > regexCacheMaxSize {
				t.Fatalf("cache cost %d exceeds %d units", c.size, regexCacheMaxSize)
			}
			a, err := c.compile(last, comparator)
			if err != nil {
				t.Fatal(err)
			}
			b, _ := c.compile(last, comparator)
			if a == b {
				t.Fatal("a key past the retention budget is retained")
			}
			if ok, err := a.Match(context.Background(), strings.Repeat("A", 50)+"x039"); !ok || err != nil {
				t.Fatalf("uncached Unicode key: match %v, %v", ok, err)
			}
		})
	}
}
