// Per-tab device identity, deliberately stored in sessionStorage (not
// localStorage) so two tabs of the same browser act as two independent
// "devices" — this is what makes the two-tab manual sync test meaningful.
const KEY = 'retainer_device_id'

export function getDeviceId(): string {
  let id = sessionStorage.getItem(KEY)
  if (!id) {
    id = crypto.randomUUID()
    sessionStorage.setItem(KEY, id)
  }
  return id
}
