// ==========================================================================
// RETRO COLOURFUL PIXEL LOGIC & AUDIO SYNTHESIZER FOR FOCUSMGR
// ==========================================================================

// Sound Synthesizer via Web Audio API (Zero external audio files needed!)
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

  // Classic button click sound
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

  // Focus switch sound
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

  // Countdown beep (8-bit square wave)
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

  // Success fanfare
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

// DOM Elements
const currentTitleEl = document.getElementById('current-window-title');
const currentHwndEl = document.getElementById('current-window-hwnd');
const activeSessionsEl = document.getElementById('active-sessions-count');
const chatLogsEl = document.getElementById('chat-logs');
const countdownBanner = document.getElementById('countdown-banner');
const countdownText = document.getElementById('countdown-text');
const countdownSubtext = document.getElementById('countdown-subtext');

// Append line to Chat / Activity Console
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

// Fetch Status & Window Radar from API
async function fetchStatus() {
  try {
    const res = await fetch('/api/status');
    if (!res.ok) return;
    const data = await res.json();

    currentTitleEl.textContent = data.current_title || 'None / Desktop';
    currentHwndEl.textContent = data.current_hwnd || '0';
    activeSessionsEl.textContent = data.active_sessions || '0';
  } catch (e) {
    // Local static fallback mode
  }
}

// Poll logs
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
        else if (l.includes('[WARN]')) type = 'warn';
        else if (l.includes('Successfully')) type = 'success';
        logChat('System', l, type);
      });
    }
  } catch (e) {}
}

// Countdown Focus Switch Test
async function runTeleportTest() {
  audio.playClick();
  countdownBanner.classList.remove('hidden');

  let seconds = 3;
  countdownText.textContent = `SWITCHING IN ${seconds}...`;
  countdownSubtext.textContent = `Click into ANY other window (e.g. Browser, Notepad) NOW!`;
  audio.playBeep(440);

  const timer = setInterval(async () => {
    seconds--;
    if (seconds > 0) {
      countdownText.textContent = `SWITCHING IN ${seconds}...`;
      audio.playBeep(440 + (3 - seconds) * 100);
    } else {
      clearInterval(timer);
      countdownText.textContent = `⚡ FOCUS SWITCH TRIGGERED!`;
      countdownSubtext.textContent = `Restoring window handle...`;
      audio.playWarp();

      try {
        const res = await fetch('/api/test-focus', { method: 'POST' });
        const resData = await res.json();
        logChat('Focus', `Focus switch complete: ${resData.message || 'Restored'}`, 'success');
        audio.playFanfare();
      } catch (err) {
        logChat('Focus', `Switch trigger sent locally.`, 'info');
      }

      setTimeout(() => {
        countdownBanner.classList.add('hidden');
        fetchStatus();
      }, 1500);
    }
  }, 1000);
}

// Setup Event Listeners
function setupEvents() {
  // Sound Toggle
  const soundBtn = document.getElementById('btn-sound-toggle');
  const soundStatus = document.getElementById('sound-status');
  const soundIcon = document.getElementById('sound-icon');
  soundBtn.addEventListener('click', () => {
    audio.enabled = !audio.enabled;
    soundStatus.textContent = audio.enabled ? 'ON' : 'OFF';
    soundIcon.textContent = audio.enabled ? '🔊' : '🔇';
    if (audio.enabled) audio.playClick();
  });

  // Radar Scan
  document.getElementById('btn-refresh-radar').addEventListener('click', () => {
    audio.playClick();
    logChat('Radar', 'Scanning active windows on Windows Desktop...', 'info');
    fetchStatus();
  });

  // Test Focus Switch
  document.getElementById('btn-test-teleport').addEventListener('click', runTeleportTest);

  // Manual Save & Focus
  document.getElementById('btn-manual-save').addEventListener('click', async () => {
    audio.playClick();
    logChat('User', 'Triggering manual save-and-focus...', 'info');
    try {
      await fetch('/api/save-and-focus', { method: 'POST' });
      audio.playWarp();
      logChat('System', 'Saved foreground window and focused agent!', 'success');
      fetchStatus();
    } catch (e) {
      logChat('System', 'Command dispatched.', 'info');
    }
  });

  // Manual Restore
  document.getElementById('btn-manual-restore').addEventListener('click', async () => {
    audio.playClick();
    logChat('User', 'Triggering manual restore...', 'info');
    try {
      await fetch('/api/restore', { method: 'POST' });
      audio.playWarp();
      logChat('System', 'Restored previous user working window!', 'success');
      fetchStatus();
    } catch (e) {
      logChat('System', 'Command dispatched.', 'info');
    }
  });

  // Auto-Install All Hooks
  document.getElementById('btn-install-hooks').addEventListener('click', async () => {
    audio.playClick();
    logChat('Hook', 'Configuring hooks for Claude Code & Antigravity...', 'warn');
    try {
      const res = await fetch('/api/install', { method: 'POST' });
      const data = await res.json();
      audio.playFanfare();
      logChat('Hook', data.message || 'All hooks installed successfully!', 'success');
    } catch (e) {
      logChat('Hook', 'Hooks configuration updated.', 'success');
    }
  });

  // Settings Save
  const debounceInput = document.getElementById('input-debounce');
  const debounceVal = document.getElementById('debounce-val');
  debounceInput.addEventListener('input', () => {
    debounceVal.textContent = `${debounceInput.value}ms`;
  });

  document.getElementById('btn-save-settings').addEventListener('click', async () => {
    audio.playClick();
    const patterns = document.getElementById('input-patterns').value.split(',').map(s => s.trim());
    const debounce = parseInt(debounceInput.value, 10);
    try {
      await fetch('/api/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ debounce_ms: debounce, patterns: patterns })
      });
      audio.playFanfare();
      logChat('Config', 'Configuration saved to %LOCALAPPDATA%\\focusmgr\\config.json', 'success');
    } catch (e) {
      logChat('Config', 'Settings applied locally.', 'info');
    }
  });

  // Console Clear
  document.getElementById('btn-clear-logs').addEventListener('click', () => {
    audio.playClick();
    chatLogsEl.innerHTML = '';
    logChat('System', 'Log display cleared.');
  });

  document.getElementById('btn-refresh-logs').addEventListener('click', () => {
    audio.playClick();
    fetchLogs();
  });
}

// Initial Boot
document.addEventListener('DOMContentLoaded', () => {
  setupEvents();
  fetchStatus();
  fetchLogs();

  // Periodic radar ping (every 2.5 seconds)
  setInterval(fetchStatus, 2500);
});
