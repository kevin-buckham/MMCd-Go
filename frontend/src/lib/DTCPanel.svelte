<script>
  export let connected = false

  let activeDTCs = []
  let storedDTCs = []
  let activeRaw = 0
  let storedRaw = 0
  let loading = false
  let error = ''

  const wails = window.go?.main?.App

  async function readDTCs() {
    if (!connected) return
    loading = true
    error = ''
    try {
      const result = await wails?.ReadDTCs()
      if (result) {
        activeDTCs = result.active || []
        storedDTCs = result.stored || []
        activeRaw = result.activeRaw || 0
        storedRaw = result.storedRaw || 0
      }
    } catch (e) {
      error = 'Failed to read DTCs: ' + e
    }
    loading = false
  }

  async function eraseDTCs() {
    if (!connected) return
    if (!confirm('Are you sure you want to erase all stored DTCs?')) return
    loading = true
    error = ''
    try {
      await wails?.EraseDTCs()
      // Re-read after erase
      await readDTCs()
    } catch (e) {
      error = 'Failed to erase DTCs: ' + e
    }
    loading = false
  }
</script>

<div class="card">
  <h2>Diagnostic Trouble Codes</h2>
  {#if !connected}
    <p style="color: var(--text-muted); font-size: 13px;">
      Connect to the ECU to read diagnostic trouble codes.
    </p>
  {:else}
    <div style="display: flex; gap: 8px; margin-bottom: 16px;">
      <button class="btn btn-primary" on:click={readDTCs} disabled={loading}>
        {loading ? 'Reading...' : 'Read DTCs'}
      </button>
      <button class="btn btn-danger" on:click={eraseDTCs} disabled={loading}>
        Erase DTCs
      </button>
    </div>

    {#if error}
      <p style="color: var(--accent); font-size: 13px; margin-bottom: 12px;">{error}</p>
    {/if}
  {/if}
</div>

{#if connected}
  <div class="card">
    <h2>Active Faults <span style="font-size: 11px; color: var(--text-muted);">(raw: 0x{activeRaw.toString(16).toUpperCase().padStart(4, '0')})</span></h2>
    {#if activeDTCs.length === 0}
      <p style="color: var(--accent-green); font-size: 13px;">No active faults</p>
    {:else}
      <table class="dtc-table">
        <thead>
          <tr>
            <th>Code</th>
            <th>Description</th>
            <th>Bit</th>
          </tr>
        </thead>
        <tbody>
          {#each activeDTCs as dtc}
            <tr>
              <td class="fault">{dtc.code}</td>
              <td>{dtc.description}</td>
              <td style="color: var(--text-muted);">{dtc.bit}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>

  <div class="card">
    <h2>Stored Faults <span style="font-size: 11px; color: var(--text-muted);">(raw: 0x{storedRaw.toString(16).toUpperCase().padStart(4, '0')})</span></h2>
    {#if storedDTCs.length === 0}
      <p style="color: var(--accent-green); font-size: 13px;">No stored faults</p>
    {:else}
      <table class="dtc-table">
        <thead>
          <tr>
            <th>Code</th>
            <th>Description</th>
            <th>Bit</th>
          </tr>
        </thead>
        <tbody>
          {#each storedDTCs as dtc}
            <tr>
              <td class="fault">{dtc.code}</td>
              <td>{dtc.description}</td>
              <td style="color: var(--text-muted);">{dtc.bit}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
{/if}
