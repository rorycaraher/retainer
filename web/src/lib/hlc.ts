// Hybrid Logical Clock stamps, matching the Go server's encoding exactly
// (fixed-width zero-padded segments so lexicographic string compare equals
// correct HLC compare) — see internal/hlc on the server and docs/adr/0001.

interface Clock {
  wallMs: number
  counter: number
  deviceId: string
}

function pad(n: number, width: number): string {
  return n.toString().padStart(width, '0')
}

function format(c: Clock): string {
  return `${pad(c.wallMs, 20)}-${pad(c.counter, 10)}-${c.deviceId}`
}

let last: Clock = { wallMs: 0, counter: 0, deviceId: '' }

// next returns a fresh, strictly-increasing HLC stamp for deviceId.
export function next(deviceId: string): string {
  const wallMs = Date.now()
  last = wallMs <= last.wallMs
    ? { wallMs: last.wallMs, counter: last.counter + 1, deviceId }
    : { wallMs, counter: 0, deviceId }
  return format(last)
}
