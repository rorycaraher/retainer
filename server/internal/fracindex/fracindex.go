// Package fracindex implements fractional indexing (LexoRank-style):
// KeyBetween generates a position key that sorts strictly between two
// neighbors using plain lexicographic string comparison — the same
// comparison SQLite's ORDER BY and the client's plain string `<` already
// use elsewhere. This means concurrent inserts (or a concurrent drag) never
// require renumbering existing siblings, and a dragged note's new Position
// participates in field_clocks like any other scalar field, so concurrent
// drags resolve via HLC (see the plan's "Position/ordering" section and
// docs/adr/0001). Shared by notesvc (creation defaults) and syncsvc (drag
// reorder mutations).
package fracindex

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var digitValue = func() [256]int {
	var m [256]int
	for i := range m {
		m[i] = -1
	}
	for i := 0; i < len(alphabet); i++ {
		m[alphabet[i]] = i
	}
	return m
}()

const base = len(alphabet)

// KeyBetween returns a key k such that a < k < b under plain lexicographic
// string comparison. a == "" means "no lower neighbor" (k is the smallest);
// b == "" means "no upper neighbor" (k is the largest); both empty seeds the
// very first key in an empty list. Callers must ensure a < b when both are
// non-empty (i.e. a and b really are adjacent siblings).
//
// Precondition: neither a nor b may end in the alphabet's minimum digit
// ('0'). If a is a strict prefix of b and b's only extra character is '0'
// (e.g. a="1", b="10"), no string strictly between them exists under plain
// lexicographic comparison — there is no character sorting below '0'. Every
// key this function returns is guaranteed to satisfy the precondition (see
// the trailing-digit guard below), so as long as all stored keys originate
// from here, the precondition holds for every future call.
func KeyBetween(a, b string) string {
	var result []byte
	i := 0
	// Once true, b no longer constrains the walk: a's digit at some earlier
	// position already fell strictly below b's digit there, which decides
	// the comparison regardless of anything appended afterward. b can have
	// more real characters beyond that point (it doesn't need to run out),
	// so this has to be tracked explicitly rather than inferred from length.
	bFree := false

	for {
		da := -1
		if i < len(a) {
			da = digitValue[a[i]]
		}
		db := base
		if !bFree && i < len(b) {
			db = digitValue[b[i]]
		}

		if da == db {
			result = append(result, alphabet[da])
			i++
			continue
		}

		gap := db - da
		if gap > 1 {
			mid := da + gap/2
			result = append(result, alphabet[mid])
			break
		}

		// gap == 1: no room for a distinct digit at this position — commit
		// to whichever side is still real and go one position deeper.
		if da >= 0 {
			// Copying a's digit is strictly < db at this position, which
			// already decides result < b for good, regardless of b's
			// remaining characters (a can supply more real digits later,
			// since exhaustion there is permanent once i passes its length).
			result = append(result, alphabet[da])
			bFree = true
		} else {
			// a is already exhausted (permanently, since len(a) is fixed) —
			// any character appended here already makes result > a via the
			// prefix rule, so just match b's digit and keep looking for
			// room below b at a deeper position.
			result = append(result, alphabet[db])
		}
		i++
	}

	key := string(result)
	// Never let a generated key end in the alphabet's minimum digit: doing
	// so can make it the unsatisfiable "b is exactly a-plus-trailing-zero"
	// case for some *future* KeyBetween call against this key (there is
	// provably no valid string between e.g. "1" and "10"). Appending one
	// more character is always safe — the decisive digit above already
	// settled the comparison, so anything appended after it doesn't change
	// the ordering.
	if key[len(key)-1] == alphabet[0] {
		key += string(alphabet[base/2])
	}
	return key
}
