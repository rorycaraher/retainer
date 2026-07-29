<script lang="ts">
  import { onMount } from 'svelte'
  import { labels, labelsError, loadLabels, createLabel, renameLabel, deleteLabel } from './stores/labels'

  let newName = ''
  let editingId: string | null = null
  let editingName = ''

  onMount(loadLabels)

  async function add() {
    if (!newName.trim()) return
    await createLabel(newName.trim())
    newName = ''
  }

  function startEdit(id: string, name: string) {
    editingId = id
    editingName = name
  }

  async function saveEdit() {
    if (editingId && editingName.trim()) {
      await renameLabel(editingId, editingName.trim())
    }
    editingId = null
  }

  async function remove(id: string) {
    await deleteLabel(id)
  }
</script>

<div class="labels-view">
  <div class="add-row">
    <input placeholder="New label name" bind:value={newName} on:keydown={(e) => e.key === 'Enter' && add()} />
    <button on:click={add}>Add</button>
  </div>

  {#if $labelsError}
    <p class="error">{$labelsError}</p>
  {/if}

  <ul class="label-list">
    {#each $labels as label (label.id)}
      <li>
        {#if editingId === label.id}
          <!-- svelte-ignore a11y_autofocus -->
          <input bind:value={editingName} on:blur={saveEdit} on:keydown={(e) => e.key === 'Enter' && saveEdit()} autofocus />
        {:else}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <span class="name" on:click={() => startEdit(label.id, label.name)}>{label.name}</span>
        {/if}
        <button class="remove" on:click={() => remove(label.id)}>Delete</button>
      </li>
    {:else}
      <li class="empty">No labels yet.</li>
    {/each}
  </ul>
</div>

<style>
  .labels-view {
    max-width: 400px;
  }
  .add-row {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 1rem;
  }
  .add-row input,
  .label-list input {
    flex: 1;
    padding: 0.4rem;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--card-bg);
    color: var(--text);
  }
  .add-row button {
    cursor: pointer;
  }
  .error {
    color: var(--danger);
  }
  .label-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .label-list li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    padding: 0.4rem 0.6rem;
    border: 1px solid var(--border);
    border-radius: 6px;
  }
  .label-list li.empty {
    opacity: 0.6;
    justify-content: center;
  }
  .name {
    cursor: pointer;
    flex: 1;
    color: var(--text);
  }
  .remove {
    border: none;
    background: transparent;
    cursor: pointer;
    opacity: 0.7;
    color: var(--text);
  }
</style>
