package interp

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
)

func retainedRegexCacheHeap() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// Repeated lookahead propagation through captures can duplicate a large
// Unicode class in the one-pass program despite a small leaf instruction cost.
func TestRegexCacheRetainedHeapCaptureClasses(t *testing.T) {
	// Warm parser/compiler tables before measuring cache-retained heap.
	_, err := compileSafeRegexFor(`^warm\p{L}$`, ComparatorOctet, DefaultRegexLimits)
	if err != nil {
		t.Fatal(err)
	}
	baseline := retainedRegexCacheHeap()
	var c regexCache
	for i := 0; i < 8; i++ {
		pattern := fmt.Sprintf("^k%03d", i) + strings.Repeat("(", 400) + `\p{L}` + strings.Repeat(")", 400) + "$"
		m, err := c.compile(pattern, ComparatorOctet)
		if err != nil {
			t.Fatalf("legal probe: %v", err)
		}
		if m == nil {
			t.Fatal("nil matcher")
		}
		matches, err := m.FindSubmatch(context.Background(), fmt.Sprintf("k%03dA", i))
		if err != nil || len(matches) != 401 {
			t.Fatalf("legal capture pattern: got %d matches, err %v", len(matches), err)
		}
		for _, capture := range matches[1:] {
			if capture != "A" {
				t.Fatalf("capture %q, want A", capture)
			}
		}
	}
	after := retainedRegexCacheHeap()
	t.Logf("retained delta=%d cache entries=%d", int64(after)-int64(baseline), len(c.matchers))
	// The advertised cache budget is 5 MiB. Allow 1 MiB of runtime and
	// allocator noise; admission must still bound actual retained memory.
	if after > baseline+6*1024*1024 {
		t.Errorf("cache retained %d bytes after GC, exceeds 5MiB plus 1 MiB allowance", after-baseline)
	}
	runtime.KeepAlive(&c)
}

// Owning bounded keys before compilation prevents a short substring from
// retaining a large source through the map key, matcher or regexp expression.
func TestRegexCacheRetainedHeapSubstringKeys(t *testing.T) {
	baseline := retainedRegexCacheHeap()
	build := func() regexCache {
		var c regexCache
		for i := 0; i < 4; i++ {
			backing := strings.Repeat("x", 8*1024*1024) + fmt.Sprintf("key%05d", i)
			pattern := backing[len(backing)-8:]
			if _, err := c.compile(pattern, ComparatorOctet); err != nil {
				t.Fatal(err)
			}
		}
		return c
	}
	c := build()
	after := retainedRegexCacheHeap()
	t.Logf("substring retained delta=%d cache entries=%d", int64(after)-int64(baseline), len(c.matchers))
	if after > baseline+6*1024*1024 {
		t.Errorf("substring cache retained %d bytes after GC", after-baseline)
	}
	runtime.KeepAlive(&c)
}

// ASCII folding expands short property escapes into long class expressions;
// both expression strings and one-pass rune tables survive with the matcher.
func TestRegexCacheRetainedHeapExpandedClasses(t *testing.T) {
	baseline := retainedRegexCacheHeap()
	var c regexCache
	for i := 0; i < 40; i++ {
		pattern := fmt.Sprintf("^k%03d", i) + strings.Repeat(`\p{Lu}`, 20) + "$"
		if _, err := c.compile(pattern, ComparatorASCIICaseMap); err != nil {
			t.Fatal(err)
		}
	}
	after := retainedRegexCacheHeap()
	t.Logf("expanded class retained delta=%d cache entries=%d", int64(after)-int64(baseline), len(c.matchers))
	if after > baseline+6*1024*1024 {
		t.Errorf("expanded-class cache retained %d bytes after GC", after-baseline)
	}
	runtime.KeepAlive(&c)
}
