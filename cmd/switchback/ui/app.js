// ==========================================================================
// SWITCHBACK v1.2.0 - MODULAR CLEAN CONTROLLER
// ==========================================================================

class PixelAudio {
  constructor() {
    this.ctx = null;
    this.enabled = true;
  }

  init() {
    if (!this.ctx) {
      const AudioCtx = window.AudioContext || window.webkitAudioContext;
      this.ctx = new AudioCtx();
    }
  }

  playClick() {
    if (!this.enabled) return;
    this.init();
    const osc = this.ctx.createOscillator();
    const gain = this.ctx.createGain();

    osc.type = 'triangle';
    osc.frequency.setValueAtTime(440, this.ctx.currentTime);
    osc.frequency.exponentialRampToValueAtTime(120, this.ctx.currentTime + 0.05);

    gain.gain.setValueAtTime(0.2, this.ctx.currentTime);
    gain.gain.linearRampToValueAtTime(0.01, this.ctx.currentTime + 0.05);

    osc.connect(gain);
    gain.connect(this.ctx.destination);

    osc.start();
    osc.stop(this.ctx.currentTime + 0.05);
  }

  playWarp() {
    if (!this.enabled) return;
    this.init();
    const osc = this.ctx.createOscillator();
    const gain = this.ctx.createGain();

    osc.type = 'sine';
    osc.frequency.setValueAtTime(260, this.ctx.currentTime);
    osc.frequency.exponentialRampToValueAtTime(980, this.ctx.currentTime + 0.25);

    gain.gain.setValueAtTime(0.3, this.ctx.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.01, this.ctx.currentTime + 0.3);

    osc.connect(gain);
    gain.connect(this.ctx.destination);

    osc.start();
    osc.stop(this.ctx.currentTime + 0.3);
  }

  playBeep(pitch = 520) {
    if (!this.enabled) return;
    this.init();
    const osc = this.ctx.createOscillator();
    const gain = this.ctx.createGain();

    osc.type = 'square';
    osc.frequency.setValueAtTime(pitch, this.ctx.currentTime);

    gain.gain.setValueAtTime(0.12, this.ctx.currentTime);
    gain.gain.linearRampToValueAtTime(0.01, this.ctx.currentTime + 0.08);

    osc.connect(gain);
    gain.connect(this.ctx.destination);

    osc.start();
    osc.stop(this.ctx.currentTime + 0.08);
  }

  playFanfare() {
    if (!this.enabled) return;
    this.init();
    const notes = [440, 554, 659, 880];
    notes.forEach((freq, idx) => {
      const osc = this.ctx.createOscillator();
      const gain = this.ctx.createGain();
      osc.type = 'square';
      osc.frequency.setValueAtTime(freq, this.ctx.currentTime + idx * 0.08);
      gain.gain.setValueAtTime(0.15, this.ctx.currentTime + idx * 0.08);
      gain.gain.exponentialRampToValueAtTime(0.01, this.ctx.currentTime + idx * 0.08 + 0.15);
      osc.connect(gain);
      gain.connect(this.ctx.destination);
      osc.start(this.ctx.currentTime + idx * 0.08);
      osc.stop(this.ctx.currentTime + idx * 0.08 + 0.15);
    });
  }
}

const audio = new PixelAudio();

// Elements
const selectAgentsEl = document.getElementById('select-detected-agents');
const selectAppsEl = document.getElementById('select-open-apps');
const selectedAgentNameEl = document.getElementById('selected-agent-name');
const selectedAgentHwndEl = document.getElementById('selected-agent-hwnd');
const selectedWorkNameEl = document.getElementById('selected-work-name');
const selectedWorkHwndEl = document.getElementById('selected-work-hwnd');

// Toggles
const toggleGamingEl = document.getElementById('toggle-gaming-mode');
const toggleAutoSwitchEl = document.getElementById('toggle-auto-switch');
const toggleFullscreenGuardEl = document.getElementById('toggle-fullscreen-guard');
const toggleMeetingGuardEl = document.getElementById('toggle-meeting-guard');
const toggleMediaControlEl = document.getElementById('toggle-media-control');
const toggleMobilePrimaryDesktopEl = document.getElementById('toggle-mobile-primary-desktop');

// Master Control
const btnMasterStop = document.getElementById('btn-master-stop');
const masterIcon = document.getElementById('master-icon');
const masterStatusText = document.getElementById('master-status-text');
const statusBadgeEl = document.getElementById('system-status-badge');

// Logs & Overlays
const chatLogsEl = document.getElementById('chat-logs');
const countdownBanner = document.getElementById('countdown-banner');
const countdownText = document.getElementById('countdown-text');

let isCurrentlyStopped = false;
let detectedAgentsList = [];
let openAppsList = [];

// Log helper
function logChat(sender, message, type = 'info') {
  const line = document.createElement('div');
  line.className = `chat-line ${type}`;
  line.innerHTML = `<span class="chat-sender">&lt;${sender}&gt;</span> ${escapeHtml(message)}`;
  chatLogsEl.appendChild(line);
  chatLogsEl.scrollTop = chatLogsEl.scrollHeight;
}

function escapeHtml(str) {
  return str.replace(/[&<>'"]/g, tag => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    "'": '&#39;',
    '"': '&quot;'
  }[tag] || tag));
}

function truncateString(str, maxLen = 42) {
  if (!str) return '';
  if (str.length <= maxLen) return str;
  return str.substring(0, maxLen - 3) + '...';
}

// Fetch open windows and populate selectors
async function fetchWindows() {
  try {
    const res = await fetch('/api/windows');
    if (!res.ok) return;
    const data = await res.json();

    detectedAgentsList = data.agents || [];
    openAppsList = data.work_apps || [];

    // Populate Agents Dropdown
    selectAgentsEl.innerHTML = '';
    if (detectedAgentsList.length === 0) {
      selectAgentsEl.innerHTML = '<option value="">No agents found (launch Antigravity, Claude, Cursor, Windsurf)</option>';
    } else {
      selectAgentsEl.innerHTML = '<option value="">-- Choose an AI Agent Window --</option>';
      detectedAgentsList.forEach(a => {
        const opt = document.createElement('option');
        opt.value = a.hwnd;
        const fullLabel = `[${a.agent_type.toUpperCase()}] ${a.title} (${a.process_name})`;
        opt.textContent = truncateString(fullLabel, 48);
        opt.title = fullLabel;
        selectAgentsEl.appendChild(opt);
      });
    }

    // Populate Open Apps Dropdown
    selectAppsEl.innerHTML = '<option value="">-- Pick a running window as switch target --</option>';
    openAppsList.forEach(w => {
      const opt = document.createElement('option');
      opt.value = w.hwnd;
      const fullLabel = `${w.title} (${w.process_name})`;
      opt.textContent = truncateString(fullLabel, 48);
      opt.title = fullLabel;
      selectAppsEl.appendChild(opt);
    });

  } catch (e) {}
}

// Fetch active status
async function fetchStatus() {
  try {
    const res = await fetch('/api/status');
    if (!res.ok) return;
    const data = await res.json();

    if (selectedAgentNameEl) {
      const title = data.selected_agent_title || 'Auto-Detect Active';
      selectedAgentNameEl.textContent = title;
      selectedAgentNameEl.title = title;
    }
    if (selectedAgentHwndEl) selectedAgentHwndEl.textContent = data.selected_agent_hwnd || '0';

    if (selectedWorkNameEl) {
      const title = data.selected_work_title || 'None Selected (Pick Target)';
      selectedWorkNameEl.textContent = title;
      selectedWorkNameEl.title = title;
    }
    if (selectedWorkHwndEl) selectedWorkHwndEl.textContent = data.selected_work_hwnd || '0';

    if (toggleGamingEl) toggleGamingEl.checked = !!data.gaming_mode;
    if (toggleAutoSwitchEl) toggleAutoSwitchEl.checked = !!data.auto_switch_enabled;
    if (toggleFullscreenGuardEl) toggleFullscreenGuardEl.checked = !!data.fullscreen_guard;
    if (toggleMeetingGuardEl) toggleMeetingGuardEl.checked = !!data.meeting_guard;
    if (toggleMediaControlEl) toggleMediaControlEl.checked = !!data.media_control;
    if (toggleMobilePrimaryDesktopEl) toggleMobilePrimaryDesktopEl.checked = !!data.mobile_control_active;
    isCurrentlyStopped = !!data.stopped;

    // Master Stop button UI state
    if (isCurrentlyStopped) {
      if (masterIcon) masterIcon.textContent = '▶';
      if (masterStatusText) masterStatusText.textContent = 'RESUME SWITCHBACK';
      if (btnMasterStop) {
        btnMasterStop.classList.remove('pixel-btn-danger');
        btnMasterStop.classList.add('pixel-btn-resume');
      }
      if (statusBadgeEl) {
        statusBadgeEl.textContent = '⏸ PAUSED';
        statusBadgeEl.classList.add('badge-stopped');
      }
    } else {
      if (masterIcon) masterIcon.textContent = '⏹';
      if (masterStatusText) masterStatusText.textContent = 'PAUSE SWITCHBACK';
      if (btnMasterStop) {
        btnMasterStop.classList.add('pixel-btn-danger');
        btnMasterStop.classList.remove('pixel-btn-resume');
      }
      if (statusBadgeEl) {
        statusBadgeEl.classList.remove('badge-stopped');
        if (data.current_status === 'agent_working') {
          statusBadgeEl.textContent = '⚡ AGENT WORKING';
          statusBadgeEl.style.background = 'var(--pixel-diamond)';
        } else if (data.current_status === 'permission_needed') {
          statusBadgeEl.textContent = '🔔 PERMISSION NEEDED';
          statusBadgeEl.style.background = 'var(--pixel-gold)';
        } else {
          statusBadgeEl.textContent = '● RUNNING';
          statusBadgeEl.style.background = 'var(--pixel-emerald)';
        }
      }
    }

    // Hook indicators
    const hooksStatusBadge = document.getElementById('hooks-status-badge');
    const agDot = document.getElementById('ag-dot');
    const agBadge = document.getElementById('ag-badge');
    const claudeDot = document.getElementById('claude-dot');
    const claudeBadge = document.getElementById('claude-badge');

    if (data.antigravity_installed || data.claude_installed) {
      if (hooksStatusBadge) {
        hooksStatusBadge.textContent = 'INSTALLED';
        hooksStatusBadge.className = 'badge-red';
      }
    } else {
      if (hooksStatusBadge) {
        hooksStatusBadge.textContent = 'DISABLED';
        hooksStatusBadge.className = 'hook-badge';
      }
    }

    if (agDot && agBadge) {
      if (data.antigravity_installed) {
        agDot.className = 'status-dot active';
        agBadge.className = 'hook-badge active';
        agBadge.textContent = 'ACTIVE';
      } else {
        agDot.className = 'status-dot';
        agBadge.className = 'hook-badge';
        agBadge.textContent = 'DISABLED';
      }
    }

    if (claudeDot && claudeBadge) {
      if (data.claude_installed) {
        claudeDot.className = 'status-dot active';
        claudeBadge.className = 'hook-badge active';
        claudeBadge.textContent = 'ACTIVE';
      } else {
        claudeDot.className = 'status-dot';
        claudeBadge.className = 'hook-badge';
        claudeBadge.textContent = 'DISABLED';
      }
    }

  } catch (e) {}
}

// Poll server logs
async function fetchLogs() {
  try {
    const res = await fetch('/api/logs');
    if (!res.ok) return;
    const lines = await res.json();
    if (Array.isArray(lines) && lines.length > 0) {
      chatLogsEl.innerHTML = '';
      lines.forEach(l => {
        let type = 'info';
        if (l.includes('[ERROR]')) type = 'error';
        else if (l.includes('[GamingMode]') || l.includes('[MeetingGuard]')) type = 'warn';
        else if (l.includes('[AutoSwitch]') || l.includes('[Media]')) type = 'success';
        logChat('System', l, type);
      });
    }
  } catch (e) {}
}

// Pick Work Window by fast switching
async function pickBySwitching() {
  audio.playClick();
  countdownBanner.classList.remove('hidden');

  let seconds = 3;
  countdownText.textContent = `SWITCH TO WINDOW IN ${seconds}...`;
  audio.playBeep(440);

  // Send request to capture target window
  const switchPromise = fetch('/api/pick-work-switch', { method: 'POST' }).then(r => r.json());

  const timer = setInterval(() => {
    seconds--;
    if (seconds > 0) {
      countdownText.textContent = `SWITCH TO WINDOW IN ${seconds}...`;
      audio.playBeep(440 + (3 - seconds) * 100);
    } else {
      clearInterval(timer);
      countdownText.textContent = `CAPTURING ACTIVE WINDOW...`;
    }
  }, 1000);

  const res = await switchPromise;
  countdownBanner.classList.add('hidden');

  if (res.success) {
    audio.playFanfare();
    logChat('Target', `Locked in Work Window: ${res.title} (HWND: ${res.hwnd})`, 'success');
    fetchStatus();
  } else {
    logChat('Target', res.message || 'Failed to capture window.', 'error');
  }
}

// Save Mode changes
async function updateModes() {
  audio.playClick();
  const gaming = toggleGamingEl ? toggleGamingEl.checked : false;
  const autoSwitch = toggleAutoSwitchEl ? toggleAutoSwitchEl.checked : true;
  const fullscreen = toggleFullscreenGuardEl ? toggleFullscreenGuardEl.checked : true;
  const meeting = toggleMeetingGuardEl ? toggleMeetingGuardEl.checked : true;
  const media = toggleMediaControlEl ? toggleMediaControlEl.checked : true;
  const mobilePrimary = toggleMobilePrimaryDesktopEl ? toggleMobilePrimaryDesktopEl.checked : false;

  try {
    await fetch('/api/toggle-mode', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        gaming_mode: gaming,
        auto_switch_enabled: autoSwitch,
        fullscreen_guard: fullscreen,
        meeting_guard: meeting,
        media_control: media,
        mobile_control_active: mobilePrimary,
        stopped: isCurrentlyStopped
      })
    });

    logChat('Modes', `Updated (AutoSwitch: ${autoSwitch}, MediaPause: ${media}, MobileSuppress: ${mobilePrimary})`, 'info');
    fetchStatus();
  } catch (e) {}
}

// Master Stop / Resume Toggle
async function toggleMasterStop() {
  audio.playClick();
  try {
    const res = await fetch('/api/toggle-stopped', { method: 'POST' });
    const data = await res.json();
    isCurrentlyStopped = !!data.stopped;
    if (data.stopped) {
      logChat('Master', '⏹ SWITCHBACK PAUSED. All window auto-focusing is paused.', 'warn');
    } else {
      audio.playFanfare();
      logChat('Master', '▶ SWITCHBACK RESUMED. Auto-focusing is ACTIVE.', 'success');
    }
    fetchStatus();
  } catch (e) {}
}

// Quick Preset Filter Handlers
function setupPresets() {
  document.querySelectorAll('.preset-chip').forEach(btn => {
    btn.addEventListener('click', async () => {
      audio.playClick();
      const filterTerms = btn.dataset.filter.split(',').map(s => s.trim().toLowerCase());
      
      // Find matching open app
      const match = openAppsList.find(app => {
        const title = (app.title || '').toLowerCase();
        const proc = (app.process_name || '').toLowerCase();
        return filterTerms.some(term => title.includes(term) || proc.includes(term));
      });

      if (match) {
        await fetch('/api/select-work', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ hwnd: match.hwnd, title: match.title })
        });
        audio.playFanfare();
        logChat('Preset', `Selected target: ${match.title}`, 'success');
        fetchStatus();
      } else {
        logChat('Preset', `No running app matching [${btn.textContent.trim()}] found. Open the app or use "Pick Target"`, 'warn');
      }
    });
  });
}

// Event Listeners
function setupEvents() {
  // Sound Toggle
  const soundBtn = document.getElementById('btn-sound-toggle');
  const soundStatus = document.getElementById('sound-status');
  const soundIcon = document.getElementById('sound-icon');
  if (soundBtn) {
    soundBtn.addEventListener('click', () => {
      audio.enabled = !audio.enabled;
      soundStatus.textContent = audio.enabled ? 'ON' : 'OFF';
      soundIcon.textContent = audio.enabled ? '🔊' : '🔇';
      if (audio.enabled) audio.playClick();
    });
  }

  // Master Stop button
  if (btnMasterStop) btnMasterStop.addEventListener('click', toggleMasterStop);

  // Refresh Agents
  const btnRefreshAgents = document.getElementById('btn-refresh-agents');
  if (btnRefreshAgents) {
    btnRefreshAgents.addEventListener('click', () => {
      audio.playClick();
      logChat('Radar', 'Scanning running processes for AI agents...', 'info');
      fetchWindows();
    });
  }

  // Select Agent from Dropdown
  if (selectAgentsEl) {
    selectAgentsEl.addEventListener('change', async () => {
      const hwnd = parseInt(selectAgentsEl.value, 10);
      if (!hwnd) return;
      const selected = detectedAgentsList.find(a => a.hwnd === hwnd);
      if (selected) {
        audio.playClick();
        await fetch('/api/select-agent', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ hwnd: selected.hwnd, title: selected.title, agent_type: selected.agent_type })
        });
        logChat('Agent', `Selected Agent: ${selected.title}`, 'success');
        fetchStatus();
      }
    });
  }

  // Select Work App from Dropdown
  if (selectAppsEl) {
    selectAppsEl.addEventListener('change', async () => {
      const hwnd = parseInt(selectAppsEl.value, 10);
      if (!hwnd) return;
      const selected = openAppsList.find(w => w.hwnd === hwnd);
      if (selected) {
        audio.playClick();
        await fetch('/api/select-work', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ hwnd: selected.hwnd, title: selected.title })
        });
        logChat('Target', `Selected Work Window: ${selected.title}`, 'success');
        fetchStatus();
      }
    });
  }

  // Pick by switching button
  const btnPick = document.getElementById('btn-pick-by-switch');
  if (btnPick) btnPick.addEventListener('click', pickBySwitching);

  // Toggles
  if (toggleGamingEl) toggleGamingEl.addEventListener('change', updateModes);
  if (toggleAutoSwitchEl) toggleAutoSwitchEl.addEventListener('change', updateModes);
  if (toggleFullscreenGuardEl) toggleFullscreenGuardEl.addEventListener('change', updateModes);
  if (toggleMeetingGuardEl) toggleMeetingGuardEl.addEventListener('change', updateModes);
  if (toggleMediaControlEl) toggleMediaControlEl.addEventListener('change', updateModes);
  if (toggleMobilePrimaryDesktopEl) toggleMobilePrimaryDesktopEl.addEventListener('change', updateModes);

  // Test Buttons
  const btnTestStep1 = document.getElementById('btn-test-step1');
  if (btnTestStep1) {
    btnTestStep1.addEventListener('click', async () => {
      audio.playWarp();
      logChat('Test', 'Testing Step 1: Switching to Work & resuming media...', 'info');
      try {
        const res = await fetch('/api/test-work', { method: 'POST' });
        const data = await res.json();
        logChat('Test', data.message, data.success ? 'success' : 'error');
      } catch (e) {}
    });
  }

  const btnTestStep2 = document.getElementById('btn-test-step2');
  if (btnTestStep2) {
    btnTestStep2.addEventListener('click', async () => {
      audio.playWarp();
      logChat('Test', 'Testing Step 2: Pausing media & switching to Agent...', 'info');
      try {
        const res = await fetch('/api/test-agent', { method: 'POST' });
        const data = await res.json();
        logChat('Test', data.message, data.success ? 'success' : 'error');
      } catch (e) {}
    });
  }

  const btnTestLoop = document.getElementById('btn-test-loop');
  if (btnTestLoop) {
    btnTestLoop.addEventListener('click', async () => {
      audio.playWarp();
      logChat('Test', 'Triggering complete simulated focus loop (Work -> Video -> Agent)...', 'info');
      try {
        const res = await fetch('/api/test-focus', { method: 'POST' });
        const data = await res.json();
        logChat('Test', data.message, 'success');
      } catch (e) {}
    });
  }

  const btnTestMedia = document.getElementById('btn-test-media');
  if (btnTestMedia) {
    btnTestMedia.addEventListener('click', async () => {
      audio.playClick();
      try {
        const res = await fetch('/api/toggle-media', { method: 'POST' });
        const data = await res.json();
        logChat('Media', data.message, 'success');
      } catch (e) {}
    });
  }

  // Re-install hooks
  const btnReinstall = document.getElementById('btn-reinstall-hooks');
  if (btnReinstall) {
    btnReinstall.addEventListener('click', async () => {
      audio.playClick();
      try {
        const res = await fetch('/api/install', { method: 'POST' });
        const data = await res.json();
        audio.playFanfare();
        logChat('Hook', data.message || 'Hooks installed successfully!', 'success');
        fetchStatus();
      } catch (e) {}
    });
  }

  // Remove / Uninstall hooks
  const btnUninstall = document.getElementById('btn-uninstall-hooks');
  if (btnUninstall) {
    btnUninstall.addEventListener('click', async () => {
      audio.playClick();
      try {
        const res = await fetch('/api/uninstall-hooks', { method: 'POST' });
        const data = await res.json();
        logChat('Hook', data.message || 'Hooks removed successfully!', 'warn');
        fetchStatus();
      } catch (e) {}
    });
  }

  // Clear logs
  const btnClearLogs = document.getElementById('btn-clear-logs');
  if (btnClearLogs) {
    btnClearLogs.addEventListener('click', () => {
      audio.playClick();
      chatLogsEl.innerHTML = '';
    });
  }

  const btnRefreshLogs = document.getElementById('btn-refresh-logs');
  if (btnRefreshLogs) {
    btnRefreshLogs.addEventListener('click', () => {
      audio.playClick();
      fetchLogs();
    });
  }

  // Mobile Controller Pairing Events
  const mobileUrlDisplay = document.getElementById('mobile-url-display');
  const qrCodeImg = document.getElementById('qr-code-img');
  const btnCopyMobileUrl = document.getElementById('btn-copy-mobile-url');
  const btnOpenMobileTab = document.getElementById('btn-open-mobile-tab');
  const btnTestMobileAlert = document.getElementById('btn-test-mobile-alert');
  let currentMobileURL = '';

  async function fetchMobileInfo() {
    try {
      const res = await fetch('/api/mobile/info');
      if (!res.ok) return;
      const data = await res.json();
      currentMobileURL = data.mobile_url;

      if (mobileUrlDisplay) mobileUrlDisplay.textContent = data.mobile_url;
      if (qrCodeImg) {
        qrCodeImg.src = `https://api.qrserver.com/v1/create-qr-code/?size=140x140&margin=2&data=${encodeURIComponent(data.mobile_url)}`;
      }

      const mobileModeBadge = document.getElementById('mobile-mode-badge');
      if (mobileModeBadge) {
        if (data.mobile_control_active) {
          mobileModeBadge.textContent = '📱 MOBILE PRIMARY ACTIVE';
          mobileModeBadge.style.background = 'var(--pixel-diamond)';
        } else {
          mobileModeBadge.textContent = 'PHONE LINK READY';
          mobileModeBadge.style.background = 'var(--pixel-emerald)';
        }
      }
    } catch (e) {}
  }

  if (btnCopyMobileUrl) {
    btnCopyMobileUrl.addEventListener('click', () => {
      audio.playClick();
      if (currentMobileURL) {
        navigator.clipboard.writeText(currentMobileURL);
        logChat('Mobile', `Copied link to clipboard: ${currentMobileURL}`, 'success');
        audio.playFanfare();
      }
    });
  }

  if (btnOpenMobileTab) {
    btnOpenMobileTab.addEventListener('click', () => {
      audio.playClick();
      window.open('/mobile.html', '_blank', 'width=420,height=750');
    });
  }

  if (btnTestMobileAlert) {
    btnTestMobileAlert.addEventListener('click', async () => {
      audio.playWarp();
      logChat('Mobile', 'Triggering test approval notification to mobile phone...', 'info');
      try {
        await fetch('/api/mobile/reply', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id: 'test_clear' }) });
        await fetch('/api/mobile/prompt', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ prompt: 'Simulated prompt received from Mobile Controller.' }) });
        logChat('Mobile', 'Sent test approval & prompt notification! Check phone screen.', 'success');
      } catch (e) {}
    });
  }

  // Exit / End App Button
  const btnExitApp = document.getElementById('btn-exit-app');
  if (btnExitApp) {
    btnExitApp.addEventListener('click', async () => {
      audio.playClick();
      if (confirm('Stop SwitchBack completely? The background process will terminate.')) {
        logChat('System', 'Shutting down SwitchBack process...', 'warn');
        try {
          await fetch('/api/shutdown', { method: 'POST' });
        } catch (e) {}
        document.body.innerHTML = `
          <div style="display:flex;flex-direction:column;align-items:center;justify-content:center;height:100vh;background:#0f111a;color:#fff;font-family:'Press Start 2P', monospace;text-align:center;padding:24px;">
            <div style="font-size:40px;margin-bottom:20px;">⏹</div>
            <h1 style="font-size:16px;color:#f9ca24;margin-bottom:14px;letter-spacing:1px;">SWITCHBACK STOPPED</h1>
            <p style="font-size:11px;color:#8890a6;line-height:1.6;font-family:sans-serif;">The background process has exited cleanly.<br>You can now safely close this browser window.</p>
          </div>
        `;
        setTimeout(() => {
          window.close();
        }, 800);
      }
    });
  }

  // Client Session Heartbeat & Auto-disconnect on window close
  const clientId = 'tab_' + Math.random().toString(36).substr(2, 9);
  function sendHeartbeat() {
    fetch('/api/heartbeat?client=' + clientId, { method: 'POST' }).catch(() => {});
  }
  sendHeartbeat();
  setInterval(sendHeartbeat, 3000);

  window.addEventListener('beforeunload', () => {
    if (navigator.sendBeacon) {
      navigator.sendBeacon('/api/disconnect?client=' + clientId);
    }
  });

  setupPresets();
  fetchMobileInfo();
}

document.addEventListener('DOMContentLoaded', () => {
  setupEvents();
  fetchWindows();
  fetchStatus();
  fetchLogs();

  setInterval(fetchStatus, 2000);
  setInterval(fetchLogs, 3000);
});
