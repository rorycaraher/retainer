<script lang="ts">
  import type { Note } from './types'
  import { restoreNote, purgeNoteForever, focusNote } from './stores/notes'
  import { noteColorVar } from './colors'
  import { audioUrl } from './api/rest'

  export let note: Note

  let confirming = false
  $: cardBg = noteColorVar(note.color)

  function confirmPurge() {
    if (confirming) {
      purgeNoteForever(note)
    } else {
      confirming = true
    }
  }

  function open() {
    focusNote(note.id, 'trash')
  }

  function openOnKey(e: KeyboardEvent) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      open()
    }
  }
</script>

<!-- Trash is read-only (CONTEXT.md) — no editable inputs, just a display.
Clicking the card opens a read-only Focused view (CONTEXT.md); Restore/Delete
forever stop propagation so they act in place instead of also opening it. -->
<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div class="card" style="background: {cardBg}" role="button" tabindex="0" on:click={open} on:keydown={openOnKey}>
  <div class="title">{note.title || '(untitled)'}</div>
  {#if note.kind === 'text'}
    <p class="body">{note.body}</p>
  {:else if note.kind === 'audio'}
    <!-- svelte-ignore a11y_no_static_element_interactions a11y_media_has_caption -->
    <div on:click|stopPropagation>
      <audio controls preload="metadata" src={audioUrl(note.id)}></audio>
    </div>
  {:else}
    <ul class="items">
      {#each note.items as item (item.id)}
        <li class:checked={item.checked}>{item.text || '(empty item)'}</li>
      {/each}
    </ul>
  {/if}

  <div class="footer">
    <button on:click|stopPropagation={() => restoreNote(note)}>Restore</button>
    <button class:confirming on:click|stopPropagation={confirmPurge} on:mouseleave={() => (confirming = false)}>
      {confirming ? 'Click again to delete forever' : 'Delete forever'}
    </button>
  </div>
</div>

<style>
  .card {
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.75rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    cursor: pointer;
    text-align: left;
    opacity: 0.8;
    color: var(--text);
  }
  .title {
    font-weight: 600;
    color: var(--text-h);
    overflow-wrap: anywhere;
  }
  .body {
    margin: 0;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  audio {
    width: 100%;
  }
  .items {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  .items li {
    overflow-wrap: anywhere;
  }
  .items li.checked {
    text-decoration: line-through;
    opacity: 0.7;
  }
  .footer {
    display: flex;
    justify-content: flex-end;
    gap: 0.4rem;
  }
  .footer button {
    background: transparent;
    border: none;
    cursor: pointer;
    color: var(--text);
  }
  .footer button.confirming {
    color: var(--danger);
    font-weight: 600;
  }
</style>
