package fracindex

import (
	"math/rand"
	"sort"
	"testing"
)

func TestKeyBetweenEmptyEmptySeedsAKey(t *testing.T) {
	k := KeyBetween("", "")
	if k == "" {
		t.Fatal("expected a non-empty seed key")
	}
}

func TestKeyBetweenOrdering(t *testing.T) {
	cases := []struct {
		name string
		a, b string
	}{
		{"seed", "", ""},
		{"before", "", "5"},
		{"after", "5", ""},
		{"normal gap", "1", "9"},
		{"adjacent digits", "1", "2"},
		{"adjacent digits at start", "0", "1"},
		{"same prefix different length", "abc", "abd"},
		// NB: KeyBetween("1", "10") is deliberately not a case here — it
		// violates KeyBetween's documented precondition (b must not end in
		// the minimum digit) and has no valid solution. That precondition
		// is exactly what the trailing-digit guard exists to uphold for
		// every key this function itself produces.
		{"a is short, b has non-min digit next", "1", "105"},
		{"deep adjacency", "1z", "20"}, // 'z' = max digit in this alphabet
		{"both long, adjacent throughout tail", "100", "101"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			k := KeyBetween(c.a, c.b)
			if c.a != "" && !(c.a < k) {
				t.Fatalf("expected %q < %q (a=%q)", c.a, k, c.a)
			}
			if c.b != "" && !(k < c.b) {
				t.Fatalf("expected %q < %q (b=%q)", k, c.b, c.b)
			}
		})
	}
}

func TestKeyBetweenNeverEndsInMinimumDigit(t *testing.T) {
	// This is the invariant that keeps future KeyBetween calls solvable —
	// see the comment in fracindex.go for why a trailing minimum digit is
	// dangerous.
	cases := [][2]string{
		{"", ""},
		{"", "1"},
		{"0", ""},
		{"0", "1"},
		{"1", "2"},
		{"1", "10"},
	}
	for _, c := range cases {
		k := KeyBetween(c[0], c[1])
		if k[len(k)-1] == alphabet[0] {
			t.Fatalf("KeyBetween(%q, %q) = %q ends in the minimum digit", c[0], c[1], k)
		}
	}
}

func TestKeyBetweenRepeatedInsertionAtSameGapStaysOrdered(t *testing.T) {
	// Simulates repeatedly dragging a note to the very top of the list:
	// each new key must land strictly before the previous "first" key.
	lo, hi := "", "5"
	prev := hi
	for i := 0; i < 200; i++ {
		k := KeyBetween(lo, prev)
		if !(k < prev) {
			t.Fatalf("iteration %d: expected %q < %q", i, k, prev)
		}
		prev = k
	}
}

func TestKeyBetweenRepeatedInsertionAtEndStaysOrdered(t *testing.T) {
	lo, hi := "5", ""
	prev := lo
	for i := 0; i < 200; i++ {
		k := KeyBetween(prev, hi)
		if !(k > prev) {
			t.Fatalf("iteration %d: expected %q > %q", i, k, prev)
		}
		prev = k
	}
}

func TestKeyBetweenRepeatedInsertionInSameMiddleGapStaysOrdered(t *testing.T) {
	// Always inserting between the same fixed pair of neighbors — worst case
	// for the "gap == 1, go deeper" path since digits get squeezed each time.
	a, b := "1", "2"
	for i := 0; i < 200; i++ {
		k := KeyBetween(a, b)
		if !(a < k && k < b) {
			t.Fatalf("iteration %d: expected %q < %q < %q", i, a, k, b)
		}
		b = k // shrink the gap from above each time — the hardest direction
	}
}

func TestKeyBetweenRandomizedInsertionsProduceConsistentOrder(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	keys := []string{KeyBetween("", "")}

	for i := 0; i < 500; i++ {
		idx := rng.Intn(len(keys) + 1)
		var a, b string
		if idx > 0 {
			a = keys[idx-1]
		}
		if idx < len(keys) {
			b = keys[idx]
		}
		k := KeyBetween(a, b)
		if a != "" && !(a < k) {
			t.Fatalf("insert %d: expected %q < %q", i, a, k)
		}
		if b != "" && !(k < b) {
			t.Fatalf("insert %d: expected %q < %q", i, k, b)
		}
		keys = append(keys[:idx], append([]string{k}, keys[idx:]...)...)
	}

	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)
	for i := range keys {
		if keys[i] != sorted[i] {
			t.Fatalf("keys list is not in sorted order at index %d: %v", i, keys)
		}
	}
}
