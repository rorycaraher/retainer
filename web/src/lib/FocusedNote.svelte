<script lang="ts">
  import { get } from 'svelte/store'
  import type { ChecklistItem, Note } from './types'
  import {
    setNoteField,
    togglePinned,
    toggleArchived,
    trashNote,
    addItem,
    addItemAfter,
    setItemField,
    removeItem,
    reorderItem,
    attachLabel,
    detachLabel,
    justCreatedId,
    justCreatedItemId,
    restoreNote,
    purgeNoteForever,
    unfocusNote,
  } from './stores/notes'
  import { labels } from './stores/labels'
  import { NOTE_COLORS, noteColorVar } from './colors'
  import { audioUrl } from './api/rest'

  export let note: Note
  // Snapshot of which list this note was opened from (App.svelte passes
  // $focusedNoteContext.view, frozen at focus time) — see stores/notes.ts.
  export let view: 'main' | 'archive' | 'trash' = 'main'

  $: readOnly = view === 'trash'

  // Same resync guard as the old always-editable card (see stores/notes.ts
  // reconcile): don't let an incoming sync update clobber mid-typing.
  let title = note.title
  let body = note.body
  let titleFocused = false
  let bodyFocused = false
  $: if (!titleFocused) title = note.title
  $: if (!bodyFocused) body = note.body

  let draggedItemId: string | null = null
  let itemDragOverIndex: number | null = null
  let showLabelPicker = false
  let showColorPicker = false
  let showCompleted = false
  let confirmingPurge = false

  $: attachedLabels = $labels.filter((l) => note.labelIds.includes(l.id))
  $: cardBg = noteColorVar(note.color)
  $: uncheckedItems = note.items.filter((it) => !it.checked)
  $: checkedItems = note.items.filter((it) => it.checked)

  // The note has left the list it was opened from (its own trash/archive
  // buttons below, or a concurrent edit synced in from another device) —
  // Focused no longer makes sense in this context, so close.
  $: if (view === 'main' && (note.archived || note.trashedAt)) unfocusNote()
  $: if (view === 'archive' && (!note.archived || note.trashedAt)) unfocusNote()
  $: if (view === 'trash' && !note.trashedAt) unfocusNote()

  function autofocusIfNew(node: HTMLInputElement) {
    if (get(justCreatedId) === note.id) {
      node.focus()
      justCreatedId.set(null)
    }
  }

  function autofocusIfNewItem(node: HTMLInputElement, itemId: string) {
    if (get(justCreatedItemId) === itemId) {
      node.focus()
      justCreatedItemId.set(null)
    }
  }

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

  function saveTitle() {
    titleFocused = false
    if (title !== note.title) setNoteField(note, 'title', title)
  }

  function saveBody() {
    bodyFocused = false
    if (body !== note.body) setNoteField(note, 'body', body)
  }

  function itemText(item: ChecklistItem, value: string) {
    setItemField(note, item, 'text', value)
  }

  function itemChecked(item: ChecklistItem, value: boolean) {
    setItemField(note, item, 'checked', value)
  }

  function onItemKeydown(e: KeyboardEvent, item: ChecklistItem) {
    if (e.key !== 'Enter') return
    e.preventDefault()
    const value = (e.currentTarget as HTMLInputElement).value
    if (value !== item.text) itemText(item, value)
    addItemAfter(note, item)
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

  function resizeToFit(node: HTMLTextAreaElement) {
    node.style.height = 'auto'
    node.style.height = node.scrollHeight + 'px'
  }
  function autogrow(node: HTMLTextAreaElement, value: string) {
    resizeToFit(node)
    return { update: () => resizeToFit(node) }
  }

  function confirmPurge() {
    if (confirmingPurge) {
      purgeNoteForever(note)
    } else {
      confirmingPurge = true
    }
  }

  // Always flush any in-progress edit before closing (blur can't be relied
  // on for a keyboard close — Escape doesn't itself move focus).
  function close() {
    if (!readOnly) {
      saveTitle()
      saveBody()
    }
    unfocusNote()
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close()
  }
</script>

<svelte:window on:keydown={onKeydown} />

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div class="backdrop" on:click={close}>
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="panel" style="background: {cardBg}" on:click|stopPropagation>
    <div class="panel-head">
      {#if readOnly}
        <div class="title">{note.title || '(untitled)'}</div>
      {:else}
        <input class="title" placeholder="Title" bind:value={title} on:focus={() => (titleFocused = true)} on:blur={saveTitle} use:autofocusIfNew />
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div class="color-picker-wrap">
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
      {/if}
      <button class="icon-btn close-btn" on:click={close} title="Close" aria-label="Close">✕</button>
    </div>

    {#if !readOnly}
      <div class="labels-row">
        {#each attachedLabels as label (label.id)}
          <button class="chip" on:click={() => detachLabel(note, label.id)} title="Click to remove">{label.name} ×</button>
        {/each}
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div class="label-picker-wrap">
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
    {/if}

    <div class="content">
      {#if note.kind === 'text'}
        {#if readOnly}
          <p class="body">{note.body}</p>
        {:else}
          <textarea placeholder="Note" bind:value={body} on:focus={() => (bodyFocused = true)} on:blur={saveBody} on:input={(e) => resizeToFit(e.currentTarget)} use:autogrow={body}></textarea>
        {/if}
      {:else if note.kind === 'audio'}
        <!-- svelte-ignore a11y_media_has_caption -->
        <audio controls preload="metadata" src={audioUrl(note.id)}></audio>
      {:else if readOnly}
        <ul class="items">
          {#each note.items as item (item.id)}
            <li class:checked={item.checked}>{item.text || '(empty item)'}</li>
          {/each}
        </ul>
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
              <span class="drag-handle small" role="button" tabindex="0" draggable="true" on:dragstart={() => (draggedItemId = item.id)} title="Drag to reorder">⠿</span>
              <input type="checkbox" checked={item.checked} on:change={(e) => itemChecked(item, e.currentTarget.checked)} />
              <input
                class="item-text"
                value={item.text}
                on:blur={(e) => itemText(item, e.currentTarget.value)}
                on:keydown={(e) => onItemKeydown(e, item)}
                use:autofocusIfNewItem={item.id}
              />
              <button class="remove" on:click={() => removeItem(note, item)}>&times;</button>
            </li>
          {/each}
        </ul>
        <button class="add-item" on:click={() => addItem(note)}>+ Add item</button>

        {#if checkedItems.length > 0}
          <div class="completed-section">
            <button class="completed-toggle" on:click={() => (showCompleted = !showCompleted)}>
              <span class="chevron" class:open={showCompleted}>▸</span>
              Completed ({checkedItems.length})
            </button>
            {#if showCompleted}
              <ul class="items completed-items">
                {#each checkedItems as item (item.id)}
                  <li>
                    <input type="checkbox" checked={item.checked} on:change={(e) => itemChecked(item, e.currentTarget.checked)} />
                    <input
                      class="item-text checked"
                      value={item.text}
                      on:blur={(e) => itemText(item, e.currentTarget.value)}
                      on:keydown={(e) => onItemKeydown(e, item)}
                    />
                    <button class="remove" on:click={() => removeItem(note, item)}>&times;</button>
                  </li>
                {/each}
              </ul>
            {/if}
          </div>
        {/if}
      {/if}
    </div>

    <div class="footer">
      {#if readOnly}
        <button on:click={() => restoreNote(note)}>Restore</button>
        <button class:confirming={confirmingPurge} on:click={confirmPurge} on:mouseleave={() => (confirmingPurge = false)}>
          {confirmingPurge ? 'Click again to delete forever' : 'Delete forever'}
        </button>
      {:else if view === 'main'}
        <button class:active={note.pinned} on:click={() => togglePinned(note)}>{note.pinned ? 'Unpin' : 'Pin'}</button>
        <button on:click={() => toggleArchived(note)}>Archive</button>
        <button on:click={() => trashNote(note)}>Trash</button>
      {:else}
        <button on:click={() => toggleArchived(note)}>Unarchive</button>
        <button on:click={() => trashNote(note)}>Trash</button>
      {/if}
    </div>
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    z-index: 100;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .panel {
    width: 90vw;
    height: 90vh;
    max-width: 900px;
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    overflow-y: auto;
  }
  @media (max-width: 640px) {
    .backdrop {
      background: transparent;
    }
    .panel {
      width: 100vw;
      height: 100vh;
      max-width: none;
      border: none;
      border-radius: 0;
    }
  }
  .panel-head {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .title {
    flex: 1;
    font-weight: 600;
    font-size: 1.1rem;
    border: none;
    background: transparent;
    color: var(--text-h);
  }
  .content {
    flex: 1;
  }
  textarea {
    width: 100%;
    height: 100%;
    border: none;
    resize: none;
    overflow: hidden;
    min-height: 8rem;
    font-family: inherit;
    font-size: 1rem;
    background: transparent;
    color: var(--text);
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
  .items li.checked {
    text-decoration: line-through;
    opacity: 0.7;
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
  .item-text {
    flex: 1;
    border: none;
    background: transparent;
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
  .add-item {
    align-self: flex-start;
    border: none;
    background: transparent;
    cursor: pointer;
    opacity: 0.7;
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
  .footer button.confirming {
    color: var(--danger);
    font-weight: 600;
  }
  .icon-btn {
    border: none;
    background: transparent;
    cursor: pointer;
    font-size: 1rem;
    line-height: 1;
    flex-shrink: 0;
  }
  .close-btn {
    font-size: 1.2rem;
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
