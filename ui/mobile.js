// ==========================================================================
// SWITCHBACK MOBILE REMOTE CONTROLLER SCRIPT
// ==========================================================================

class MobileFeedback {
  constructor() {
    this.hapticsEnabled = true;
    this.soundEnabled = true;
    this.ctx = null;
  }

  initAudio() {
    if (!this.ctx) {
      const AudioCtx = window.AudioContext || window.webkitAudioContext;
      this.ctx = new AudioCtx();
    }
  }

  vibrate(pattern = [50]) {
    if (this.hapticsEnabled && 'vibrate' in navigator) {
      try {
        navigator.vibrate(pattern);
      } catch (e) {}
    }
  }

  playBeep(freq = 600, duration = 0.08) {
    if (!this.soundEnabled) return;
    this.initAudio();
    try {
      const osc = this.ctx.createOscillator();
      const gain = this.ctx.createGain();
      osc.type = 'square';
      osc.frequency.setValueAtTime(freq, this.ctx.currentTime);
      gain.gain.setValueAtTime(0.15, this.ctx.currentTime);
      gain.gain.linearRampToValueAtTime(0.01, this.ctx.currentTime + duration);
      osc.connect(gain);
      gain.connect(this.ctx.destination);
      osc.start();
      osc.stop(this.ctx.currentTime + duration);
    } catch (e) {}
  }

  alertNotification() {
    this.vibrate([150, 100, 200, 100, 250]);
    this.playBeep(880, 0.15);
  }

  success() {
    this.vibrate([40, 60, 80]);
    this.playBeep(650, 0.1);
  }
}

const feedback = new MobileFeedback();

// DOM Elements
const statusBadge = document.getElementById('mobile-status-badge');
const togglePrimary = document.getElementById('toggle-mobile-primary');
const agentNameEl = document.getElementById('m-agent-name');
const workNameEl = document.getElementById('m-work-name');
const btnMedia = document.getElementById('btn-mobile-media');

const approvalSection = document.getElementById('m-approval-section');
const approvalTitle = document.getElementById('m-approval-title');
const approvalDesc = document.getElementById('m-approval-desc');
const approvalOptions = document.getElementById('m-approval-options');

const promptInput = document.getElementById('m-prompt-input');
const btnSendPrompt = document.getElementById('btn-send-prompt');
const logsBox = document.getElementById('m-logs');

let lastSeenApprovalId = null;

// Append log helper
function addLog(msg, type = 'info') {
  const line = document.createElement('div');
  line.className = `m-log-line ${type}`;
  line.innerHTML = `&lt;${new Date().toLocaleTimeString()}&gt; ${escapeHtml(msg)}`;
  logsBox.appendChild(line);
  logsBox.scrollTop = logsBox.scrollHeight;
}

function escapeHtml(str) {
  return (str || '').replace(/[&<>'"]/g, tag => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    "'": '&#39;',
    '"': '&quot;'
  }[tag] || tag));
}

// Fetch Mobile Live Status
async function fetchMobileStatus() {
  try {
    const res = await fetch('/api/mobile/status');
    if (!res.ok) return;
    const data = await res.json();

    // Update status badge
    if (data.current_status === 'agent_working') {
      statusBadge.textContent = '⚡ THINKING';
      statusBadge.style.background = 'var(--pixel-diamond)';
    } else if (data.current_status === 'permission_needed') {
      statusBadge.textContent = '🔔 NEEDS INPUT';
      statusBadge.style.background = 'var(--pixel-gold)';
    } else if (data.current_status === 'completed') {
      statusBadge.textContent = '✔ DONE';
      statusBadge.style.background = 'var(--pixel-emerald)';
    } else {
      statusBadge.textContent = '● IDLE';
      statusBadge.style.background = 'var(--pixel-emerald)';
    }

    // Update Target / Agent names
    agentNameEl.textContent = data.agent_title || 'Auto-Detect Active';
    workNameEl.textContent = data.work_title || 'None Selected';

    // Update Primary Switch
    if (togglePrimary) {
      togglePrimary.checked = !!data.mobile_control_active;
    }

    // Handle Active Approval / Question
    if (data.pending_approval) {
      const app = data.pending_approval;
      approvalSection.style.display = 'flex';
      approvalTitle.textContent = app.title || 'Action Approval Needed';
      approvalDesc.textContent = app.message || 'Agent requested confirmation.';

      // Render options
      if (app.id !== lastSeenApprovalId) {
        lastSeenApprovalId = app.id;
        feedback.alertNotification();
        renderApprovalButtons(app);
      }
    } else {
      approvalSection.style.display = 'none';
      lastSeenApprovalId = null;
    }

  } catch (e) {}
}

function renderApprovalButtons(app) {
  approvalOptions.innerHTML = '';

  if (Array.isArray(app.options) && app.options.length > 0) {
    app.options.forEach((optText, index) => {
      const btn = document.createElement('button');
      // For binary allow/deny
      const lower = optText.toLowerCase();
      const isDeny = lower.includes('deny') || lower.includes('cancel') || lower.includes('no');
      btn.className = isDeny ? 'm-btn m-btn-deny w-100' : 'm-btn m-btn-gold w-100';
      btn.style.marginBottom = '8px';
      btn.innerHTML = `<span style="opacity:0.7;margin-right:8px;">[ ${index + 1} ]</span> ${escapeHtml(optText)}`;
      btn.addEventListener('click', () => {
        sendApprovalReply(app.id, isDeny ? 'deny' : 'option', optText, index + 1);
      });
      approvalOptions.appendChild(btn);
    });
  } else {
    // Default Approve / Deny
    const btnApprove = document.createElement('button');
    btnApprove.className = 'm-btn m-btn-approve w-100';
    btnApprove.textContent = '✔ APPROVE / ALLOW';
    btnApprove.addEventListener('click', () => sendApprovalReply(app.id, 'allow', 'Allowed by user', 1));

    const btnDeny = document.createElement('button');
    btnDeny.className = 'm-btn m-btn-deny w-100';
    btnDeny.textContent = '✖ DENY / CANCEL';
    btnDeny.addEventListener('click', () => sendApprovalReply(app.id, 'deny', 'Denied by user', 0));

    approvalOptions.appendChild(btnApprove);
    approvalOptions.appendChild(btnDeny);
  }
}

// Send Approval Reply
async function sendApprovalReply(id, decision, answer, index = 0) {
  feedback.success();
  addLog(`Replied: Option ${index > 0 ? index : ''} - ${escapeHtml(answer)}`, decision === 'deny' ? 'warn' : 'success');
  approvalSection.style.display = 'none';

  try {
    await fetch('/api/mobile/reply', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id, decision, answer, index })
    });
    fetchMobileStatus();
    fetchAgentOutput();
  } catch (e) {}
}

// Send Prompt to Agent
async function sendPrompt() {
  const prompt = (promptInput.value || '').trim();
  if (!prompt) return;

  feedback.success();
  addLog(`Prompt sent: "${prompt}"`, 'success');
  promptInput.value = '';

  try {
    const res = await fetch('/api/mobile/prompt', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ prompt })
    });
    if (res.ok) {
      const data = await res.json();
      addLog(`Agent: ${data.message || 'Prompt received by Agent'}`, 'success');
    } else {
      addLog(`Failed to submit: HTTP ${res.status}`, 'error');
    }
    fetchMobileStatus();
    fetchAgentOutput();
  } catch (e) {
    addLog(`Delivery failed: ${e.message}`, 'error');
  }
}

// Toggle PC Media Play/Pause
async function toggleMedia() {
  feedback.vibrate([40]);
  feedback.playBeep(440, 0.05);
  addLog('Toggled PC Media Playback', 'info');

  try {
    await fetch('/api/mobile/media', { method: 'POST' });
  } catch (e) {}
}

// Toggle Mobile Primary Mode
async function togglePrimaryMode() {
  feedback.vibrate([50]);
  const active = togglePrimary.checked;
  addLog(`Mobile Primary Mode: ${active ? 'ACTIVE (PC focus suppressed)' : 'DISABLED'}`, active ? 'success' : 'warn');

  try {
    await fetch('/api/mobile/toggle', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ active })
    });
  } catch (e) {}
}

// Run Autonomous Dev Task
async function runDevCommand(command) {
  const cmd = (command || (document.getElementById('m-cmd-input') ? document.getElementById('m-cmd-input').value : '')).trim();
  if (!cmd) return;

  feedback.success();
  addLog(`Running dev task: "${cmd}"`, 'info');
  const inputEl = document.getElementById('m-cmd-input');
  if (inputEl) inputEl.value = '';

  try {
    const res = await fetch('/api/mobile/execute', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ command: cmd })
    });
    const data = await res.json();
    fetchMobileStatus();
    fetchAgentOutput();
  } catch (e) {
    addLog(`Task failed: ${e.message}`, 'error');
  }
}

// Setup Event Listeners
function setupEvents() {
  // Mode Switcher Tabs
  const tabAi = document.getElementById('tab-ai-mode');
  const tabDev = document.getElementById('tab-dev-mode');
  const secAi = document.getElementById('section-ai-mode');
  const secDev = document.getElementById('section-dev-mode');

  if (tabAi && tabDev) {
    tabAi.addEventListener('click', () => {
      tabAi.classList.add('active');
      tabDev.classList.remove('active');
      if (secAi) secAi.style.display = 'block';
      if (secDev) secDev.style.display = 'none';
      feedback.vibrate([20]);
    });

    tabDev.addEventListener('click', () => {
      tabDev.classList.add('active');
      tabAi.classList.remove('active');
      if (secAi) secAi.style.display = 'none';
      if (secDev) secDev.style.display = 'block';
      feedback.vibrate([20]);
    });
  }

  // Haptics Toggle
  const btnHaptics = document.getElementById('btn-haptics-toggle');
  const hapticText = document.getElementById('haptic-text');
  const hapticIcon = document.getElementById('haptic-icon');
  btnHaptics.addEventListener('click', () => {
    feedback.hapticsEnabled = !feedback.hapticsEnabled;
    hapticText.textContent = feedback.hapticsEnabled ? 'ON' : 'OFF';
    hapticIcon.textContent = feedback.hapticsEnabled ? '📳' : '📴';
    if (feedback.hapticsEnabled) feedback.vibrate([50]);
  });

  // Sound Toggle
  const btnSound = document.getElementById('btn-sound-toggle');
  const soundIcon = document.getElementById('sound-icon');
  btnSound.addEventListener('click', () => {
    feedback.soundEnabled = !feedback.soundEnabled;
    soundIcon.textContent = feedback.soundEnabled ? '🔊' : '🔇';
    if (feedback.soundEnabled) feedback.playBeep(520);
  });

  // Media Button
  if (btnMedia) btnMedia.addEventListener('click', toggleMedia);

  // Toggle Primary
  if (togglePrimary) togglePrimary.addEventListener('change', togglePrimaryMode);

  // Send Prompt Button
  if (btnSendPrompt) btnSendPrompt.addEventListener('click', sendPrompt);

  // Send Prompt on Enter (Shift+Enter for newline)
  if (promptInput) {
    promptInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        sendPrompt();
      }
    });
  }

  // Run Dev Command Button
  const btnRunCmd = document.getElementById('btn-run-cmd');
  if (btnRunCmd) btnRunCmd.addEventListener('click', () => runDevCommand());

  // Run Dev Command on Enter
  const cmdInput = document.getElementById('m-cmd-input');
  if (cmdInput) {
    cmdInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        runDevCommand();
      }
    });
  }

  // Quick AI Chips
  document.querySelectorAll('.m-chip:not(.dev-chip)').forEach(chip => {
    chip.addEventListener('click', () => {
      promptInput.value = chip.dataset.prompt;
      feedback.vibrate([30]);
    });
  });

  // Quick Dev Chips
  document.querySelectorAll('.dev-chip').forEach(chip => {
    chip.addEventListener('click', () => {
      const cmd = chip.dataset.cmd;
      const cmdInput = document.getElementById('m-cmd-input');
      if (cmdInput) cmdInput.value = cmd;
      feedback.vibrate([30]);
      runDevCommand(cmd);
    });
  });

  // Copy Output Button
  const btnCopyOutput = document.getElementById('btn-copy-output');
  if (btnCopyOutput) {
    btnCopyOutput.addEventListener('click', () => {
      const outputText = agentOutputEl ? agentOutputEl.textContent : '';
      if (outputText) {
        navigator.clipboard.writeText(outputText);
        feedback.success();
        addLog('Copied agent output to clipboard.', 'info');
      }
    });
  }
}

const agentOutputEl = document.getElementById('m-agent-output');
let lastFetchedOutput = '';

async function fetchAgentOutput() {
  try {
    const res = await fetch('/api/mobile/output');
    if (!res.ok) return;
    const data = await res.json();
    if (data.output && data.output !== lastFetchedOutput) {
      lastFetchedOutput = data.output;
      if (agentOutputEl) {
        agentOutputEl.textContent = data.output;
      }
    }
  } catch (e) {}
}

document.addEventListener('DOMContentLoaded', () => {
  setupEvents();
  fetchMobileStatus();
  fetchAgentOutput();
  setInterval(fetchMobileStatus, 1500);
  setInterval(fetchAgentOutput, 1500);

  // Mobile Client Session Heartbeat
  const mobileClientId = 'mob_' + Math.random().toString(36).substr(2, 9);
  function sendMobileHeartbeat() {
    fetch('/api/heartbeat?client=' + mobileClientId, { method: 'POST' }).catch(() => {});
  }
  sendMobileHeartbeat();
  setInterval(sendMobileHeartbeat, 3000);

  window.addEventListener('beforeunload', () => {
    if (navigator.sendBeacon) {
      navigator.sendBeacon('/api/disconnect?client=' + mobileClientId);
    }
  });
});
