// ==========================================================================
// RETRO COLOURFUL PIXEL LOGIC & CONTROLLER FOR FOCUSMGR v1.1.0
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

    gain.gain.setValueAtTime(0.3, this.ctx.currentTime);
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

    gain.gain.setValueAtTime(0.4, this.ctx.currentTime);
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
      gain.gain.setValueAtTime(0.2, this.ctx.currentTime + idx * 0.08);
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
const toggleGamingEl = document.getElementById('toggle-gaming-mode');
const toggleAutoSwitchEl = document.getElementById('toggle-auto-switch');
const statusBadgeEl = document.getElementById('system-status-badge');
const chatLogsEl = document.getElementById('chat-logs');
const countdownBanner = document.getElementById('countdown-banner');
const countdownText = document.getElementById('countdown-text');

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
      selectAgentsEl.innerHTML = '<option value="">No agents found (launch Antigravity/Claude)</option>';
    } else {
      selectAgentsEl.innerHTML = '<option value="">-- Choose an AI Agent Window --</option>';
      detectedAgentsList.forEach(a => {
        const opt = document.createElement('option');
        opt.value = a.hwnd;
        opt.textContent = `[${a.agent_type.toUpperCase()}] ${a.title} (${a.process_name})`;
        selectAgentsEl.appendChild(opt);
      });
    }

    // Populate Open Apps Dropdown
    selectAppsEl.innerHTML = '<option value="">-- Or choose from running windows --</option>';
    openAppsList.forEach(w => {
      const opt = document.createElement('option');
      opt.value = w.hwnd;
      opt.textContent = `${w.title} (${w.process_name})`;
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

    selectedAgentNameEl.textContent = data.selected_agent_title || 'None Selected (Auto-resolve)';
    selectedAgentHwndEl.textContent = data.selected_agent_hwnd || '0';

    selectedWorkNameEl.textContent = data.selected_work_title || 'None Selected (Click Pick)';
    selectedWorkHwndEl.textContent = data.selected_work_hwnd || '0';

    toggleGamingEl.checked = !!data.gaming_mode;
    toggleAutoSwitchEl.checked = !!data.auto_switch_enabled;

    // Status badge
    if (data.current_status === 'agent_working') {
      statusBadgeEl.textContent = '⚡ AGENT WORKING';
      statusBadgeEl.style.background = 'var(--pixel-diamond)';
    } else if (data.current_status === 'permission_needed') {
      statusBadgeEl.textContent = '🔔 NEEDS PERMISSION';
      statusBadgeEl.style.background = 'var(--pixel-gold)';
    } else {
      statusBadgeEl.textContent = '● IDLE / READY';
      statusBadgeEl.style.background = 'var(--pixel-emerald)';
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
        else if (l.includes('[GamingMode]')) type = 'warn';
        else if (l.includes('[AutoSwitch]')) type = 'success';
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

  // Send request immediately to let server sleep 3.2s and capture
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
  const gaming = toggleGamingEl.checked;
  const autoSwitch = toggleAutoSwitchEl.checked;

  try {
    await fetch('/api/toggle-mode', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ gaming_mode: gaming, auto_switch_enabled: autoSwitch })
    });

    if (gaming) {
      logChat('Mode', '🎮 Gaming Mode ENABLED: Notifications only for permissions (No focus stealing).', 'warn');
    } else {
      logChat('Mode', '⚡ Gaming Mode DISABLED: Focus will automatically switch to agent on permissions.', 'info');
    }
    fetchStatus();
  } catch (e) {}
}

// Event Listeners
function setupEvents() {
  // Sound
  const soundBtn = document.getElementById('btn-sound-toggle');
  const soundStatus = document.getElementById('sound-status');
  const soundIcon = document.getElementById('sound-icon');
  soundBtn.addEventListener('click', () => {
    audio.enabled = !audio.enabled;
    soundStatus.textContent = audio.enabled ? 'ON' : 'OFF';
    soundIcon.textContent = audio.enabled ? '🔊' : '🔇';
    if (audio.enabled) audio.playClick();
  });

  // Refresh Agents
  document.getElementById('btn-refresh-agents').addEventListener('click', () => {
    audio.playClick();
    logChat('Radar', 'Scanning running processes for AI agents...', 'info');
    fetchWindows();
  });

  // Select Agent from Dropdown
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

  // Select Work App from Dropdown
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

  // Pick by switching button
  document.getElementById('btn-pick-by-switch').addEventListener('click', pickBySwitching);

  // Toggles
  toggleGamingEl.addEventListener('change', updateModes);
  toggleAutoSwitchEl.addEventListener('change', updateModes);

  // Test loop
  document.getElementById('btn-test-loop').addEventListener('click', async () => {
    audio.playWarp();
    logChat('Test', 'Triggering complete focus loop test...', 'info');
    try {
      const res = await fetch('/api/test-focus', { method: 'POST' });
      const data = await res.json();
      logChat('Test', data.message, 'success');
    } catch (e) {}
  });

  // Re-install hooks
  document.getElementById('btn-reinstall-hooks').addEventListener('click', async () => {
    audio.playClick();
    try {
      const res = await fetch('/api/install', { method: 'POST' });
      const data = await res.json();
      audio.playFanfare();
      logChat('Hook', data.message || 'Hooks installed successfully!', 'success');
    } catch (e) {}
  });

  // Clear logs
  document.getElementById('btn-clear-logs').addEventListener('click', () => {
    audio.playClick();
    chatLogsEl.innerHTML = '';
  });

  document.getElementById('btn-refresh-logs').addEventListener('click', () => {
    audio.playClick();
    fetchLogs();
  });
}

document.addEventListener('DOMContentLoaded', () => {
  setupEvents();
  fetchWindows();
  fetchStatus();
  fetchLogs();

  setInterval(fetchStatus, 2000);
  setInterval(fetchLogs, 3000);
});
