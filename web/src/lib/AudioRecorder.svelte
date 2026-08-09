<script lang="ts">
  import { addAudioNote } from './stores/notes'

  // record-then-create (docs/adr/0005): nothing is persisted until the user
  // stops and saves — canceling mid-recording just discards the local blob,
  // no server-side cleanup needed.
  const MAX_DURATION_MS = 5 * 60 * 1000

  type State = 'idle' | 'requesting' | 'recording' | 'saving' | 'denied' | 'error'
  let state: State = 'idle'
  let errorMessage = ''
  let elapsedMs = 0

  let mediaRecorder: MediaRecorder | null = null
  let stream: MediaStream | null = null
  let chunks: Blob[] = []
  let startedAt = 0
  let tickTimer: ReturnType<typeof setInterval> | null = null
  let autoStopTimer: ReturnType<typeof setTimeout> | null = null

  function pickMimeType(): string {
    const candidates = ['audio/webm;codecs=opus', 'audio/webm', 'audio/mp4']
    for (const type of candidates) {
      if (typeof MediaRecorder !== 'undefined' && MediaRecorder.isTypeSupported?.(type)) return type
    }
    return '' // let the browser pick its own default
  }

  function stopTimers() {
    if (tickTimer) clearInterval(tickTimer)
    if (autoStopTimer) clearTimeout(autoStopTimer)
    tickTimer = null
    autoStopTimer = null
  }

  function teardownStream() {
    stream?.getTracks().forEach((t) => t.stop())
    stream = null
    mediaRecorder = null
  }

  async function start() {
    errorMessage = ''
    state = 'requesting'
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    } catch (e) {
      state = e instanceof DOMException && e.name === 'NotAllowedError' ? 'denied' : 'error'
      errorMessage = state === 'denied' ? 'Microphone access denied — enable it in your browser settings.' : 'Could not access the microphone.'
      return
    }

    chunks = []
    const mimeType = pickMimeType()
    mediaRecorder = new MediaRecorder(stream, mimeType ? { mimeType } : undefined)
    mediaRecorder.ondataavailable = (e) => {
      if (e.data.size > 0) chunks.push(e.data)
    }
    mediaRecorder.start()
    startedAt = Date.now()
    elapsedMs = 0
    state = 'recording'

    tickTimer = setInterval(() => {
      elapsedMs = Date.now() - startedAt
    }, 200)
    autoStopTimer = setTimeout(stopAndSave, MAX_DURATION_MS)
  }

  function cancel() {
    stopTimers()
    if (mediaRecorder && mediaRecorder.state !== 'inactive') mediaRecorder.stop()
    teardownStream()
    state = 'idle'
  }

  async function stopAndSave() {
    if (!mediaRecorder || mediaRecorder.state === 'inactive') return
    stopTimers()
    const finalDurationMs = Math.min(Date.now() - startedAt, MAX_DURATION_MS)
    const recorder = mediaRecorder

    const blob: Blob = await new Promise((resolve) => {
      recorder.onstop = () => resolve(new Blob(chunks, { type: recorder.mimeType || 'audio/webm' }))
      recorder.stop()
    })
    teardownStream()
    state = 'saving'

    try {
      await addAudioNote(blob, finalDurationMs)
      state = 'idle'
    } catch (e) {
      state = 'error'
      errorMessage = e instanceof Error ? e.message : 'Failed to save the recording.'
    }
  }

  function formatElapsed(ms: number): string {
    const totalSeconds = Math.floor(ms / 1000)
    const m = Math.floor(totalSeconds / 60)
    const s = totalSeconds % 60
    return `${m}:${s.toString().padStart(2, '0')}`
  }

  function dismissError() {
    state = 'idle'
    errorMessage = ''
  }
</script>

{#if state === 'idle'}
  <button on:click={start}>🎙 Add audio</button>
{:else if state === 'requesting'}
  <button disabled>Requesting microphone…</button>
{:else if state === 'recording'}
  <div class="recording">
    <span class="dot" aria-hidden="true"></span>
    <span class="elapsed">{formatElapsed(elapsedMs)} / 5:00</span>
    <button on:click={stopAndSave}>Stop &amp; save</button>
    <button on:click={cancel}>Cancel</button>
  </div>
{:else if state === 'saving'}
  <button disabled>Saving…</button>
{:else if state === 'denied' || state === 'error'}
  <div class="error-row">
    <span class="error-text">{errorMessage}</span>
    <button on:click={dismissError}>Dismiss</button>
  </div>
{/if}

<style>
  .recording,
  .error-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .dot {
    width: 0.5rem;
    height: 0.5rem;
    border-radius: 50%;
    background: var(--danger);
    flex-shrink: 0;
  }
  .elapsed {
    font-variant-numeric: tabular-nums;
    opacity: 0.8;
  }
  .error-text {
    color: var(--danger);
    font-size: 0.9rem;
  }
</style>
