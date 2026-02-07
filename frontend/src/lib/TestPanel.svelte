<script>
  export let connected = false
  export let monitoring = false
  export let demoMode = false

  let loading = false
  let result = ''
  let error = ''
  let countdown = 0
  let countdownInterval = null
  let activeCommand = null

  const wails = window.go?.main?.App

  const solenoidCommands = [
    { id: 'fuel-pump', label: 'Fuel Pump', desc: 'Activate fuel pump relay' },
    { id: 'purge', label: 'Purge', desc: 'Canister purge solenoid' },
    { id: 'pressure', label: 'Pressure', desc: 'Pressure solenoid' },
    { id: 'egr', label: 'EGR', desc: 'EGR solenoid' },
    { id: 'mvic', label: 'MVIC', desc: 'MVIC motor' },
    { id: 'boost', label: 'Boost', desc: 'Boost solenoid' },
  ]

  const injectorCommands = [
    { id: 'inj1', label: 'Inj #1', desc: 'Disable injector 1' },
    { id: 'inj2', label: 'Inj #2', desc: 'Disable injector 2' },
    { id: 'inj3', label: 'Inj #3', desc: 'Disable injector 3' },
    { id: 'inj4', label: 'Inj #4', desc: 'Disable injector 4' },
    { id: 'inj5', label: 'Inj #5', desc: 'Disable injector 5' },
    { id: 'inj6', label: 'Inj #6', desc: 'Disable injector 6' },
  ]

  function isSolenoid(id) {
    return solenoidCommands.some(c => c.id === id)
  }

  function getLabel(id) {
    const all = [...solenoidCommands, ...injectorCommands]
    return all.find(c => c.id === id)?.label || id
  }

  async function runTest(command) {
    if (!connected) return

    if (!demoMode) {
      const label = getLabel(command)
      const msg = isSolenoid(command)
        ? `Activate ${label}? This physically controls the component for ~6 seconds. Engine must be OFF.`
        : `Disable ${label}? Only use with engine running at idle.`

      if (!confirm(msg)) return
    }

    loading = true
    activeCommand = command
    result = ''
    error = ''
    countdown = demoMode ? 6 : 10
    countdownInterval = setInterval(() => {
      countdown--
      if (countdown <= 0) clearInterval(countdownInterval)
    }, 1000)

    try {
      result = await wails?.RunActuatorTest(command)
    } catch (e) {
      error = 'Test failed: ' + e
    }
    loading = false
    activeCommand = null
    countdown = 0
    if (countdownInterval) clearInterval(countdownInterval)
  }
</script>

<div class="card">
  <h2>Actuator Tests {#if demoMode}<span style="font-size: 11px; color: var(--accent-yellow);">(DEMO)</span>{/if}</h2>
  {#if !connected}
    <p style="color: var(--text-muted); font-size: 13px;">
      Connect to the ECU to run actuator tests.
    </p>
  {:else if monitoring}
    <p style="color: var(--accent-yellow); font-size: 13px;">
      ⚠ Pause monitoring before running actuator tests.
    </p>
  {:else}
    <p style="color: var(--accent-yellow); font-size: 12px; margin-bottom: 12px;">
      ⚠ Solenoid/relay tests require engine OFF. Injector disable works with engine running.
      ECU activates the component for ~6 seconds.
    </p>

    {#if loading && countdown > 0}
      <p style="color: var(--accent-blue, #60a5fa); font-size: 13px; margin-bottom: 12px; font-family: var(--font-mono);">
        Test in progress... {countdown}s remaining
      </p>
    {/if}
    {#if result}
      <p style="color: var(--accent-green); font-size: 13px; margin-bottom: 12px;">Result: {result}</p>
    {/if}
    {#if error}
      <p style="color: var(--accent); font-size: 13px; margin-bottom: 12px;">{error}</p>
    {/if}
  {/if}
</div>

{#if connected && !monitoring}
  <div class="card">
    <h2>Solenoids / Relays (Engine OFF)</h2>
    <div style="display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px;">
      {#each solenoidCommands as cmd}
        <button class="btn" class:btn-active={activeCommand === cmd.id} on:click={() => runTest(cmd.id)} disabled={loading}>
          <div>
            <div style="font-weight: 600;">{cmd.label}</div>
            <div style="font-size: 11px; color: var(--text-muted);">{cmd.desc}</div>
          </div>
        </button>
      {/each}
    </div>
  </div>

  <div class="card">
    <h2>Injector Disable (Engine Running)</h2>
    <div style="display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px;">
      {#each injectorCommands as cmd}
        <button class="btn" class:btn-active={activeCommand === cmd.id} on:click={() => runTest(cmd.id)} disabled={loading}>
          <div>
            <div style="font-weight: 600;">{cmd.label}</div>
            <div style="font-size: 11px; color: var(--text-muted);">{cmd.desc}</div>
          </div>
        </button>
      {/each}
    </div>
  </div>
{/if}

