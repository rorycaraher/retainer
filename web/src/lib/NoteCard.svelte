<script lang="ts">
  import type { ChecklistItem, Note } from './types'
  import { focusNote, togglePinned, toggleArchived, trashNote, setNoteField, setItemField, removeItem, reorderItem, attachLabel, detachLabel } from './stores/notes'
  import { labels } from './stores/labels'
  import { NOTE_COLORS, noteColorVar } from './colors'
  import { audioUrl } from './api/rest'

  export let note: Note
  export let onDragStart: () => void = () => {}
  export let view: 'main' | 'archive' = 'main'

  let draggedItemId: string | null = null
  let itemDragOverIndex: number | null = null
  let showLabelPicker = false
  let showColorPicker = false
  let showCompleted = false

  $: attachedLabels = $labels.filter((l) => note.labelIds.includes(l.id))
  $: cardBg = noteColorVar(note.color)
  // note.items is kept sorted by position, but checked/unchecked items can be
  // interleaved within it — split here for display, completed items tucked
  // under a collapsible section instead of interspersed with active ones.
  $: uncheckedItems = note.items.filter((it) => !it.checked)
  $: checkedItems = note.items.filter((it) => it.checked)

  function toggleLabel(labelId: string) {
    if (note.labelIds.includes(labelId)) {
      detachLabel(note, labelId)
    } else {
      attachLabel(note, labelId)
    }
  }

  function setColor(colorKey: string) {
    setNoteField(note, 'color', colorKey)
    showColorPicker = false
  }

  function itemChecked(item: ChecklistItem, value: boolean) {
    setItemField(note, item, 'checked', value)
  }

  function onItemDragOver(e: DragEvent, index: number) {
    e.preventDefault()
    itemDragOverIndex = index
  }

  function onItemDrop(e: DragEvent, index: number) {
    e.preventDefault()
    if (draggedItemId) reorderItem(note, uncheckedItems, draggedItemId, index)
    draggedItemId = null
    itemDragOverIndex = null
  }

  function open() {
    focusNote(note.id, view)
  }

  function openOnKey(e: KeyboardEvent) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      open()
    }
  }
</script>

<!-- Card itself opens the note (Focused, CONTEXT.md) on click; every quick
action below (pin/archive/trash/color/label/checkbox/reorder/audio) stops
propagation so it acts in place instead of also opening the note. -->
<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div class="card" style="background: {cardBg}" role="button" tabindex="0" on:click={open} on:keydown={openOnKey}>
  <div class="card-head">
    <span class="drag-handle" role="button" tabindex="0" draggable="true" on:click|stopPropagation on:dragstart={onDragStart} title="Drag to reorder">⠿</span>
    <div class="title">{note.title || '(untitled)'}</div>
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="color-picker-wrap" on:click|stopPropagation>
      <button class="icon-btn" on:click={() => (showColorPicker = !showColorPicker)} title="Change color">🎨</button>
      {#if showColorPicker}
        <ul class="color-picker" on:mouseleave={() => (showColorPicker = false)}>
          {#each NOTE_COLORS as color (color.key)}
            <li>
              <button
                class="swatch"
                class:selected={note.color === color.key || (!note.color && color.key === 'default')}
                style="background: var(--note-{color.key}-bg)"
                title={color.name}
                on:click={() => setColor(color.key)}
              ></button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  </div>

  <div class="labels-row">
    {#each attachedLabels as label (label.id)}
      <button class="chip" on:click|stopPropagation={() => detachLabel(note, label.id)} title="Click to remove">{label.name} ×</button>
    {/each}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="label-picker-wrap" on:click|stopPropagation>
      <button class="add-label" on:click={() => (showLabelPicker = !showLabelPicker)}>+ Label</button>
      {#if showLabelPicker}
        <ul class="label-picker" on:mouseleave={() => (showLabelPicker = false)}>
          {#each $labels as label (label.id)}
            <li>
              <label>
                <input type="checkbox" checked={note.labelIds.includes(label.id)} on:change={() => toggleLabel(label.id)} />
                {label.name}
              </label>
            </li>
          {:else}
            <li class="empty">No labels yet</li>
          {/each}
        </ul>
      {/if}
    </div>
  </div>

  {#if note.kind === 'text'}
    <p class="body">{note.body}</p>
  {:else if note.kind === 'audio'}
    <!-- svelte-ignore a11y_no_static_element_interactions a11y_media_has_caption -->
    <div on:click|stopPropagation>
      <audio controls preload="metadata" src={audioUrl(note.id)}></audio>
    </div>
  {:else}
    <ul class="items">
      {#each uncheckedItems as item, i (item.id)}
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <li
          class:drag-over={itemDragOverIndex === i && draggedItemId !== item.id}
          on:dragover={(e) => onItemDragOver(e, i)}
          on:dragleave={() => { if (itemDragOverIndex === i) itemDragOverIndex = null }}
          on:drop={(e) => onItemDrop(e, i)}
        >
          <span class="drag-handle small" role="button" tabindex="0" draggable="true" on:click|stopPropagation on:dragstart={() => (draggedItemId = item.id)} title="Drag to reorder">⠿</span>
          <input type="checkbox" checked={item.checked} on:click|stopPropagation on:change={(e) => itemChecked(item, e.currentTarget.checked)} />
          <span class="item-text">{item.text || '(empty item)'}</span>
          <button class="remove" on:click|stopPropagation={() => removeItem(note, item)}>&times;</button>
        </li>
      {/each}
    </ul>

    {#if checkedItems.length > 0}
      <div class="completed-section">
        <button class="completed-toggle" on:click|stopPropagation={() => (showCompleted = !showCompleted)}>
          <span class="chevron" class:open={showCompleted}>▸</span>
          Completed ({checkedItems.length})
        </button>
        {#if showCompleted}
          <ul class="items completed-items">
            {#each checkedItems as item (item.id)}
              <li>
                <input type="checkbox" checked={item.checked} on:click|stopPropagation on:change={(e) => itemChecked(item, e.currentTarget.checked)} />
                <span class="item-text checked">{item.text || '(empty item)'}</span>
                <button class="remove" on:click|stopPropagation={() => removeItem(note, item)}>&times;</button>
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    {/if}
  {/if}

  <div class="footer">
    {#if view === 'main'}
      <button class:active={note.pinned} on:click|stopPropagation={() => togglePinned(note)}>{note.pinned ? 'Unpin' : 'Pin'}</button>
      <button on:click|stopPropagation={() => toggleArchived(note)}>Archive</button>
    {:else}
      <button on:click|stopPropagation={() => toggleArchived(note)}>Unarchive</button>
    {/if}
    <button on:click|stopPropagation={() => trashNote(note)}>Trash</button>
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
  }
  .card-head {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .drag-handle {
    cursor: grab;
    opacity: 0.4;
    user-select: none;
    flex-shrink: 0;
  }
  .drag-handle:active {
    cursor: grabbing;
  }
  .drag-handle.small {
    font-size: 0.85em;
  }
  .title {
    flex: 1;
    font-weight: 600;
    font-size: 1rem;
    color: var(--text-h);
  }
  .body {
    margin: 0;
    white-space: pre-wrap;
    color: var(--text);
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
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .items li.drag-over {
    outline: 2px dashed var(--text);
    outline-offset: 1px;
    border-radius: 4px;
  }
  .item-text {
    flex: 1;
    color: var(--text);
  }
  .completed-items .item-text.checked {
    text-decoration: line-through;
    opacity: 0.6;
  }
  .remove {
    border: none;
    background: transparent;
    cursor: pointer;
    color: var(--text);
  }
  .completed-section {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  .completed-toggle {
    align-self: flex-start;
    display: flex;
    align-items: center;
    gap: 0.3rem;
    border: none;
    background: transparent;
    cursor: pointer;
    color: var(--text);
    opacity: 0.7;
    font-size: 0.85rem;
    padding: 0;
  }
  .chevron {
    display: inline-block;
    transition: transform 0.1s ease;
  }
  .chevron.open {
    transform: rotate(90deg);
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
  .footer button.active {
    font-weight: 600;
    color: var(--accent);
  }
  .icon-btn {
    border: none;
    background: transparent;
    cursor: pointer;
    font-size: 1rem;
    line-height: 1;
    flex-shrink: 0;
  }
  .color-picker-wrap {
    position: relative;
  }
  .color-picker {
    position: absolute;
    top: 100%;
    right: 0;
    z-index: 10;
    list-style: none;
    display: flex;
    gap: 0.3rem;
    margin: 0.2rem 0 0;
    padding: 0.4rem;
    background: var(--card-bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.25);
  }
  .swatch {
    width: 1.4rem;
    height: 1.4rem;
    border-radius: 50%;
    border: 1px solid var(--border);
    cursor: pointer;
    padding: 0;
  }
  .swatch.selected {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
  .labels-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.3rem;
    position: relative;
  }
  .chip {
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 0.1rem 0.5rem;
    font-size: 0.8rem;
    background: transparent;
    color: var(--text);
    cursor: pointer;
  }
  .add-label {
    border: none;
    background: transparent;
    font-size: 0.8rem;
    opacity: 0.6;
    cursor: pointer;
    color: var(--text);
  }
  .label-picker-wrap {
    position: relative;
  }
  .label-picker {
    position: absolute;
    top: 100%;
    left: 0;
    z-index: 10;
    list-style: none;
    margin: 0.2rem 0 0;
    padding: 0.3rem;
    background: var(--card-bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    min-width: 8rem;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.25);
  }
  .label-picker li {
    padding: 0.15rem 0.3rem;
    font-size: 0.85rem;
  }
  .label-picker li.empty {
    opacity: 0.6;
  }
  .label-picker label {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    cursor: pointer;
    color: var(--text);
  }
</style>
