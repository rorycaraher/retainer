// TypeScript port of the server's internal/notesvc/fracindex.go — see that
// file for the full derivation and correctness reasoning. Both sides must
// produce values that sort correctly under plain string comparison (SQLite's
// ORDER BY and this client's `<` alike), but they don't need to be
// byte-for-byte identical implementations to interoperate correctly.

const alphabet = '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz'
const base = alphabet.length

function digitValue(ch: string): number {
  return alphabet.indexOf(ch)
}

// keyBetween returns a key k such that a < k < b under plain string
// comparison. a === '' means "no lower neighbor"; b === '' means "no upper
// neighbor"; both empty seeds the first key in an empty list.
//
// Precondition: neither a nor b may end in the alphabet's minimum digit
// ('0') — every key this function returns upholds that, so it's safe as
// long as all stored keys originate from here (see fracindex.go).
export function keyBetween(a: string, b: string): string {
  const result: string[] = []
  let i = 0
  let bFree = false

  for (;;) {
    let da = -1
    if (i < a.length) da = digitValue(a[i])
    let db = base
    if (!bFree && i < b.length) db = digitValue(b[i])

    if (da === db) {
      result.push(alphabet[da])
      i++
      continue
    }

    const gap = db - da
    if (gap > 1) {
      const mid = da + Math.floor(gap / 2)
      result.push(alphabet[mid])
      break
    }

    if (da >= 0) {
      result.push(alphabet[da])
      bFree = true
    } else {
      result.push(alphabet[db])
    }
    i++
  }

  let key = result.join('')
  if (key[key.length - 1] === alphabet[0]) {
    key += alphabet[Math.floor(base / 2)]
  }
  return key
}
