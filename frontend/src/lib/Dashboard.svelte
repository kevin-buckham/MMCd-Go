<script>
  export let latestValues = {}
  export let latestFloats = {}
  export let sensorDefs = []
  export let monitoring = false

  $: activeSensors = sensorDefs.filter(d => d.exists && !d.computed)
  $: computedSensors = sensorDefs.filter(d => d.exists && d.computed)
  $: allSensors = [...activeSensors, ...computedSensors]

  function getValueClass(slug, value) {
    if (slug === 'KNCK' && value > 0) return 'highlight'
    if (slug === 'COOL' && value > 100) return 'highlight'
    return ''
  }
</script>

<div class="card">
  <h2>Live Sensor Data</h2>
  {#if !monitoring}
    <p style="color: var(--text-muted); font-size: 13px;">
      Start monitoring to see live data. Connect to the ECU and press Monitor.
    </p>
  {/if}
</div>

<div class="sensor-grid">
  {#each allSensors as sensor}
    {@const val = latestValues[sensor.slug] || '—'}
    {@const fval = latestFloats[sensor.slug]}
    <div class="sensor-tile {getValueClass(sensor.slug, fval)}">
      <span class="slug">{sensor.slug}</span>
      <span class="value">{val}</span>
      <span class="desc">{sensor.description}</span>
    </div>
  {/each}
</div>
