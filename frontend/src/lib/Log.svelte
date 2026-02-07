<script>
  let entries = []
  let stats = { samplesTotal: 0, errorsTotal: 0, currentHz: 0, uptimeSeconds: 0 }
  let autoScroll = true
  let logContainer

  const wails = window.go?.main?.App
  const maxEntries = 500

  // Fetch existing entries on mount
  async function loadEntries() {
    try {
      const existing = await wails?.GetCommLog()
      if (existing && existing.length > 0) {
        entries = existing
      }
    } catch (e) {
      console.error('Failed to load comm log:', e)
    }
  }

  loadEntries()

  // Listen for new log entries
  if (window.runtime) {
    window.runtime.EventsOn('comm:log', (entry) => {
      entries = [...entries, entry]
      if (entries.length > maxEntries) {
        entries = entries.slice(entries.length - maxEntries)
      }
      if (autoScroll && logContainer) {
        requestAnimationFrame(() => {
          logContainer.scrollTop = logContainer.scrollHeight
        })
      }
    })

    window.runtime.EventsOn('comm:stats', (data) => {
      stats = data
    })
  }

  function clearLog() {
    entries = []
  }

  function handleScroll() {
    if (!logContainer) return
    const atBottom = logContainer.scrollHeight - logContainer.scrollTop - logContainer.clientHeight < 30
    autoScroll = atBottom
  }

  function formatUptime(seconds) {
    if (!seconds || seconds <= 0) return '0s'
    const m = Math.floor(seconds / 60)
    const s = Math.floor(seconds % 60)
    if (m > 0) return `${m}m ${s}s`
    return `${s}s`
  }

  function levelColor(level) {
    switch (level) {
      case 'error': return 'var(--accent)'
      case 'warn': return 'var(--accent-yellow)'
      default: return 'var(--accent-green)'
    }
  }
</script>

<div class="card">
  <h2>Communication Log</h2>
  <div class="stats-bar">
    <span class="stat">{stats.currentHz.toFixed(1)} Hz</span>
    <span class="stat">{stats.samplesTotal.toLocaleString()} samples</span>
    <span class="stat" class:has-errors={stats.errorsTotal > 0}>
      {stats.errorsTotal} errors
    </span>
    <span class="stat">uptime {formatUptime(stats.uptimeSeconds)}</span>
    <button class="btn btn-sm" on:click={clearLog} style="margin-left: auto;">Clear</button>
  </div>
</div>

<div class="card log-card">
  <div class="log-container" bind:this={logContainer} on:scroll={handleScroll}>
    {#if entries.length === 0}
      <p style="color: var(--text-muted); font-size: 13px; padding: 8px;">
        No log entries yet. Connect to the ECU to see communication events.
      </p>
    {:else}
      {#each entries as entry}
        <div class="log-entry">
          <span class="log-time">{entry.time}</span>
          <span class="log-level" style="color: {levelColor(entry.level)};">{entry.level.toUpperCase()}</span>
          <span class="log-msg">{entry.message}</span>
          {#if entry.detail}
            <span class="log-detail">{entry.detail}</span>
          {/if}
        </div>
      {/each}
    {/if}
  </div>
  {#if !autoScroll}
    <button class="scroll-btn" on:click={() => { autoScroll = true; if (logContainer) logContainer.scrollTop = logContainer.scrollHeight }}>
      Scroll to bottom
    </button>
  {/if}
</div>

<style>
  .stats-bar {
    display: flex;
    align-items: center;
    gap: 16px;
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .stat {
    color: var(--text-secondary);
  }

  .stat.has-errors {
    color: var(--accent);
    font-weight: 600;
  }

  .log-card {
    position: relative;
    padding: 0;
  }

  .log-container {
    height: 400px;
    overflow-y: auto;
    padding: 8px;
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 1.6;
  }

  .log-entry {
    display: flex;
    gap: 8px;
    padding: 2px 4px;
    border-bottom: 1px solid rgba(255,255,255,0.03);
  }

  .log-entry:hover {
    background: rgba(255,255,255,0.03);
  }

  .log-time {
    color: var(--text-muted);
    flex-shrink: 0;
    width: 90px;
  }

  .log-level {
    flex-shrink: 0;
    width: 45px;
    font-weight: 600;
    font-size: 10px;
    text-transform: uppercase;
  }

  .log-msg {
    color: var(--text-primary);
    flex-shrink: 0;
  }

  .log-detail {
    color: var(--text-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .scroll-btn {
    position: absolute;
    bottom: 8px;
    right: 8px;
    padding: 4px 10px;
    font-size: 11px;
    border-radius: 4px;
    border: 1px solid var(--border);
    background: var(--bg-card);
    color: var(--text-secondary);
    cursor: pointer;
  }

  .scroll-btn:hover {
    background: rgba(255,255,255,0.1);
  }
</style>
