<script>
  import { onMount, onDestroy } from 'svelte'

  export let sensorDefs = []
  export let isFileMode = false
  export let history = {}           // shared history from App.svelte: slug -> float[]
  export let historyTimes = []      // elapsed ms per sample, parallel to history arrays
  export let historyVersion = 0     // bumped by parent when history changes

  let graphContainer
  let canvas
  let ctx
  let selectedSensors = ['RPM', 'TPS', 'COOL', 'O2-R']
  let sensorsAutoSelected = false

  const viewSize = 300       // samples visible at once
  const scrollBarH = 12      // scrollbar height in px

  // Viewport — which slice of history is visible
  let viewEnd = 0             // absolute index of rightmost visible sample
  let isLive = true           // auto-scroll to latest data

  // Crosshair state — pinned tracks absolute sample index
  let hoverX = null           // canvas X during hover (null when not hovering)
  let pinnedAbsIndex = null   // absolute history index of pinned sample (null = not pinned)
  let cursorValues = []       // current readout values

  // Scrollbar drag state
  let isDraggingScrollbar = false
  let dragStartX = 0
  let dragStartViewEnd = 0

  const colors = {
    'RPM': '#e94560', 'TPS': '#4ade80', 'COOL': '#60a5fa', 'O2-R': '#fbbf24',
    'TIMA': '#a78bfa', 'KNCK': '#f87171', 'INJP': '#34d399', 'BATT': '#fb923c',
    'MAFS': '#38bdf8', 'AIRT': '#c084fc', 'BARO': '#94a3b8', 'INJD': '#f472b6',
    'O2-F': '#fbbf24', 'FTRL': '#a3e635', 'FTRM': '#22d3ee', 'FTRH': '#f9a8d4',
    'FTO2': '#d4d4d8', 'ACLE': '#facc15', 'ISC': '#86efac', 'EGRT': '#fca5a5',
    'FLG0': '#9ca3af', 'FLG2': '#6b7280',
  }

  const ranges = {
    'RPM':  [0, 8000], 'TPS':  [0, 100], 'COOL': [-40, 120], 'O2-R': [0, 5],
    'O2-F': [0, 5], 'TIMA': [-10, 60], 'KNCK': [0, 255], 'INJP': [0, 65],
    'BATT': [0, 18], 'MAFS': [0, 1600], 'AIRT': [-40, 100], 'BARO': [0, 1.5],
    'INJD': [0, 100], 'FTRL': [0, 200], 'FTRM': [0, 200], 'FTRH': [0, 200],
    'FTO2': [0, 200], 'ACLE': [0, 100], 'ISC':  [0, 100], 'EGRT': [0, 400],
  }

  const unitLabels = {
    'RPM': 'rpm', 'TPS': '%', 'COOL': '°C', 'O2-R': 'V', 'O2-F': 'V',
    'TIMA': '°', 'KNCK': '', 'INJP': 'ms', 'BATT': 'V', 'MAFS': 'Hz',
    'AIRT': '°C', 'BARO': 'bar', 'INJD': '%', 'FTRL': '%', 'FTRM': '%',
    'FTRH': '%', 'FTO2': '%', 'ACLE': '%', 'ISC': '%', 'EGRT': '°C',
  }

  $: activeSlugs = sensorDefs.filter(d => d.exists).map(d => d.slug)

  // Get longest history length across selected sensors
  function getMaxLen() {
    let m = 0
    for (const slug of selectedSensors) {
      if (history[slug]) m = Math.max(m, history[slug].length)
    }
    return m
  }

  // Compute viewport bounds
  function getViewBounds() {
    const maxLen = getMaxLen()
    if (maxLen === 0) return { start: 0, end: 0, maxLen: 0 }
    let end = isLive ? maxLen : Math.min(viewEnd, maxLen)
    let start = Math.max(0, end - viewSize)
    end = Math.min(start + viewSize, maxLen)
    return { start, end, maxLen }
  }

  function toggleSensor(slug) {
    if (selectedSensors.includes(slug)) {
      selectedSensors = selectedSensors.filter(s => s !== slug)
    } else {
      selectedSensors = [...selectedSensors, slug]
    }
    drawGraph()
  }

  // Convert canvas X → absolute history index
  function xToAbsIndex(canvasX) {
    if (!canvas) return null
    const w = canvas.width
    const pad = { left: 10, right: 10 }
    const plotW = w - pad.left - pad.right
    const frac = (canvasX - pad.left) / plotW
    if (frac < 0 || frac > 1) return null
    const { start } = getViewBounds()
    return start + Math.round(frac * (viewSize - 1))
  }

  // Get value for a slug at an absolute index
  function getValueAt(slug, absIdx) {
    const data = history[slug]
    if (!data || absIdx < 0 || absIdx >= data.length) return null
    return data[absIdx]
  }

  // Get all values at an absolute index
  function getValuesAtAbsIndex(absIdx) {
    const vals = []
    for (const slug of selectedSensors) {
      const v = getValueAt(slug, absIdx)
      if (v !== null) {
        vals.push({ slug, color: colors[slug] || '#888', value: v, unit: unitLabels[slug] || '' })
      }
    }
    return vals
  }

  // Get the active cursor absolute index
  function getCursorAbsIndex() {
    if (pinnedAbsIndex !== null) return pinnedAbsIndex
    if (hoverX !== null) return xToAbsIndex(hoverX)
    return null
  }

  // Format elapsed ms as m:ss.s or h:mm:ss
  function formatElapsed(ms) {
    if (ms == null || isNaN(ms)) return '—'
    const totalSec = ms / 1000
    if (totalSec < 3600) {
      const m = Math.floor(totalSec / 60)
      const s = (totalSec % 60).toFixed(1)
      return `${m}:${s.padStart(4, '0')}`
    }
    const h = Math.floor(totalSec / 3600)
    const m = Math.floor((totalSec % 3600) / 60)
    const s = Math.floor(totalSec % 60)
    return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  }

  // Get elapsed time at an absolute index
  function getTimeAtIndex(absIdx) {
    if (historyTimes && absIdx >= 0 && absIdx < historyTimes.length) {
      return historyTimes[absIdx]
    }
    return null
  }

  // Check if a canvas Y coordinate is in the scrollbar region
  function isInScrollbar(canvasY) {
    if (!canvas) return false
    return canvasY >= canvas.height - scrollBarH - 6
  }

  // Convert canvas X to a scrollbar viewEnd value
  function scrollbarXToViewEnd(canvasX) {
    const w = canvas.width
    const pad = { left: 10, right: 10 }
    const trackW = w - pad.left - pad.right
    const maxLen = getMaxLen()
    const thumbW = Math.max(30, (viewSize / maxLen) * trackW)
    const scrollRange = trackW - thumbW
    const frac = Math.max(0, Math.min(1, (canvasX - pad.left - thumbW / 2) / scrollRange))
    return Math.round(viewSize + frac * (maxLen - viewSize))
  }

  // Mouse handlers
  function handleMouseMove(e) {
    if (!canvas) return
    const rect = canvas.getBoundingClientRect()
    const cx = (e.clientX - rect.left) * (canvas.width / rect.width)
    const cy = (e.clientY - rect.top) * (canvas.height / rect.height)

    if (isDraggingScrollbar) {
      const maxLen = getMaxLen()
      const dx = cx - dragStartX
      const pad = { left: 10, right: 10 }
      const trackW = canvas.width - pad.left - pad.right
      const thumbW = Math.max(30, (viewSize / maxLen) * trackW)
      const scrollRange = trackW - thumbW
      const dSamples = (dx / scrollRange) * (maxLen - viewSize)
      viewEnd = Math.max(viewSize, Math.min(maxLen, Math.round(dragStartViewEnd + dSamples)))
      isLive = viewEnd >= maxLen
      drawGraph()
      return
    }

    // Update cursor style
    if (isInScrollbar(cy) && getMaxLen() > viewSize) {
      canvas.style.cursor = 'pointer'
    } else {
      canvas.style.cursor = 'crosshair'
    }

    hoverX = cx
    drawGraph()
  }

  function handleMouseLeave() {
    hoverX = null
    if (!isDraggingScrollbar) drawGraph()
  }

  function handleMouseDown(e) {
    if (!canvas) return
    const rect = canvas.getBoundingClientRect()
    const cx = (e.clientX - rect.left) * (canvas.width / rect.width)
    const cy = (e.clientY - rect.top) * (canvas.height / rect.height)
    const maxLen = getMaxLen()

    if (isInScrollbar(cy) && maxLen > viewSize) {
      e.preventDefault()
      isDraggingScrollbar = true
      dragStartX = cx

      if (isLive) {
        viewEnd = maxLen
        isLive = false
      }
      dragStartViewEnd = viewEnd

      // If clicking outside the thumb, jump to that position first
      const pad = { left: 10, right: 10 }
      const trackW = canvas.width - pad.left - pad.right
      const thumbW = Math.max(30, (viewSize / maxLen) * trackW)
      const { start } = getViewBounds()
      const thumbX = pad.left + (start / maxLen) * trackW
      if (cx < thumbX || cx > thumbX + thumbW) {
        viewEnd = scrollbarXToViewEnd(cx)
        dragStartViewEnd = viewEnd
        isLive = viewEnd >= maxLen
      }

      drawGraph()

      // Listen for global mouse events during drag
      window.addEventListener('mousemove', handleGlobalMouseMove)
      window.addEventListener('mouseup', handleGlobalMouseUp)
    }
  }

  function handleGlobalMouseMove(e) {
    if (!isDraggingScrollbar || !canvas) return
    const rect = canvas.getBoundingClientRect()
    const cx = (e.clientX - rect.left) * (canvas.width / rect.width)
    const maxLen = getMaxLen()
    const pad = { left: 10, right: 10 }
    const trackW = canvas.width - pad.left - pad.right
    const thumbW = Math.max(30, (viewSize / maxLen) * trackW)
    const scrollRange = trackW - thumbW
    const dx = cx - dragStartX
    const dSamples = (dx / scrollRange) * (maxLen - viewSize)
    viewEnd = Math.max(viewSize, Math.min(maxLen, Math.round(dragStartViewEnd + dSamples)))
    isLive = viewEnd >= maxLen
    drawGraph()
  }

  function handleGlobalMouseUp() {
    isDraggingScrollbar = false
    window.removeEventListener('mousemove', handleGlobalMouseMove)
    window.removeEventListener('mouseup', handleGlobalMouseUp)
  }

  function handleClick(e) {
    if (!canvas) return
    const rect = canvas.getBoundingClientRect()
    const cy = (e.clientY - rect.top) * (canvas.height / rect.height)

    // Don't process click as pin if clicking on scrollbar
    if (isInScrollbar(cy)) return

    const cx = (e.clientX - rect.left) * (canvas.width / rect.width)
    const clickedAbs = xToAbsIndex(cx)

    if (pinnedAbsIndex !== null && pinnedAbsIndex === clickedAbs) {
      pinnedAbsIndex = null   // unpin
    } else {
      pinnedAbsIndex = clickedAbs
    }
    drawGraph()
  }

  function handleWheel(e) {
    e.preventDefault()
    const maxLen = getMaxLen()
    if (maxLen <= viewSize) return

    // Support both vertical and horizontal scroll
    const delta = Math.abs(e.deltaX) > Math.abs(e.deltaY) ? e.deltaX : e.deltaY
    const scrollAmount = Math.max(1, Math.round(viewSize * 0.1)) * Math.sign(delta)

    if (isLive) {
      viewEnd = maxLen
      isLive = false
    }

    viewEnd = Math.max(viewSize, Math.min(maxLen, viewEnd + scrollAmount))

    if (viewEnd >= maxLen) {
      isLive = true
    }

    drawGraph()
  }

  function goToLive() {
    isLive = true
    drawGraph()
  }

  function handleKeyDown(e) {
    if (pinnedAbsIndex === null) return
    const maxLen = getMaxLen()
    if (maxLen === 0) return

    let newIdx = pinnedAbsIndex
    if (e.key === 'ArrowLeft') {
      newIdx = Math.max(0, pinnedAbsIndex - 1)
    } else if (e.key === 'ArrowRight') {
      newIdx = Math.min(maxLen - 1, pinnedAbsIndex + 1)
    } else if (e.key === 'Escape') {
      pinnedAbsIndex = null
      drawGraph()
      return
    } else {
      return
    }

    e.preventDefault()
    pinnedAbsIndex = newIdx

    // Auto-scroll viewport to keep pin visible
    const { start, end } = getViewBounds()
    if (newIdx < start) {
      viewEnd = newIdx + viewSize
      isLive = false
    } else if (newIdx >= end) {
      viewEnd = newIdx + 1
      isLive = viewEnd >= maxLen
    }

    drawGraph()
  }

  // React to history changes from parent (new samples or loaded file)
  $: if (historyVersion >= 0 && history) {
    // Auto-select sensors from file data if we haven't already
    if (isFileMode && !sensorsAutoSelected) {
      const available = Object.keys(history).filter(s => history[s] && history[s].length > 0)
      if (available.length > 0) {
        // Prefer common sensors, fall back to whatever is available
        const preferred = ['RPM', 'TPS', 'COOL', 'O2-R', 'TIMA', 'INJP']
        const picked = preferred.filter(s => available.includes(s))
        if (picked.length > 0) {
          selectedSensors = picked
        } else {
          selectedSensors = available.slice(0, 6)
        }
        sensorsAutoSelected = true
        isLive = false
        viewEnd = getMaxLen()
      }
    }
    drawGraph()
  }

  // Reset auto-select flag when switching away from file mode
  $: if (!isFileMode) {
    sensorsAutoSelected = false
    isLive = true
  }

  function normalize(value, slug) {
    const [min, max] = ranges[slug] || [0, 255]
    return (value - min) / (max - min)
  }

  function drawGraph() {
    if (!canvas || !ctx) return

    const w = canvas.width
    const h = canvas.height
    const pad = { top: 10, bottom: 42, left: 10, right: 10 }
    const plotW = w - pad.left - pad.right
    const plotH = h - pad.top - pad.bottom
    const { start, end, maxLen } = getViewBounds()

    ctx.clearRect(0, 0, w, h)

    // Background grid
    ctx.strokeStyle = '#2a2a4e'
    ctx.lineWidth = 0.5
    for (let i = 0; i <= 4; i++) {
      const y = pad.top + (plotH * i) / 4
      ctx.beginPath()
      ctx.moveTo(pad.left, y)
      ctx.lineTo(w - pad.right, y)
      ctx.stroke()
    }

    // Draw traces
    const visibleCount = end - start
    if (visibleCount > 0) {
      for (const slug of selectedSensors) {
        const data = history[slug]
        if (!data || data.length < 2) continue

        const color = colors[slug] || '#888'
        ctx.strokeStyle = color
        ctx.lineWidth = 1.5
        ctx.beginPath()
        let started = false

        for (let abs = start; abs < end; abs++) {
          if (abs < 0 || abs >= data.length) continue
          const screenFrac = (abs - start) / (viewSize - 1)
          const x = pad.left + screenFrac * plotW
          const norm = normalize(data[abs], slug)
          const y = pad.top + plotH - norm * plotH
          if (!started) { ctx.moveTo(x, y); started = true }
          else ctx.lineTo(x, y)
        }
        ctx.stroke()
      }
    }

    // Crosshair cursor
    const cursorAbs = getCursorAbsIndex()
    if (cursorAbs !== null && cursorAbs >= start && cursorAbs < end) {
      const screenFrac = (cursorAbs - start) / (viewSize - 1)
      const cursorX = pad.left + screenFrac * plotW
      const isPinned = pinnedAbsIndex !== null

      // Vertical line
      ctx.strokeStyle = isPinned ? 'rgba(255, 255, 255, 0.8)' : 'rgba(255, 255, 255, 0.4)'
      ctx.lineWidth = isPinned ? 1.5 : 1
      ctx.setLineDash(isPinned ? [] : [4, 4])
      ctx.beginPath()
      ctx.moveTo(cursorX, pad.top)
      ctx.lineTo(cursorX, pad.top + plotH)
      ctx.stroke()
      ctx.setLineDash([])

      // Dots on traces
      const vals = getValuesAtAbsIndex(cursorAbs)
      cursorValues = vals

      for (const v of vals) {
        const norm = normalize(v.value, v.slug)
        const dotY = pad.top + plotH - norm * plotH
        ctx.fillStyle = v.color
        ctx.beginPath()
        ctx.arc(cursorX, dotY, 4, 0, Math.PI * 2)
        ctx.fill()
        ctx.strokeStyle = '#fff'
        ctx.lineWidth = 1
        ctx.stroke()
      }

      // Value readout panel
      if (vals.length > 0) {
        const panelW = 148
        const lineH = 16
        const panelH = vals.length * lineH + 32
        const panelX = cursorX > w / 2 ? pad.left + 8 : w - pad.right - panelW - 4
        const panelY = pad.top + 4

        ctx.fillStyle = 'rgba(15, 15, 30, 0.92)'
        ctx.strokeStyle = isPinned ? 'rgba(255, 255, 255, 0.3)' : 'rgba(255, 255, 255, 0.15)'
        ctx.lineWidth = 1
        ctx.beginPath()
        ctx.roundRect(panelX, panelY, panelW, panelH, 4)
        ctx.fill()
        ctx.stroke()

        // Time + sample label
        const cursorMs = getTimeAtIndex(cursorAbs)
        const timeStr = cursorMs !== null ? formatElapsed(cursorMs) : '—'
        ctx.font = '9px monospace'
        ctx.fillStyle = '#8080a0'
        ctx.fillText(`T ${timeStr}  #${cursorAbs}/${maxLen}`, panelX + 8, panelY + 11)

        ctx.font = '11px monospace'
        for (let i = 0; i < vals.length; i++) {
          const v = vals[i]
          const rowY = panelY + 26 + i * lineH

          ctx.fillStyle = v.color
          ctx.beginPath()
          ctx.arc(panelX + 10, rowY - 3, 3, 0, Math.PI * 2)
          ctx.fill()

          ctx.fillStyle = '#a0a0b0'
          ctx.fillText(v.slug, panelX + 18, rowY)

          ctx.fillStyle = '#e0e0e0'
          const valText = `${v.value.toFixed(1)}${v.unit}`
          const valWidth = ctx.measureText(valText).width
          ctx.fillText(valText, panelX + panelW - valWidth - 8, rowY)
        }
      }
    } else {
      cursorValues = []
      // If pin scrolled out of view, unpin
      if (pinnedAbsIndex !== null && (pinnedAbsIndex < start || pinnedAbsIndex >= end)) {
        // Keep pin but don't show values — user can scroll back to find it
      }
    }

    // Time labels on X axis edges
    const startMs = getTimeAtIndex(start)
    const endMs = getTimeAtIndex(Math.min(end - 1, maxLen - 1))
    ctx.font = '10px monospace'
    ctx.fillStyle = '#505068'
    if (startMs !== null) {
      ctx.fillText(formatElapsed(startMs), pad.left + 2, pad.top + plotH + 14)
    }
    if (endMs !== null) {
      const endLabel = formatElapsed(endMs)
      const ew = ctx.measureText(endLabel).width
      ctx.fillText(endLabel, w - pad.right - ew - 2, pad.top + plotH + 14)
    }

    // Bottom bar: legend
    const legendY = h - scrollBarH - 14
    ctx.font = '11px monospace'
    let legendX = pad.left + 4
    for (const slug of selectedSensors) {
      const data = history[slug]
      const color = colors[slug] || '#888'
      const lastVal = data && data.length > 0 ? data[data.length - 1] : null

      ctx.fillStyle = color
      ctx.fillRect(legendX, legendY, 8, 8)
      legendX += 12

      const label = lastVal !== null ? `${slug}: ${lastVal.toFixed(1)}` : slug
      ctx.fillText(label, legendX, legendY + 8)
      legendX += ctx.measureText(label).width + 16
    }

    // Scroll position text
    if (maxLen > viewSize) {
      ctx.fillStyle = '#606070'
      ctx.font = '10px monospace'
      const scrollText = isLive ? 'LIVE' : `${start}–${end} / ${maxLen}`
      const tw = ctx.measureText(scrollText).width
      ctx.fillText(scrollText, w - pad.right - tw - 4, legendY + 8)
    }

    // Scrollbar (always drawn, thicker, draggable)
    const barY = h - scrollBarH - 2
    const barTrackW = plotW
    // Track background
    ctx.fillStyle = '#1a1a3e'
    ctx.beginPath()
    ctx.roundRect(pad.left, barY, barTrackW, scrollBarH, scrollBarH / 2)
    ctx.fill()

    if (maxLen > viewSize) {
      // Thumb
      const thumbW = Math.max(30, (viewSize / maxLen) * barTrackW)
      const thumbX = pad.left + (start / maxLen) * barTrackW
      ctx.fillStyle = isDraggingScrollbar ? '#fff' : (isLive ? '#4ade80' : '#e94560')
      ctx.beginPath()
      ctx.roundRect(thumbX, barY, thumbW, scrollBarH, scrollBarH / 2)
      ctx.fill()
    } else {
      // Full thumb when all data fits
      ctx.fillStyle = '#4ade8040'
      ctx.beginPath()
      ctx.roundRect(pad.left, barY, barTrackW, scrollBarH, scrollBarH / 2)
      ctx.fill()
    }
  }

  function resizeCanvas() {
    if (!canvas || !graphContainer) return
    canvas.width = graphContainer.clientWidth
    canvas.height = graphContainer.clientHeight
    drawGraph()
  }

  onMount(() => {
    ctx = canvas.getContext('2d')
    resizeCanvas()
    window.addEventListener('resize', resizeCanvas)
    window.addEventListener('keydown', handleKeyDown)
  })

  onDestroy(() => {
    window.removeEventListener('resize', resizeCanvas)
    window.removeEventListener('keydown', handleKeyDown)
    window.removeEventListener('mousemove', handleGlobalMouseMove)
    window.removeEventListener('mouseup', handleGlobalMouseUp)
  })
</script>

<div class="card">
  <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
    <h2 style="margin: 0;">Real-Time Graph</h2>
    <div style="display: flex; align-items: center; gap: 8px;">
      {#if pinnedAbsIndex !== null}
        <span style="font-size: 11px; color: var(--accent-yellow);">PINNED #{pinnedAbsIndex}</span>
        <button class="btn btn-sm" style="font-size: 10px;" on:click={() => { pinnedAbsIndex = null; drawGraph() }}>Unpin</button>
      {/if}
      {#if !isLive && getMaxLen() > viewSize}
        <button class="btn btn-sm btn-primary" style="font-size: 10px;" on:click={goToLive}>Go to Live</button>
      {/if}
      {#if !isLive}
        <span style="font-size: 11px; color: var(--text-muted);">Scroll: mousewheel</span>
      {/if}
    </div>
  </div>
  <div style="display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 8px;">
    {#each activeSlugs as slug}
      <button
        class="btn btn-sm"
        style="font-size: 11px; {selectedSensors.includes(slug) ? `background: ${colors[slug] || '#555'}; color: white; border-color: ${colors[slug] || '#555'}` : ''}"
        on:click={() => toggleSensor(slug)}
      >
        {slug}
      </button>
    {/each}
  </div>
</div>

<div class="graph-container" bind:this={graphContainer}>
  <canvas
    bind:this={canvas}
    on:mousemove={handleMouseMove}
    on:mouseleave={handleMouseLeave}
    on:mousedown={handleMouseDown}
    on:click={handleClick}
    on:wheel={handleWheel}
  ></canvas>
</div>

<style>
  canvas {
    width: 100%;
    height: 100%;
    display: block;
    cursor: crosshair;
  }
</style>
