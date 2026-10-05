// ==========================================================================
// SWITCHBACK v1.2.0 - MODULAR CLEAN CONTROLLER
// ==========================================================================

class PixelAudio {
  constructor() {
    this.ctx = null;
    this.enabled = localStorage.getItem('switchback_sound_enabled') === 'true';
  }

  init() {
    if (!this.ctx) {
      const AudioCtx = window.AudioContext || window.webkitAudioContext;
      this.ctx = new AudioCtx();
    }
  }

  toggleSound() {
    this.enabled = !this.enabled;
    localStorage.setItem('switchback_sound_enabled', this.enabled ? 'true' : 'false');
    return this.enabled;
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

  playAlert() {
    if (!this.enabled) return;
    this.init();
    const now = this.ctx.currentTime;
    // Pleasant dual chime: 880Hz (A5) -> 1174Hz (D6)
    const tones = [
      { freq: 880, start: 0, dur: 0.1 },
      { freq: 1174, start: 0.1, dur: 0.22 }
    ];
    tones.forEach(t => {
      const osc = this.ctx.createOscillator();
      const gain = this.ctx.createGain();
      osc.type = 'triangle';
      osc.frequency.setValueAtTime(t.freq, now + t.start);
      gain.gain.setValueAtTime(0.2, now + t.start);
      gain.gain.exponentialRampToValueAtTime(0.001, now + t.start + t.dur);
      osc.connect(gain);
      gain.connect(this.ctx.destination);
      osc.start(now + t.start);
      osc.stop(now + t.start + t.dur);
    });
  }

  playSubmit() {
    if (!this.enabled) return;
    this.init();
    const osc = this.ctx.createOscillator();
    const gain = this.ctx.createGain();
    osc.type = 'square';
    osc.frequency.setValueAtTime(587, this.ctx.currentTime);
    osc.frequency.exponentialRampToValueAtTime(880, this.ctx.currentTime + 0.08);
    gain.gain.setValueAtTime(0.15, this.ctx.currentTime);
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
const toggleAutoApproveEl = document.getElementById('toggle-auto-approve');
const toggleKeepInTrayEl = document.getElementById('toggle-keep-in-tray');

// Productivity HUD Elements
const hudTimeSavedEl = document.getElementById('hud-time-saved');
const hudTasksCountEl = document.getElementById('hud-tasks-count');
const hudApprovalsCountEl = document.getElementById('hud-approvals-count');

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
let lastKnownStatus = '';
let lastPendingApprovalId = null;

function formatDuration(sec) {
  sec = Math.max(0, Math.floor(sec || 0));
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  if (m > 0) {
    return `${m}m ${s < 10 ? '0' : ''}${s}s`;
  }
  return `${s}s`;
}

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

  } catch (e) {
    console.warn('fetchWindows error:', e);
  }
}

let consecutiveStatusErrors = 0;

// Fetch active status
async function fetchStatus() {
  try {
    const res = await fetch('/api/status');
    if (!res.ok) throw new Error('HTTP ' + res.status);
    const data = await res.json();
    consecutiveStatusErrors = 0;

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
    if (toggleAutoApproveEl) toggleAutoApproveEl.checked = !!data.auto_approve_safe;
    if (toggleKeepInTrayEl) toggleKeepInTrayEl.checked = !!data.keep_in_tray;

    // HUD Stats
    if (hudTimeSavedEl) hudTimeSavedEl.textContent = formatDuration(data.total_time_saved_secs);
    if (hudTasksCountEl) hudTasksCountEl.textContent = data.total_tasks_completed || 0;
    if (hudApprovalsCountEl) hudApprovalsCountEl.textContent = data.total_approvals_handled || 0;

    // Audio chime only on true state completion if enabled
    const currentApprovalId = data.pending_approval ? data.pending_approval.id : null;
    if (currentApprovalId && currentApprovalId !== lastPendingApprovalId) {
      lastPendingApprovalId = currentApprovalId;
      if (audio.enabled) audio.playAlert();
    } else if (!currentApprovalId) {
      lastPendingApprovalId = null;
    }

    if (data.current_status && data.current_status !== lastKnownStatus) {
      if (data.current_status === 'completed') {
        if (audio.enabled) audio.playFanfare();
      }
      lastKnownStatus = data.current_status;
    }

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
        statusBadgeEl.style.background = '';
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
        } else if (data.current_status === 'completed') {
          statusBadgeEl.textContent = '✓ TASK COMPLETED';
          statusBadgeEl.style.background = 'var(--pixel-emerald)';
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
        hooksStatusBadge.className = 'badge-green';
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

  } catch (e) {
    consecutiveStatusErrors++;
    if (consecutiveStatusErrors >= 4 && statusBadgeEl) {
      statusBadgeEl.textContent = '⚡ SERVER DISCONNECTED';
      statusBadgeEl.style.background = 'var(--pixel-red)';
      statusBadgeEl.classList.add('badge-stopped');
    }
    console.warn('fetchStatus error:', e);
  }
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
  } catch (e) {
    console.warn('fetchLogs error:', e);
  }
}

// Pick Work Window by fast switching
async function pickBySwitching() {
  const btnPick = document.getElementById('btn-pick-by-switch');
  if (btnPick) btnPick.disabled = true;

  audio.playClick();
  countdownBanner.classList.remove('hidden');

  let seconds = 3;
  countdownText.textContent = `SWITCH TO WINDOW IN ${seconds}...`;
  audio.playBeep(440);

  // Run the 3-second countdown visually so the user has time to focus their work window
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

  // Wait 3.1 seconds for user to switch focus
  await new Promise(resolve => setTimeout(resolve, 3100));

  try {
    // Immediate capture of foreground window after countdown
    const res = await fetch('/api/pick-work-capture', { method: 'POST' }).then(r => r.json());
    countdownBanner.classList.add('hidden');

    if (res.success) {
      audio.playFanfare();
      logChat('Target', `Locked in Work Window: ${res.title} (HWND: ${res.hwnd})`, 'success');
      fetchStatus();
    } else {
      logChat('Target', res.message || 'Failed to capture window.', 'error');
    }
  } catch (e) {
    countdownBanner.classList.add('hidden');
    logChat('Target', `Failed to capture window: ${e.message}`, 'error');
  } finally {
    clearInterval(timer);
    if (btnPick) btnPick.disabled = false;
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
        mobile_control_active: mobilePrimary
      })
    });

    logChat('Modes', `Updated (AutoSwitch: ${autoSwitch}, MediaPause: ${media}, MobileSuppress: ${mobilePrimary})`, 'info');
    fetchStatus();
  } catch (e) {
    logChat('Modes', 'Failed to update modes: ' + e.message, 'error');
  }
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
  } catch (e) {
    logChat('Master', 'Failed to toggle pause: ' + e.message, 'error');
  }
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
    if (soundStatus) soundStatus.textContent = audio.enabled ? 'ON' : 'OFF';
    if (soundIcon) soundIcon.textContent = audio.enabled ? '🔊' : '🔇';

    soundBtn.addEventListener('click', () => {
      const isEnabled = audio.toggleSound();
      if (soundStatus) soundStatus.textContent = isEnabled ? 'ON' : 'OFF';
      if (soundIcon) soundIcon.textContent = isEnabled ? '🔊' : '🔇';
      if (isEnabled) audio.playClick();
      logChat('Audio', isEnabled ? 'Sound chimes enabled.' : 'Sound chimes muted.', 'info');
    });
  }

  // Preview Chime Button
  const btnTestChime = document.getElementById('btn-test-chime');
  if (btnTestChime) {
    btnTestChime.addEventListener('click', () => {
      audio.playAlert();
      logChat('Audio', 'Previewing 8-bit permission chime!', 'info');
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

  // Safe Auto-Approve Toggle
  if (toggleAutoApproveEl) {
    toggleAutoApproveEl.addEventListener('change', async () => {
      audio.playClick();
      try {
        const res = await fetch('/api/toggle-auto-approve', { method: 'POST' });
        const data = await res.json();
        logChat('SafeAuto', data.auto_approve_safe ? '🛡️ Auto-Approve Safe Actions ENABLED.' : 'Auto-Approve Safe Actions DISABLED.', 'info');
        fetchStatus();
      } catch (e) {
        logChat('SafeAuto', 'Error updating auto-approve: ' + e.message, 'error');
      }
    });
  }

  // System Tray Toggle
  if (toggleKeepInTrayEl) {
    toggleKeepInTrayEl.addEventListener('change', async () => {
      audio.playClick();
      try {
        const res = await fetch('/api/toggle-tray', { method: 'POST' });
        const data = await res.json();
        logChat('Tray', data.keep_in_tray ? '📥 Run in System Tray ENABLED. SwitchBack stays alive when tabs close.' : 'Run in System Tray DISABLED.', 'info');
        fetchStatus();
      } catch (e) {
        logChat('Tray', 'Error updating tray setting: ' + e.message, 'error');
      }
    });
  }

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
      } catch (e) {
        logChat('Test', 'Test Step 1 failed: ' + e.message, 'error');
      }
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
      } catch (e) {
        logChat('Test', 'Test Step 2 failed: ' + e.message, 'error');
      }
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
      } catch (e) {
        logChat('Test', 'Simulation failed: ' + e.message, 'error');
      }
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
      } catch (e) {
        logChat('Media', 'Media toggle failed: ' + e.message, 'error');
      }
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
      } catch (e) {
        logChat('Hook', 'Install failed: ' + e.message, 'error');
      }
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
      } catch (e) {
        logChat('Hook', 'Uninstall failed: ' + e.message, 'error');
      }
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
  const clientId = 'tab_' + Math.random().toString(36).substring(2, 11);
  function sendHeartbeat() {
    fetch('/api/heartbeat?client=' + clientId, { method: 'POST' }).catch(() => {});
  }
  sendHeartbeat();

  // Send heartbeat immediately whenever user refocuses tab
  document.addEventListener('visibilitychange', () => {
    if (!document.hidden) {
      sendHeartbeat();
    }
  });

  // Use a dedicated Web Worker timer so heartbeats aren't throttled when the tab is in background
  try {
    const workerBlob = new Blob([`setInterval(() => postMessage('tick'), 4000);`], { type: 'application/javascript' });
    const worker = new Worker(URL.createObjectURL(workerBlob));
    worker.onmessage = () => sendHeartbeat();
  } catch (e) {
    setInterval(sendHeartbeat, 4000);
  }

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

  setInterval(fetchStatus, 400);
  setInterval(fetchLogs, 2000);
  setInterval(fetchWindows, 8000);
});
