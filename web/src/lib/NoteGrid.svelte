<script lang="ts">
  import type { Note } from './types'
  import NoteCard from './NoteCard.svelte'

  export let notesList: Note[]
  export let view: 'main' | 'archive' = 'main'
  export let onReorder: (list: Note[], draggedId: string, targetIndex: number) => void

  let draggedId: string | null = null
  let dragOverIndex: number | null = null

  function onDrop(e: DragEvent, index: number) {
    e.preventDefault()
    if (draggedId) onReorder(notesList, draggedId, index)
    draggedId = null
    dragOverIndex = null
  }
</script>

<div class="grid">
  {#each notesList as note, i (note.id)}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="card-slot"
      class:drag-over={dragOverIndex === i && draggedId !== note.id}
      on:dragover={(e) => { e.preventDefault(); dragOverIndex = i }}
      on:dragleave={() => { if (dragOverIndex === i) dragOverIndex = null }}
      on:drop={(e) => onDrop(e, i)}
    >
      <NoteCard {note} {view} onDragStart={() => (draggedId = note.id)} />
    </div>
  {/each}
</div>

<style>
  .grid {
    columns: 4 240px;
    column-gap: 1rem;
  }
  .grid > :global(*) {
    margin-bottom: 1rem;
  }
  .card-slot {
    break-inside: avoid;
  }
  .card-slot.drag-over {
    outline: 2px dashed var(--text);
    outline-offset: 2px;
    border-radius: 8px;
  }
</style>
