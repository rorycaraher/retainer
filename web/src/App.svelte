<script lang="ts">
  import { onMount } from 'svelte'
  import {
    loadError,
    load,
    addTextNote,
    addChecklist,
    reorderNote,
    reorderArchivedNote,
    pinnedNotes,
    unpinnedNotes,
    archivedNotes,
    trashedNotes,
  } from './lib/stores/notes'
  import { isAuthed, checkAuth, logout } from './lib/stores/auth'
  import { loadLabels } from './lib/stores/labels'
  import { searchQuery, searchResults } from './lib/stores/search'
  import { wsConnected } from './lib/ws/client'
  import NoteGrid from './lib/NoteGrid.svelte'
  import NoteCard from './lib/NoteCard.svelte'
  import TrashCard from './lib/TrashCard.svelte'
  import LabelsView from './lib/LabelsView.svelte'
  import Login from './lib/Login.svelte'

  let view: 'main' | 'archive' | 'trash' | 'labels' = 'main'

  onMount(checkAuth)

  $: if ($isAuthed) {
    load()
    loadLabels()
  }
</script>

{#if $isAuthed === null}
  <p class="checking">Checking session...</p>
{:else if !$isAuthed}
  <Login />
{:else}
  <main>
    <div class="header">
      <h1>Retainer</h1>
      <div class="header-right">
        <span class="status" class:live={$wsConnected}>{$wsConnected ? '● Live' : '○ Reconnecting...'}</span>
        <button on:click={logout}>Log out</button>
      </div>
    </div>

    <nav class="tabs">
      <button class:active={view === 'main'} on:click={() => (view = 'main')}>Notes</button>
      <button class:active={view === 'archive'} on:click={() => (view = 'archive')}>Archive ({$archivedNotes.length})</button>
      <button class:active={view === 'trash'} on:click={() => (view = 'trash')}>Trash ({$trashedNotes.length})</button>
      <button class:active={view === 'labels'} on:click={() => (view = 'labels')}>Labels</button>
    </nav>

    {#if $loadError}
      <p class="error">Failed to load notes: {$loadError}</p>
    {/if}

    {#if view === 'main'}
      <div class="search-row">
        <input class="search-input" placeholder="Search notes..." bind:value={$searchQuery} />
        {#if $searchQuery}
          <button on:click={() => ($searchQuery = '')}>Clear</button>
        {/if}
      </div>

      {#if $searchResults !== null}
        <p class="section-label">{$searchResults.length} result{$searchResults.length === 1 ? '' : 's'}</p>
        <div class="grid">
          {#each $searchResults as note (note.id)}
            <div class="card-slot"><NoteCard {note} view="main" /></div>
          {/each}
        </div>
      {:else}
        <div class="composer">
          <button on:click={() => addTextNote('', '')}>Add note</button>
          <button on:click={() => addChecklist('')}>Add checklist</button>
        </div>

        {#if $pinnedNotes.length}
          <h2 class="section-label">Pinned</h2>
          <NoteGrid notesList={$pinnedNotes} view="main" onReorder={reorderNote} />
          <h2 class="section-label">Others</h2>
        {/if}
        <NoteGrid notesList={$unpinnedNotes} view="main" onReorder={reorderNote} />
      {/if}
    {:else if view === 'archive'}
      <NoteGrid notesList={$archivedNotes} view="archive" onReorder={reorderArchivedNote} />
    {:else if view === 'trash'}
      <div class="grid">
        {#each $trashedNotes as note (note.id)}
          <TrashCard {note} />
        {/each}
      </div>
    {:else}
      <LabelsView />
    {/if}
  </main>
{/if}

<style>
  main {
    max-width: 960px;
    margin: 0 auto;
    padding: 1rem;
  }
  .error {
    color: var(--danger);
  }
  .checking {
    text-align: center;
    margin-top: 4rem;
    opacity: 0.6;
  }
  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .header-right {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }
  .status {
    font-size: 0.85rem;
    opacity: 0.6;
  }
  .status.live {
    color: var(--success);
    opacity: 1;
  }
  .tabs {
    display: flex;
    gap: 0.5rem;
    margin: 1rem 0;
    border-bottom: 1px solid var(--border);
    padding-bottom: 0.5rem;
  }
  .tabs button {
    border: none;
    background: transparent;
    padding: 0.3rem 0.6rem;
    cursor: pointer;
    opacity: 0.6;
    color: var(--text);
  }
  .tabs button.active {
    opacity: 1;
    font-weight: 600;
    border-bottom: 2px solid var(--text-h);
    color: var(--text-h);
  }
  .section-label {
    font-size: 0.85rem;
    text-transform: uppercase;
    opacity: 0.6;
    margin: 1rem 0 0.5rem;
  }
  .composer {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 1.5rem;
    flex-wrap: wrap;
  }
  .composer button {
    cursor: pointer;
    padding: 0.4rem 0.8rem;
  }
  .search-row {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 1rem;
  }
  .search-input {
    flex: 1;
    padding: 0.4rem;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--card-bg);
    color: var(--text);
  }
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
</style>
