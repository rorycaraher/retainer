# Retainer

A private, self-hosted notes and checklists app, synced instantly across a web app and native mobile apps (Android first, iOS later), behind sign-in. Single-user — one person, one pool of notes, no sharing or collaborators.

## Language

**Note**:
The top-level object a user creates. At any point in time, a Note is exactly one of two kinds — a Text Note or a Checklist — never both at once, but it can be converted between kinds later without losing its identity (text splits into items on newlines when converting to a Checklist; items join back into lines when converting to a Text Note).
_Avoid_: Item (too generic — see Checklist Item), Memo

**Checklist**:
A Note whose content is an ordered list of Checklist Items instead of a text body.
_Avoid_: List, Task list (implies a to-do app, which this is not)

**Checklist Item**:
A single line within a Checklist: text plus a checked/unchecked state.
_Avoid_: Task, To-do

**Pinned**:
A Note state that keeps it at the top of the main list, above unpinned Notes.

**Archived**:
A Note state that removes it from the main list without deleting it, kept in a separate Archive view. Mutually exclusive with Pinned — archiving a Pinned Note unpins it.

**Trash**:
A holding state for a deleted Note. A Trashed Note is read-only, not visible in the main list or Archive, and is permanently purged 7 days after being trashed.
_Avoid_: Deleted (ambiguous — could mean Trashed or permanently purged)

**Label**:
A first-class, user-managed entity (create/rename/delete in a settings screen) attached to zero or more Notes for filtering. Renaming a Label updates it everywhere; deleting one strips it from all Notes.
_Avoid_: Tag, Category

**Color**:
A background color assigned to a Note, chosen from a fixed palette.

**Position**:
A Note's place in the manually-ordered main list, user-controlled via drag-and-drop. Independent of Pinned/Archived/Trash (each of those views has its own ordering).
