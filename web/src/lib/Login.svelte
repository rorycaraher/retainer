<script lang="ts">
  import { login } from './stores/auth'

  let password = ''
  let error = ''
  let loading = false

  async function submit() {
    error = ''
    loading = true
    try {
      await login(password)
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      loading = false
    }
  }
</script>

<main class="login">
  <h1>Retainer</h1>
  <form on:submit|preventDefault={submit}>
    <!-- svelte-ignore a11y_autofocus -->
    <input type="password" placeholder="Password" bind:value={password} autofocus />
    <button type="submit" disabled={loading}>Log in</button>
  </form>
  {#if error}
    <p class="error">{error}</p>
  {/if}
</main>

<style>
  .login {
    max-width: 320px;
    margin: 4rem auto;
    padding: 1rem;
    text-align: center;
  }
  form {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    margin-top: 1rem;
  }
  input,
  button {
    padding: 0.5rem;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--card-bg);
    color: var(--text);
  }
  button {
    cursor: pointer;
  }
  .error {
    color: var(--danger);
  }
</style>
