<script lang="ts">
  import type { Note } from './types'
  import { restoreNote, purgeNoteForever } from './stores/notes'
  import { noteColorVar } from './colors'

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
</script>

<!-- Trash is read-only (CONTEXT.md) — no editable inputs, just a display. -->
<div class="card" style="background: {cardBg}">
  <div class="title">{note.title || '(untitled)'}</div>
  {#if note.kind === 'text'}
    <p class="body">{note.body}</p>
  {:else}
    <ul class="items">
      {#each note.items as item (item.id)}
        <li class:checked={item.checked}>{item.text || '(empty item)'}</li>
      {/each}
    </ul>
  {/if}

  <div class="footer">
    <button on:click={() => restoreNote(note)}>Restore</button>
    <button class:confirming on:click={confirmPurge} on:mouseleave={() => (confirming = false)}>
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
    opacity: 0.8;
    color: var(--text);
  }
  .title {
    font-weight: 600;
    color: var(--text-h);
  }
  .body {
    margin: 0;
    white-space: pre-wrap;
  }
  .items {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
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
