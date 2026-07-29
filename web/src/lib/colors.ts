// Fixed palette (CONTEXT.md: Color is "chosen from a fixed palette"). Each
// key names a `--note-{key}-bg` custom property defined in app.css, with
// separate light/dark-mode values so cards stay readable in both themes.
export const NOTE_COLORS = [
  { key: 'default', name: 'Default' },
  { key: 'red', name: 'Red' },
  { key: 'orange', name: 'Orange' },
  { key: 'yellow', name: 'Yellow' },
  { key: 'green', name: 'Green' },
  { key: 'blue', name: 'Blue' },
  { key: 'purple', name: 'Purple' },
] as const

export type NoteColorKey = (typeof NOTE_COLORS)[number]['key']

export function noteColorVar(color: string): string {
  const key = NOTE_COLORS.some((c) => c.key === color) ? color : 'default'
  return `var(--note-${key}-bg)`
}
