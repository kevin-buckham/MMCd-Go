<script>
  import Dashboard from './lib/Dashboard.svelte'
  import Graph from './lib/Graph.svelte'
  import DTCPanel from './lib/DTCPanel.svelte'
  import TestPanel from './lib/TestPanel.svelte'
  import Settings from './lib/Settings.svelte'
  import About from './lib/About.svelte'
  import Log from './lib/Log.svelte'

  let currentView = 'dashboard'
  let sensorDefs = []

  // Data source: 'none' | 'live' | 'demo' | 'file'
  let dataSource = 'none'
  let connected = false
  let monitoring = false
  let logging = false
  let selectedPort = ''
  let baudRate = 1953
  let ports = []
  let loadedFileName = ''
  let actionLoading = false
  let disconnectReason = ''
  let commStats = { samplesTotal: 0, errorsTotal: 0, currentHz: 0, uptimeSeconds: 0 }

  // Shared state — available to ALL views at ALL times
  let sampleCount = 0
  let latestValues = {}
  let latestFloats = {}

  // Shared history buffer — Graph and Dashboard both read from this.
  // Accumulated regardless of which view is active.
  const maxHistory = 30000
  let history = {}          // slug -> float[]
  let historyTimes = []     // elapsed ms from start, parallel to history arrays
  let historyVersion = 0    // bump to trigger Graph reactivity
  let historyStartMs = 0    // Date.now() when first sample arrived

  // Wails runtime bindings
  const wails = window.go?.main?.App

  async function refreshPorts() {
    try {
      ports = await wails?.ListSerialPorts() || []
      if (ports.length > 0 && !selectedPort) selectedPort = ports[0]
    } catch (e) {
      console.error('Failed to list ports:', e)
    }
  }

  async function loadSensorDefs() {
    try {
      sensorDefs = await wails?.GetSensorDefinitions() || []
    } catch (e) {
      console.error('Failed to load sensor defs:', e)
    }
  }

  // --- Data source actions ---

  function clearHistory() {
    history = {}
    historyTimes = []
    historyVersion = 0
    historyStartMs = 0
    sampleCount = 0
    latestValues = {}
    latestFloats = {}
  }

  async function selectLive() {
    if (actionLoading) return
    actionLoading = true
    disconnectReason = ''
    if (dataSource !== 'none') await stopDataSource()
    try {
      await wails?.Connect(selectedPort, baudRate)
      dataSource = 'live'
      connected = true
      clearHistory()
    } catch (e) {
      alert('Connection failed: ' + e)
    }
    actionLoading = false
  }

  async function selectDemo() {
    if (actionLoading) return
    actionLoading = true
    disconnectReason = ''
    if (dataSource !== 'none') await stopDataSource()
    try {
      await wails?.ConnectDemo()
      dataSource = 'demo'
      connected = true
      clearHistory()
    } catch (e) {
      alert('Demo mode failed: ' + e)
    }
    actionLoading = false
  }

  async function selectFile() {
    try {
      const result = await wails?.LoadLogFile()
      if (result && result.data) {
        if (dataSource !== 'none') await stopDataSource()
        dataSource = 'file'
        connected = false
        monitoring = false
        loadedFileName = result.name || 'Log file'

        // Populate shared history from loaded file
        history = {}
        for (const slug of Object.keys(result.data)) {
          history[slug] = [...result.data[slug]]
        }
        historyTimes = result.elapsedMs ? [...result.elapsedMs] : []
        sampleCount = result.count || 0
        historyVersion++

        // Update latestValues/Floats to the last sample
        latestFloats = {}
        latestValues = {}
        for (const slug of Object.keys(result.data)) {
          const arr = result.data[slug]
          if (arr && arr.length > 0) {
            latestFloats[slug] = arr[arr.length - 1]
            latestValues[slug] = arr[arr.length - 1].toFixed(1)
          }
        }

        currentView = 'graph'
      }
    } catch (e) {
      if (e !== 'cancelled') alert('Failed to load log: ' + e)
    }
  }

  async function stopDataSource() {
    if (actionLoading) return
    actionLoading = true
    if (monitoring) {
      try { await wails?.StopMonitoring() } catch (_) {}
      monitoring = false
    }
    if (logging) {
      try { await wails?.StopLogging() } catch (_) {}
      logging = false
    }
    if (connected) {
      try { await wails?.Disconnect() } catch (_) {}
      connected = false
    }
    dataSource = 'none'
    loadedFileName = ''
    actionLoading = false
  }

  async function toggleMonitoring() {
    if (actionLoading) return
    actionLoading = true
    if (!monitoring) {
      try {
        await wails?.StartMonitoring()
        monitoring = true
      } catch (e) {
        alert('Failed to start monitoring: ' + e)
      }
    } else {
      try {
        await wails?.StopMonitoring()
        monitoring = false
      } catch (e) {
        console.error('Stop monitoring error:', e)
      }
    }
    actionLoading = false
  }

  async function toggleLogging() {
    if (actionLoading) return
    actionLoading = true
    if (!logging) {
      const filename = `mmcd-log-${new Date().toISOString().replace(/[:.]/g, '-')}.csv`
      try {
        await wails?.StartLogging(filename)
        logging = true
      } catch (e) {
        alert('Failed to start logging: ' + e)
      }
    } else {
      try {
        await wails?.StopLogging()
        logging = false
      } catch (e) {
        console.error('Stop logging error:', e)
      }
    }
    actionLoading = false
  }

  // Record every sample into shared history — runs regardless of active view
  function recordSample(floats) {
    const now = Date.now()
    if (historyStartMs === 0) historyStartMs = now
    historyTimes.push(now - historyStartMs)
    if (historyTimes.length > maxHistory) {
      historyTimes = historyTimes.slice(historyTimes.length - maxHistory)
    }
    for (const slug of Object.keys(floats)) {
      if (!history[slug]) history[slug] = []
      history[slug].push(floats[slug])
      if (history[slug].length > maxHistory) {
        history[slug] = history[slug].slice(history[slug].length - maxHistory)
      }
    }
    historyVersion++
  }

  // Listen for sensor data events from Go backend
  if (window.runtime) {
    window.runtime.EventsOn('sensor:sample', (data) => {
      latestValues = data.values || {}
      latestFloats = data.floats || {}
      sampleCount++
      recordSample(data.floats || {})
    })

    window.runtime.EventsOn('connection:status', (data) => {
      connected = data.connected
      if (!data.connected) {
        monitoring = false
        if (data.reason) {
          disconnectReason = data.reason
        }
        if (dataSource === 'live' || dataSource === 'demo') dataSource = 'none'
      }
    })

    window.runtime.EventsOn('logging:status', (data) => {
      logging = data.logging
    })

    window.runtime.EventsOn('comm:stats', (data) => {
      commStats = data
    })
  }

  // Initialize
  refreshPorts()
  loadSensorDefs()

  const views = [
    { id: 'dashboard', label: 'Dashboard', icon: '◉' },
    { id: 'graph', label: 'Graph', icon: '◈' },
    { id: 'dtc', label: 'DTCs', icon: '⚠' },
    { id: 'test', label: 'Test', icon: '⚡' },
    { id: 'log', label: 'Log', icon: '☰' },
    { id: 'settings', label: 'Settings', icon: '⚙' },
    { id: 'about', label: 'About', icon: 'ⓘ' },
  ]

  $: sourceLabel = dataSource === 'live' ? `Live: ${selectedPort}`
                  : dataSource === 'demo' ? 'Demo Simulator'
                  : dataSource === 'file' ? `File: ${loadedFileName.split('/').pop()}`
                  : 'No data source'
</script>

<div class="app-header">
  <h1>MMCD DATALOGGER</h1>
  <div class="connection-bar">
    {#if dataSource === 'none'}
      <div class="source-buttons">
        <select bind:value={selectedPort}>
          <option value="">Select port...</option>
          {#each ports as port}
            <option value={port}>{port}</option>
          {/each}
        </select>
        <button class="btn btn-sm" on:click={refreshPorts}>↻</button>
        <input type="number" bind:value={baudRate} placeholder="Baud" style="width: 70px;" />
        {#if baudRate !== 1953}
          <span style="color: var(--accent-yellow); font-size: 10px;">Non-standard baud</span>
        {/if}
        <button class="btn btn-primary btn-sm" on:click={selectLive} disabled={!selectedPort || actionLoading}>
          Live ECU
        </button>
        <button class="btn btn-sm" style="background: var(--accent-yellow); color: #000; border-color: var(--accent-yellow);" on:click={selectDemo} disabled={actionLoading}>
          Demo
        </button>
        <button class="btn btn-sm" on:click={selectFile} disabled={actionLoading}>
          Load File
        </button>
      </div>
      {#if disconnectReason}
        <span style="color: var(--accent); font-size: 11px;">{disconnectReason}</span>
      {/if}
    {:else}
      <span class="source-label" class:demo={dataSource === 'demo'} class:live={dataSource === 'live'} class:file={dataSource === 'file'}>
        {sourceLabel}
      </span>
      <span style="font-size: 11px; color: var(--text-muted); font-family: var(--font-mono);">{sampleCount} samples</span>
      <button class="btn btn-danger btn-sm" on:click={stopDataSource} disabled={actionLoading}>
        ✕ Stop
      </button>
    {/if}
    <span class="status-dot" class:connected={dataSource !== 'none'} class:disconnected={dataSource === 'none'}></span>
  </div>
</div>

<div class="app-body">
  <div class="sidebar">
    <div class="nav-section">
      <h3>Views</h3>
      {#each views as view}
        <button
          class="nav-item"
          class:active={currentView === view.id}
          on:click={() => currentView = view.id}
        >
          <span>{view.icon}</span>
          {view.label}
          {#if view.id === 'log' && commStats.errorsTotal > 0}
            <span style="margin-left: auto; background: var(--accent); color: white; font-size: 10px; padding: 1px 5px; border-radius: 8px; font-family: var(--font-mono);">{commStats.errorsTotal}</span>
          {/if}
        </button>
      {/each}
    </div>

    <div class="nav-section">
      <h3>Controls</h3>
      {#if dataSource === 'live' || dataSource === 'demo'}
        <button class="nav-item" on:click={toggleMonitoring} disabled={actionLoading}>
          {monitoring ? '⏸ Pause' : '▶ Monitor'}
        </button>
        <button class="nav-item" on:click={toggleLogging} disabled={actionLoading}>
          {logging ? '⏹ Stop Log' : '⏺ Record'}
        </button>
      {:else if dataSource === 'file'}
        <div class="nav-item" style="cursor: default; color: var(--text-muted); font-size: 12px;">
          Reviewing log file
        </div>
      {:else}
        <div class="nav-item" style="color: var(--text-muted)">
          Select a data source
        </div>
      {/if}
    </div>

    <div class="nav-section">
      <h3>Status</h3>
      <div class="nav-item" style="cursor: default; font-family: var(--font-mono); font-size: 11px;">
        {#if dataSource === 'live'}
          <span style="color: var(--accent-green);">● LIVE</span>
          {#if commStats.currentHz > 0}
            <span style="color: var(--text-muted); margin-left: 4px;">{commStats.currentHz.toFixed(1)} Hz</span>
          {/if}
        {:else if dataSource === 'demo'}
          <span style="color: var(--accent-yellow);">● DEMO</span>
          {#if commStats.currentHz > 0}
            <span style="color: var(--text-muted); margin-left: 4px;">{commStats.currentHz.toFixed(1)} Hz</span>
          {/if}
        {:else if dataSource === 'file'}
          <span style="color: var(--accent-blue, #60a5fa);">● FILE</span>
        {:else}
          <span style="color: var(--text-muted);">○ IDLE</span>
        {/if}
      </div>
    </div>
  </div>

  <div class="main-content">
    {#if currentView === 'dashboard'}
      <Dashboard {latestValues} {latestFloats} {sensorDefs} monitoring={monitoring || dataSource === 'file'} />
    {:else if currentView === 'graph'}
      <Graph {sensorDefs} {history} {historyTimes} {historyVersion} isFileMode={dataSource === 'file'} />
    {:else if currentView === 'dtc'}
      <DTCPanel connected={dataSource === 'live' || dataSource === 'demo'} {monitoring} demoMode={dataSource === 'demo'} />
    {:else if currentView === 'test'}
      <TestPanel connected={dataSource === 'live' || dataSource === 'demo'} {monitoring} demoMode={dataSource === 'demo'} />
    {:else if currentView === 'log'}
      <Log />
    {:else if currentView === 'settings'}
      <Settings {sensorDefs} connected={dataSource === 'live' || dataSource === 'demo'} />
    {:else if currentView === 'about'}
      <About />
    {/if}
  </div>
</div>
