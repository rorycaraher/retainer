// Package hlc implements Hybrid Logical Clocks: per-field edit stamps that
// order concurrent writes correctly without trusting wall-clock time across
// devices (see docs/adr/0001).
package hlc

import (
	"fmt"
	"strings"
	"time"
)

// Clock is one HLC stamp: wall-clock milliseconds, a logical counter that
// breaks ties within the same millisecond, and the device that produced it
// (the final tie-break for genuinely simultaneous stamps).
type Clock struct {
	WallMs   int64
	Counter  uint64
	DeviceID string
}

// String encodes the clock as a fixed-width, zero-padded string so that
// plain lexicographic string comparison equals correct HLC comparison —
// this is also what gets stored in a note/item's field_clocks JSON.
func (c Clock) String() string {
	return fmt.Sprintf("%020d-%010d-%s", c.WallMs, c.Counter, c.DeviceID)
}

// Parse decodes a string produced by Clock.String.
func Parse(s string) (Clock, error) {
	parts := strings.SplitN(s, "-", 3)
	if len(parts) != 3 {
		return Clock{}, fmt.Errorf("hlc: malformed clock %q", s)
	}
	var wallMs int64
	if _, err := fmt.Sscanf(parts[0], "%d", &wallMs); err != nil {
		return Clock{}, fmt.Errorf("hlc: malformed wall time in %q: %w", s, err)
	}
	var counter uint64
	if _, err := fmt.Sscanf(parts[1], "%d", &counter); err != nil {
		return Clock{}, fmt.Errorf("hlc: malformed counter in %q: %w", s, err)
	}
	return Clock{WallMs: wallMs, Counter: counter, DeviceID: parts[2]}, nil
}

// Compare returns -1, 0, or 1 as a sorts before, equal to, or after b.
// Both are fixed-width zero-padded strings, so this is plain lexicographic
// string comparison — kept as a named function so callers don't have to
// know that encoding detail.
func Compare(a, b string) int {
	return strings.Compare(a, b)
}

// Next advances a per-device clock past both the device's own last-issued
// stamp and the current wall time, per the standard HLC algorithm: if wall
// time has moved forward, use it with a fresh counter; otherwise stay on the
// last wall time and bump the counter so stamps issued within the same
// millisecond still sort strictly after one another.
func Next(last Clock, deviceID string, now time.Time) Clock {
	wallMs := now.UnixMilli()
	if wallMs <= last.WallMs {
		return Clock{WallMs: last.WallMs, Counter: last.Counter + 1, DeviceID: deviceID}
	}
	return Clock{WallMs: wallMs, Counter: 0, DeviceID: deviceID}
}
