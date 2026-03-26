(() => {
  const API_BASE = location.hostname === 'localhost' || location.hostname === '127.0.0.1'
    ? 'http://127.0.0.1:8100'
    : '';

  let token = localStorage.getItem('ph_token') || '';
  let paused = false;
  let eventSource = null;
  let statsTimer = null;

  const $ = (id) => document.getElementById(id);

  function authHeaders() {
    return { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' };
  }

  // ── Login ──
  $('login-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const t = $('token-input').value.trim();
    if (!t) return;
    try {
      const res = await fetch(`${API_BASE}/api/auth/check?token=${encodeURIComponent(t)}`);
      if (!res.ok) { $('login-error').textContent = 'Invalid token'; return; }
      const data = await res.json();
      token = t;
      localStorage.setItem('ph_token', t);
      showDashboard(data.label);
    } catch (err) {
      $('login-error').textContent = 'API unreachable';
    }
  });

  $('logout-btn').addEventListener('click', () => {
    token = '';
    localStorage.removeItem('ph_token');
    if (eventSource) { eventSource.close(); eventSource = null; }
    if (statsTimer) { clearInterval(statsTimer); statsTimer = null; }
    $('dashboard').style.display = 'none';
    $('login-view').style.display = '';
    $('stream').innerHTML = '';
  });

  // ── Auto-login if token saved ──
  async function tryAutoLogin() {
    if (!token) return;
    try {
      const res = await fetch(`${API_BASE}/api/auth/check?token=${encodeURIComponent(token)}`);
      if (res.ok) {
        const data = await res.json();
        showDashboard(data.label);
      } else {
        localStorage.removeItem('ph_token');
        token = '';
      }
    } catch (_) { /* show login */ }
  }

  function showDashboard(label) {
    $('login-view').style.display = 'none';
    $('dashboard').style.display = '';
    $('user-label').textContent = label || 'user';
    refreshHealth();
    loadRecentEvents();
    connectStream();
    refreshStats();
    statsTimer = setInterval(refreshStats, 10000);
  }

  // ── Health ──
  async function refreshHealth() {
    const apiChip = $('api-chip');
    const natsChip = $('nats-chip');
    try {
      const res = await fetch(`${API_BASE}/health`);
      const d = await res.json();
      apiChip.textContent = `API: ${d.status}`;
      apiChip.className = 'chip ok';
      natsChip.textContent = `NATS: ${d.nats_connected ? 'connected' : 'disconnected'}`;
      natsChip.className = `chip ${d.nats_connected ? 'ok' : 'bad'}`;
    } catch (_) {
      apiChip.textContent = 'API: down';
      apiChip.className = 'chip bad';
      natsChip.textContent = 'NATS: unknown';
      natsChip.className = 'chip';
    }
  }

  // ── Stats + Services + Subjects ──
  async function refreshStats() {
    try {
      const res = await fetch(`${API_BASE}/api/dashboard/stats`, { headers: authHeaders() });
      if (!res.ok) return;
      const d = await res.json();

      $('stat-events').textContent = formatNum(d.total_events || 0);
      $('stat-subjects').textContent = (d.subjects || []).length;

      const svcs = d.services || [];
      $('stat-services').textContent = svcs.length;
      $('stat-online').textContent = svcs.filter(s => s.status === 'online').length;

      renderServices(svcs);
      renderSubjects(d.subjects || []);
      refreshHealth();
    } catch (_) { /* retry next interval */ }
  }

  function renderServices(svcs) {
    const grid = $('services-grid');
    grid.innerHTML = svcs.length === 0 ? '<div style="color:var(--muted);font-size:0.85rem">No services registered yet</div>' : '';
    svcs.sort((a, b) => a.name.localeCompare(b.name)).forEach(s => {
      const card = document.createElement('div');
      card.className = 'svc-card';
      const ago = timeAgo(s.last_seen);
      card.innerHTML = `
        <div class="svc-name"><span class="svc-status ${s.status}"></span>${esc(s.name)}</div>
        <div class="svc-meta">${s.version || ''} ${s.tags ? s.tags.join(', ') : ''}</div>
        <div class="svc-meta">${s.status} &middot; ${ago}</div>
      `;
      grid.appendChild(card);
    });
  }

  function renderSubjects(subjects) {
    const list = $('subjects-list');
    list.innerHTML = subjects.length === 0 ? '<div style="color:var(--muted);font-size:0.85rem">No events yet</div>' : '';
    subjects.sort((a, b) => b.count - a.count).forEach(s => {
      const row = document.createElement('div');
      row.className = 'subj-row';
      row.innerHTML = `<span>${esc(s.subject)}</span><span class="subj-count">${formatNum(s.count)}</span>`;
      list.appendChild(row);
    });
  }

  // ── Recent Events ──
  async function loadRecentEvents() {
    try {
      const res = await fetch(`${API_BASE}/api/events/recent?limit=50`, { headers: authHeaders() });
      if (!res.ok) return;
      const d = await res.json();
      (d.events || []).forEach(evt => renderEvent(evt, false));
    } catch (_) { /* no-op */ }
  }

  // ── SSE Stream ──
  function connectStream() {
    if (eventSource) eventSource.close();
    eventSource = new EventSource(`${API_BASE}/api/events/stream?token=${encodeURIComponent(token)}`);
    eventSource.addEventListener('event', (e) => {
      if (paused) return;
      try { renderEvent(JSON.parse(e.data), true); } catch (_) {}
    });
    eventSource.onerror = () => {
      setTimeout(() => { if (token) connectStream(); }, 3000);
    };
  }

  function renderEvent(evt, toTop) {
    const el = document.createElement('article');
    el.className = toTop ? 'event new' : 'event';
    const payload = typeof evt.payload === 'string' ? evt.payload : JSON.stringify(evt.payload, null, 2);
    const ts = evt.timestamp ? new Date(evt.timestamp).toLocaleTimeString() : '';
    el.innerHTML = `
      <div class="meta">
        <span class="subj">${esc(evt.subject || '')}</span>
        <span>${ts}</span>
      </div>
      <pre>${esc(payload)}</pre>
    `;
    const stream = $('stream');
    if (toTop && stream.firstChild) {
      stream.insertBefore(el, stream.firstChild);
    } else {
      stream.appendChild(el);
    }
    while (stream.children.length > 200) stream.removeChild(stream.lastChild);
  }

  // ── Pause / Clear ──
  $('pause-btn').addEventListener('click', () => {
    paused = !paused;
    $('pause-btn').textContent = paused ? 'Resume' : 'Pause';
  });
  $('clear-btn').addEventListener('click', () => { $('stream').innerHTML = ''; });

  // ── Publish ──
  $('publish-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const subject = $('pub-subject').value.trim();
    const raw = $('pub-payload').value.trim();
    let payload;
    try { payload = JSON.parse(raw || '{}'); } catch (_) { alert('Invalid JSON'); return; }
    const res = await fetch(`${API_BASE}/api/events`, {
      method: 'POST', headers: authHeaders(),
      body: JSON.stringify({ subject, payload }),
    });
    if (!res.ok) { const e = await res.json().catch(() => ({})); alert(e.error || 'Failed'); }
  });

  // ── Helpers ──
  function esc(s) { return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;'); }
  function formatNum(n) { return n >= 1000 ? (n/1000).toFixed(1) + 'k' : String(n); }
  function timeAgo(ts) {
    if (!ts) return 'never';
    const diff = (Date.now() - new Date(ts).getTime()) / 1000;
    if (diff < 60) return Math.floor(diff) + 's ago';
    if (diff < 3600) return Math.floor(diff/60) + 'm ago';
    if (diff < 86400) return Math.floor(diff/3600) + 'h ago';
    return Math.floor(diff/86400) + 'd ago';
  }

  tryAutoLogin();
})();
