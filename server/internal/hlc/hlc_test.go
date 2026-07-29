package hlc

import (
	"testing"
	"time"
)

func TestFormatParseRoundTrip(t *testing.T) {
	c := Clock{WallMs: 1700000000000, Counter: 7, DeviceID: "device-a"}
	parsed, err := Parse(c.String())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if parsed != c {
		t.Fatalf("round trip mismatch: got %+v, want %+v", parsed, c)
	}
}

func TestCompareOrdersByWallTimeFirst(t *testing.T) {
	earlier := Clock{WallMs: 100, Counter: 999, DeviceID: "z"}
	later := Clock{WallMs: 200, Counter: 0, DeviceID: "a"}
	if Compare(earlier.String(), later.String()) >= 0 {
		t.Fatal("expected earlier wall time to sort first regardless of counter/deviceID")
	}
}

func TestCompareOrdersByCounterWhenWallTimeTies(t *testing.T) {
	a := Clock{WallMs: 100, Counter: 1, DeviceID: "z"}
	b := Clock{WallMs: 100, Counter: 2, DeviceID: "a"}
	if Compare(a.String(), b.String()) >= 0 {
		t.Fatal("expected lower counter to sort first when wall time ties")
	}
}

func TestCompareTieBreaksOnDeviceID(t *testing.T) {
	a := Clock{WallMs: 100, Counter: 1, DeviceID: "device-a"}
	b := Clock{WallMs: 100, Counter: 1, DeviceID: "device-b"}
	if Compare(a.String(), b.String()) >= 0 {
		t.Fatal("expected deviceID to break ties deterministically")
	}
	if Compare(b.String(), a.String()) <= 0 {
		t.Fatal("expected comparison to be antisymmetric")
	}
}

func TestNextBumpsCounterWithinSameMillisecond(t *testing.T) {
	now := time.UnixMilli(1700000000000)
	last := Clock{WallMs: now.UnixMilli(), Counter: 5, DeviceID: "device-a"}
	next := Next(last, "device-a", now)
	if next.WallMs != last.WallMs {
		t.Fatalf("expected wall time to stay put, got %d want %d", next.WallMs, last.WallMs)
	}
	if next.Counter != last.Counter+1 {
		t.Fatalf("expected counter to bump, got %d want %d", next.Counter, last.Counter+1)
	}
}

func TestNextResetsCounterOnNewMillisecond(t *testing.T) {
	last := Clock{WallMs: 1700000000000, Counter: 5, DeviceID: "device-a"}
	later := time.UnixMilli(1700000000500)
	next := Next(last, "device-a", later)
	if next.WallMs != later.UnixMilli() {
		t.Fatalf("expected wall time to advance, got %d want %d", next.WallMs, later.UnixMilli())
	}
	if next.Counter != 0 {
		t.Fatalf("expected counter reset to 0, got %d", next.Counter)
	}
}

func TestNextIsStrictlyMonotonic(t *testing.T) {
	last := Clock{}
	deviceID := "device-a"
	now := time.UnixMilli(1700000000000)
	for i := 0; i < 5; i++ {
		next := Next(last, deviceID, now) // same "now" every time simulates a fast loop
		if Compare(next.String(), last.String()) <= 0 {
			t.Fatalf("expected strictly increasing clocks, got %v after %v", next, last)
		}
		last = next
	}
}
