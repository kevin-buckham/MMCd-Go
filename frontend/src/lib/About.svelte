<script>
  let about = null

  const wails = window.go?.main?.App

  async function loadAbout() {
    try {
      about = await wails?.GetAboutInfo()
    } catch (e) {
      console.error('Failed to load about info:', e)
    }
  }

  loadAbout()
</script>

<div class="card about-card">
  {#if about}
    <div class="about-header">
      <h2>{about.name}</h2>
      <span class="version">v{about.version}</span>
    </div>

    <p class="description">{about.description}</p>

    <div class="about-details">
      <div class="detail-row">
        <span class="detail-label">Developers</span>
        <span class="detail-value">{about.developers}</span>
      </div>
      <div class="detail-row">
        <span class="detail-label">License</span>
        <span class="detail-value">{about.license}</span>
      </div>
      <div class="detail-row">
        <span class="detail-label">Source</span>
        <span class="detail-value">{about.url}</span>
      </div>
      <div class="detail-row">
        <span class="detail-label"></span>
        <span class="detail-value" style="font-size: 11px; color: var(--text-muted);">{about.copyright}</span>
      </div>
    </div>

    <p class="attribution">{about.attribution}</p>

    <div class="about-section">
      <h3>Features</h3>
      <ul>
        <li>Real-time sensor datalogging at 1953 baud over ALDL/OBDI serial</li>
        <li>22 sensor channels with live dashboard and scrollable graph</li>
        <li>Diagnostic trouble code (DTC) reading and clearing</li>
        <li>Actuator/solenoid testing (fuel pump, injectors, EGR, etc.)</li>
        <li>CSV and native binary log recording</li>
        <li>Import and review PalmOS PDB logs from original MMCd 1.5</li>
        <li>Demo/simulator mode for UI development without hardware</li>
        <li>Cross-platform desktop GUI and headless CLI</li>
      </ul>
    </div>

    <div class="about-section">
      <h3>Supported Vehicles</h3>
      <p>1990–1994 Mitsubishi Eclipse, Eagle Talon, Plymouth Laser (1G DSM) with 4G63 ECU</p>
    </div>
  {:else}
    <p style="color: var(--text-muted)">Loading...</p>
  {/if}
</div>

<style>
  .about-card {
    max-width: 600px;
  }

  .about-header {
    display: flex;
    align-items: baseline;
    gap: 12px;
    margin-bottom: 16px;
  }

  .about-header h2 {
    margin: 0;
  }

  .version {
    font-family: var(--font-mono);
    font-size: 14px;
    color: var(--accent-green);
    font-weight: 600;
  }

  .description {
    color: var(--text-secondary);
    font-size: 14px;
    line-height: 1.5;
    margin-bottom: 20px;
  }

  .about-details {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-bottom: 24px;
    padding: 12px;
    background: var(--bg-input);
    border-radius: 6px;
    border: 1px solid var(--border);
  }

  .detail-row {
    display: flex;
    gap: 16px;
  }

  .detail-label {
    font-size: 12px;
    color: var(--text-muted);
    min-width: 80px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .detail-value {
    font-size: 13px;
    color: var(--text-primary);
    font-family: var(--font-mono);
  }

  .attribution {
    font-size: 13px;
    color: var(--text-muted);
    font-style: italic;
    margin-bottom: 20px;
  }

  .about-section {
    margin-bottom: 16px;
  }

  .about-section h3 {
    font-size: 13px;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
    margin-bottom: 8px;
  }

  .about-section ul {
    list-style: none;
    padding: 0;
  }

  .about-section li {
    font-size: 13px;
    color: var(--text-secondary);
    padding: 3px 0;
    padding-left: 16px;
    position: relative;
  }

  .about-section li::before {
    content: '·';
    position: absolute;
    left: 4px;
    color: var(--accent);
    font-weight: bold;
  }

  .about-section p {
    font-size: 13px;
    color: var(--text-secondary);
    line-height: 1.5;
  }
</style>
