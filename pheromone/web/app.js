const API_BASE = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
  ? 'http://127.0.0.1:8100'
  : `${window.location.protocol}//${window.location.hostname}:8100`;

const apiStatusEl = document.getElementById('api-status');
const natsStatusEl = document.getElementById('nats-status');
const streamEl = document.getElementById('stream');
const form = document.getElementById('publish-form');

async function boot() {
  await refreshHealth();
  await loadRecent();
  connectStream();
}

async function refreshHealth() {
  try {
    const res = await fetch(`${API_BASE}/health`);
    const data = await res.json();
    apiStatusEl.textContent = `API: ${data.status}`;
    apiStatusEl.classList.add('ok');
    natsStatusEl.textContent = `NATS: ${data.nats_connected ? 'connected' : 'disconnected'}`;
    natsStatusEl.classList.add(data.nats_connected ? 'ok' : 'bad');
  } catch (err) {
    apiStatusEl.textContent = 'API: down';
    apiStatusEl.classList.add('bad');
    natsStatusEl.textContent = 'NATS: unknown';
  }
}

async function loadRecent() {
  try {
    const res = await fetch(`${API_BASE}/api/events/recent?limit=40`);
    const data = await res.json();
    (data.events || []).forEach(renderEvent);
  } catch (_) {
    // no-op
  }
}

function connectStream() {
  const es = new EventSource(`${API_BASE}/api/events/stream`);
  es.addEventListener('event', (e) => {
    try {
      const evt = JSON.parse(e.data);
      renderEvent(evt, true);
    } catch (_) {
      // no-op
    }
  });
}

function renderEvent(evt, toTop = false) {
  const wrapper = document.createElement('article');
  wrapper.className = 'event';
  const payload = typeof evt.payload === 'string' ? evt.payload : JSON.stringify(evt.payload, null, 2);
  wrapper.innerHTML = `
    <div class="meta">
      <span>${evt.subject || 'unknown'}</span>
      <span>${new Date(evt.timestamp || Date.now()).toLocaleString()}</span>
    </div>
    <pre>${escapeHtml(payload)}</pre>
  `;
  if (toTop && streamEl.firstChild) {
    streamEl.insertBefore(wrapper, streamEl.firstChild);
  } else {
    streamEl.appendChild(wrapper);
  }
}

form.addEventListener('submit', async (e) => {
  e.preventDefault();
  const subject = document.getElementById('subject').value.trim();
  const rawPayload = document.getElementById('payload').value.trim();

  let payload;
  try {
    payload = JSON.parse(rawPayload || '{}');
  } catch (_) {
    alert('Payload must be valid JSON');
    return;
  }

  const res = await fetch(`${API_BASE}/api/events`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ subject, payload }),
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'publish failed' }));
    alert(err.error || 'publish failed');
    return;
  }

  document.getElementById('payload').value = JSON.stringify(payload, null, 2);
});

function escapeHtml(str) {
  return str
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;');
}

boot();
