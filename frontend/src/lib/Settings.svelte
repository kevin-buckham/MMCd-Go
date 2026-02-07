<script>
  export let sensorDefs = []
  export let connected = false

  let units = 'metric'
  let selectedSensors = ['RPM', 'TPS', 'COOL', 'TIMA', 'KNCK', 'INJP', 'O2-R', 'BATT']

  const wails = window.go?.main?.App

  $: activeDefs = sensorDefs.filter(d => d.exists)

  function toggleSensor(slug) {
    if (selectedSensors.includes(slug)) {
      selectedSensors = selectedSensors.filter(s => s !== slug)
    } else {
      selectedSensors = [...selectedSensors, slug]
    }
    applySelection()
  }

  function selectAll() {
    selectedSensors = activeDefs.filter(d => !d.computed).map(d => d.slug)
    applySelection()
  }

  function selectCommon() {
    selectedSensors = ['RPM', 'TPS', 'COOL', 'TIMA', 'KNCK', 'INJP', 'O2-R', 'BATT']
    applySelection()
  }

  function selectNone() {
    selectedSensors = []
    applySelection()
  }

  async function applySelection() {
    try {
      await wails?.SetActiveSensors(selectedSensors)
    } catch (e) {
      console.error('Failed to set sensors:', e)
    }
  }

  async function changeUnits() {
    try {
      await wails?.SetUnits(units)
    } catch (e) {
      console.error('Failed to set units:', e)
    }
  }
</script>

<div class="card">
  <h2>Unit System</h2>
  <div style="display: flex; gap: 8px;">
    <label class="toggle">
      <input type="radio" bind:group={units} value="metric" on:change={changeUnits} />
      Metric (°C, bar)
    </label>
    <label class="toggle">
      <input type="radio" bind:group={units} value="imperial" on:change={changeUnits} />
      Imperial (°F, psi)
    </label>
    <label class="toggle">
      <input type="radio" bind:group={units} value="raw" on:change={changeUnits} />
      Raw (0-255)
    </label>
  </div>
</div>

<div class="card">
  <h2>Sensor Selection</h2>
  <p style="color: var(--text-muted); font-size: 12px; margin-bottom: 12px;">
    Fewer sensors = higher sample rate. Select only what you need for best performance.
  </p>
  <div style="display: flex; gap: 8px; margin-bottom: 12px;">
    <button class="btn btn-sm" on:click={selectCommon}>Common</button>
    <button class="btn btn-sm" on:click={selectAll}>All</button>
    <button class="btn btn-sm" on:click={selectNone}>None</button>
  </div>

  <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 4px;">
    {#each activeDefs as sensor}
      <label class="toggle" style="padding: 4px 0;">
        <input
          type="checkbox"
          checked={selectedSensors.includes(sensor.slug)}
          on:change={() => toggleSensor(sensor.slug)}
          disabled={sensor.computed}
        />
        <span style="font-family: var(--font-mono); font-size: 12px; width: 40px;">{sensor.slug}</span>
        <span style="font-size: 12px; color: var(--text-muted);">{sensor.description}</span>
        {#if sensor.computed}
          <span style="font-size: 10px; color: var(--accent-yellow);">(auto)</span>
        {/if}
      </label>
    {/each}
  </div>
</div>

<div class="card">
  <h2>Protocol Info</h2>
  <div style="font-size: 13px; color: var(--text-secondary); line-height: 1.6;">
    <p><strong>Protocol:</strong> Mitsubishi ALDL (OBDI) — request/reply, 1 byte each</p>
    <p><strong>Default baud:</strong> 1953 bps, 8N1, no flow control</p>
    <p><strong>Vehicles:</strong> 1990-1994 Mitsubishi Eclipse, Eagle Talon, Plymouth Laser, Galant (4G63)</p>
    <p><strong>Hardware:</strong> ALDL diagnostic connector with USB-TTL adapter (FTDI/CH340) and signal diode</p>
  </div>
</div>
