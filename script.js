let allRepos = [];
let currentView = 'grid';
let basePaths = [];
let selectedRepo = null;

const statusColors = { 'synced': '#10b981', 'untracked': '#f43f5e', 'commit': '#fbbf24', 'pending_sync': '#a855f7', 'no_remote': '#f97316', 'not_init': '#555' };

// UI Helpers
function hideLoader() { const loader = document.getElementById('loader'); if (loader) { loader.style.opacity = '0'; setTimeout(() => loader.style.display = 'none', 400); } }
function setTab(tab, el) {
    document.querySelectorAll('.nav-item').forEach(nav => nav.classList.remove('active'));
    if (el) el.classList.add('active');
    document.getElementById('reposView').style.display = (tab === 'repos') ? 'block' : 'none';
    document.getElementById('reposHeader').style.display = (tab === 'repos') ? 'flex' : 'none';
    document.getElementById('monitoringView').style.display = (tab === 'monitoring') ? 'block' : 'none';
    document.getElementById('statsView').style.display = (tab === 'stats') ? 'block' : 'none';
    if (tab === 'repos') fetchData(); else if (tab === 'monitoring') fetchBasePaths();
}

// Data Management
async function fetchData() {
    try {
        const [repRes, cfgRes] = await Promise.all([fetch('/api/repos'), fetch('/api/config')]);
        allRepos = await repRes.json();
        basePaths = await cfgRes.json();
        updateMonitoredOptions();
        renderRepos();
    } catch (e) { console.error(e); } finally { hideLoader(); }
}

async function fetchRepoDetails(path) {
    const res = await fetch('/api/repo-details?path=' + encodeURIComponent(path));
    return await res.json();
}

// Context Menu
window.addEventListener('contextmenu', e => {
    const card = e.target.closest('.repo-card');
    if (card) {
        e.preventDefault();
        selectedRepo = allRepos.find(r => r.path === card.dataset.path);
        const menu = document.getElementById('contextMenu');
        menu.style.display = 'block'; menu.style.left = e.clientX + 'px'; menu.style.top = e.clientY + 'px';
    } else { document.getElementById('contextMenu').style.display = 'none'; }
});
window.addEventListener('click', () => { document.getElementById('contextMenu').style.display = 'none'; });

async function handleMenuAction(action) {
    if (!selectedRepo) return;
    switch (action) {
        case 'highlight': document.querySelector(`[data-path="${selectedRepo.path.replace(/\\/g, '\\\\')}"]`).classList.toggle('highlighted'); break;
        case 'remote':
            if (selectedRepo.remote_url) fetch('/api/open?path=' + encodeURIComponent(selectedRepo.remote_url) + '&action=remote');
            else alert('Nenhum remoto configurado.'); break;
        case 'monitoring':
            const parent = selectedRepo.path.split(/[\\/]/).slice(0, -1).join('\\');
            setTab('monitoring', document.getElementById('nav-monitoring'));
            setTimeout(() => {
                const items = document.querySelectorAll('#basePathsList > div');
                items.forEach(item => { if (item.dataset.path.toLowerCase() === parent.toLowerCase()) { item.classList.add('monitor-pulse'); item.scrollIntoView({ behavior: 'smooth', block: 'center' }); item.onmouseenter = () => item.classList.remove('monitor-pulse'); } });
            }, 100); break;
        case 'stats':
            const details = await fetchRepoDetails(selectedRepo.path);
            renderStats(details); break;
    }
}

function renderStats(d) {
    setTab('stats', document.getElementById('nav-stats'));
    document.getElementById('statsEmpty').style.display = 'none';
    const sd = document.getElementById('statsDetails');
    sd.style.display = 'block';

    const folderName = d.path.split(/[\\/]/).pop();
    const tagsHtml = d.tags ? d.tags.split('\n').map(t => `<span class="tag-pill">${t}</span>`).join('') : '<span style="color:var(--text-dim)">Sem tags</span>';

    // Verifica se o log contém apenas o último commit para evitar duplicidade visual óbvia
    const logLines = d.recent_activity ? d.recent_activity.split('\n') : [];
    const showLog = logLines.length > 1;

    sd.innerHTML = `
        <div style="background: var(--sidebar-bg); border:1px solid var(--border); border-radius:20px; padding:32px; margin-bottom:20px;">
            <div style="display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:32px; border-bottom:1px solid var(--border); padding-bottom:24px;">
                <div>
                    <div style="display:flex; align-items:center; gap:12px; margin-bottom:8px;">
                        <h2 style="margin:0; font-size:2rem; font-weight:800;">${folderName}</h2>
                        <span class="status-pill" style="color:var(--accent); border:1px solid var(--accent)44; height:24px; min-width:auto; padding:0 10px;">${d.branch}</span>
                    </div>
                    <p style="color:var(--text-dim); margin:0; font-family:monospace; font-size:0.85rem;">${d.path}</p>
                </div>
                <div style="text-align:right">
                    <div class="stats-label" style="margin-bottom:4px">Commits</div>
                    <div style="font-size:1.5rem; font-weight:800; color:white">${d.commit_count || '0'}</div>
                </div>
            </div>

            <div class="stats-grid" style="grid-template-columns: 1.5fr 1fr;">
                <div class="stats-box" style="background: rgba(99, 102, 241, 0.03); border-color: var(--accent)22">
                    <div class="stats-label">Último Commit</div>
                    <div style="display:flex; align-items:center; gap:10px; margin-bottom:12px;">
                        <div style="width:32px; height:32px; border-radius:50%; background:var(--accent); display:flex; align-items:center; justify-content:center; font-weight:800; font-size:0.8rem;">${(d.last_author || 'U').charAt(0)}</div>
                        <div>
                            <div style="color:white; font-weight:700; font-size:0.95rem;">${d.last_author || 'Desconhecido'}</div>
                            <div style="font-size:0.75rem; color:var(--text-dim);">${d.last_date}</div>
                        </div>
                    </div>
                    <div style="font-size:1.1rem; color:white; font-weight:500; line-height:1.4; margin-top:16px;">"${d.last_commit || 'Sem mensagem'}"</div>
                </div>
                
                <div class="stats-box">
                    <div class="stats-label">Tags & Versões</div>
                    <div style="margin-top:10px;">${tagsHtml}</div>
                    
                    <div class="stats-label" style="margin-top:24px;">Ranking de Contribuição</div>
                    <div style="font-size:0.85rem; color:var(--text-dim); line-height:1.5; font-family:monospace;">${d.summary.replace(/\n/g, '<br>') || 'N/A'}</div>
                </div>
            </div>

            ${showLog ? `
            <div style="margin-top:24px;">
                <div class="stats-label">Histórico Recente (Anteriores)</div>
                <div class="git-log-box" style="opacity:0.8">${logLines.slice(1).join('\n') || 'Nenhum commit anterior'}</div>
            </div>` : ''}

            <div style="margin-top:24px; padding-top:24px; border-top:1px solid var(--border);">
                <div class="stats-label">Integridade Local (.git)</div>
                <div class="git-log-box" style="background:transparent; border:none; padding:0; display:grid; grid-template-columns:repeat(auto-fill, minmax(200px, 1fr)); gap:10px;">
                    ${d.disk_usage.split('\n').filter(l => l.includes(':')).map(l => {
        const [k, v] = l.split(':');
        return `<div><span style="color:var(--text-dim)">${k}:</span> <span style="color:white">${v}</span></div>`;
    }).join('')}
                </div>
            </div>
        </div>
    `;
}

// Project View Management
function updateMonitoredOptions() {
    const s = document.getElementById('sortMonitored');
    const val = s.value;
    s.innerHTML = '<option value="any">Qualquer</option>';
    basePaths.forEach(p => { const o = document.createElement('option'); o.value = p; o.textContent = p.split(/[\\/]/).pop() || p; s.appendChild(o); });
    s.value = val;
    setTimeout(initCustomSelects, 0);
}

function updateSubControls() {
    const t = document.getElementById('sortType').value;
    document.querySelectorAll('.secondary-control').forEach(c => c.classList.remove('visible'));
    const s = document.getElementById('sub' + t.charAt(0).toUpperCase() + t.slice(1));
    if (s) s.classList.add('visible');

    // Forçar recriação dos dropdowns customizados para novos elementos visíveis
    initCustomSelects();
    renderRepos();
}

function toggleDateInput() { document.getElementById('dateInput').style.display = (document.getElementById('sortDate').value === 'custom') ? 'block' : 'none'; renderRepos(); }
function toggleSizeInput() { document.getElementById('sizeInput').style.display = (document.getElementById('sortSize').value === 'approx') ? 'block' : 'none'; renderRepos(); }
function toggleView() {
    const nextView = (currentView === 'grid') ? 'list' : 'grid';
    setView(nextView);
}

function setView(v) {
    currentView = v;
    document.getElementById('grid').className = (v === 'grid') ? 'repo-grid' : 'repo-list';
    document.getElementById('listHeader').style.display = (v === 'list') ? 'flex' : 'none';

    const btn = document.getElementById('viewToggle');
    btn.textContent = (v === 'grid') ? 'Lista' : 'Grade';
    renderRepos();
}

function renderRepos() {
    const grid = document.getElementById('grid');
    const search = document.getElementById('searchInput').value.toLowerCase();
    const sortType = document.getElementById('sortType').value;
    let filtered = allRepos.filter(r => r.name.toLowerCase().includes(search) || r.path.toLowerCase().includes(search));

    // Lógica de Ordenação Completa
    filtered.sort((a, b) => {
        if (sortType === 'name') {
            const sub = document.getElementById('sortName').value;
            return sub === 'az' ? a.name.localeCompare(b.name) : b.name.localeCompare(a.name);
        }
        if (sortType === 'size') {
            const getVal = s => {
                const n = parseFloat(s);
                if (s.includes('GB')) return n * 1024;
                if (s.includes('KB')) return n / 1024;
                return n;
            };
            const sub = document.getElementById('sortSize').value;
            return sub === 'large' ? getVal(b.size) - getVal(a.size) : getVal(a.size) - getVal(b.size);
        }
        if (sortType === 'status') {
            const sub = document.getElementById('sortStatus').value;
            if (sub !== 'any') filtered = filtered.filter(r => r.status === sub);
            return a.name.localeCompare(b.name);
        }
        return a.name.localeCompare(b.name);
    });

    if (sortType === 'monitored') {
        const p = document.getElementById('sortMonitored').value;
        if (p !== 'any') filtered = filtered.filter(r => r.path.toLowerCase().startsWith(p.toLowerCase()));
    }

    grid.innerHTML = '';
    filtered.forEach(repo => {
        const col = statusColors[repo.status] || '#fff';
        const card = document.createElement('div');
        card.className = 'repo-card'; card.dataset.path = repo.path;
        card.onclick = () => fetch('/api/open?path=' + encodeURIComponent(repo.path) + '&action=explorer');

        if (currentView === 'grid') {
            card.innerHTML = `
                <div class="repo-info-main">
                    <div class="repo-name">${repo.name}</div>
                    <div class="repo-path">${repo.path}</div>
                </div>
                <div style="margin-top:12px; display:flex; align-items:center; justify-content:space-between;">
                    <div class="status-pill" style="color:${col}; border:1px solid ${col}44; min-width:auto; padding:0 10px; height:22px;">
                        <span style="width:6px; height:6px; border-radius:50%; background:${col}"></span>${repo.status}
                    </div>
                    <div class="stat-text" style="font-size:0.7rem; opacity:0.6;">${repo.size}</div>
                </div>
                <div class="repo-footer" style="margin-top:8px; padding-top:8px; border-top:1px solid rgba(255,255,255,0.03);">
                    <div class="stat-text" style="font-size:0.7rem;">${repo.relative_time}</div>
                </div>`;
        } else {
            card.innerHTML = `<div class="col-status"><div class="status-pill" style="color:${col}; border:1px solid ${col}44">${repo.status}</div></div>
                <div class="col-name" style="padding-left:12px;"><div class="repo-name" style="font-size:1.05rem;">${repo.name}</div></div>
                <div class="col-path"><div class="repo-path" style="opacity:0.7;">${repo.path}</div></div>
                <div class="col-size"><div class="stat-text" style="font-weight:600;">${repo.size}</div></div>
                <div class="col-time"><div class="stat-text" style="color:var(--text-dim); font-size:0.75rem;">${repo.relative_time}</div></div>`;
        }
        grid.appendChild(card);
    });
    document.getElementById('statTotal').innerText = filtered.length;
}

async function fetchBasePaths() {
    try {
        const res = await fetch('/api/config');
        basePaths = await res.json();
        const list = document.getElementById('basePathsList');
        list.innerHTML = '';
        basePaths.forEach(p => {
            const item = document.createElement('div');
            item.dataset.path = p;
            item.style = 'display:flex; justify-content:space-between; align-items:center; padding:16px; background:rgba(255,255,255,0.02); border-radius:12px; border:1px solid var(--border)';
            item.innerHTML = `<div><div style="font-weight:600; color:white">${p}</div></div>
                <button class="action-btn" style="background:rgba(244,63,94,0.1); color:#f43f5e;" onclick="event.stopPropagation(); removePath('${p.replace(/\\/g, '\\\\')}')">Remover</button>`;
            list.appendChild(item);
        });
        updateMonitoredOptions();
    } catch (e) { console.error(e); }
}

async function removePath(path) { if (confirm('Remover?')) { await fetch('/api/remove-path', { method: 'POST', body: JSON.stringify({ path }), headers: { 'Content-Type': 'application/json' } }); fetchBasePaths(); } }
async function addPath() { const p = prompt("Caminho:"); if (p) { await fetch('/api/add-path', { method: 'POST', body: JSON.stringify({ path: p }), headers: { 'Content-Type': 'application/json' } }); fetchBasePaths(); } }

// Daemon Modal Logic
let startTime = Date.now();
function openDaemonModal() {
    document.getElementById('daemonModal').style.display = 'flex';
    document.getElementById('daemonProjectCount').textContent = allRepos.length;
    updateUptime();
}
function closeDaemonModal() { document.getElementById('daemonModal').style.display = 'none'; }
function updateUptime() {
    const diff = Math.floor((Date.now() - startTime) / 1000);
    const h = String(Math.floor(diff / 3600)).padStart(2, '0');
    const m = String(Math.floor((diff % 3600) / 60)).padStart(2, '0');
    const s = String(diff % 60).padStart(2, '0');
    document.getElementById('daemonUptime').textContent = `${h}:${m}:${s}`;
    if (document.getElementById('daemonModal').style.display === 'flex') setTimeout(updateUptime, 1000);
}
async function controlDaemon(action) {
    if (confirm(`Deseja realmente ${action === 'restart' ? 'reiniciar' : 'desligar'} o daemon?`)) {
        alert(`${action.charAt(0).toUpperCase() + action.slice(1)}ing daemon...`);
        closeDaemonModal();
    }
}

async function initCustomSelects() {
    document.querySelectorAll('select.control-btn').forEach(select => {
        if (select.nextElementSibling?.classList.contains('custom-select')) {
            select.nextElementSibling.remove();
        }

        const wrapper = document.createElement('div');
        wrapper.className = 'custom-select';
        const trigger = document.createElement('div');
        trigger.className = 'select-trigger';
        trigger.textContent = select.options[select.selectedIndex]?.textContent || '';

        const optionsDiv = document.createElement('div');
        optionsDiv.className = 'select-options';

        Array.from(select.options).forEach((opt, i) => {
            const item = document.createElement('div');
            item.className = 'select-option' + (i === select.selectedIndex ? ' selected' : '');
            item.textContent = opt.textContent;
            item.onclick = (e) => {
                e.preventDefault();
                e.stopPropagation();
                select.value = opt.value;
                trigger.textContent = opt.textContent;
                optionsDiv.querySelectorAll('.select-option').forEach(el => el.classList.remove('selected'));
                item.classList.add('selected');
                wrapper.classList.remove('active');
                select.dispatchEvent(new Event('change'));
            };
            optionsDiv.appendChild(item);
        });

        trigger.onclick = (e) => {
            e.preventDefault();
            e.stopPropagation();
            const wasActive = wrapper.classList.contains('active');
            document.querySelectorAll('.custom-select').forEach(s => s.classList.remove('active'));
            if (!wasActive) wrapper.classList.add('active');
        };

        wrapper.appendChild(trigger);
        wrapper.appendChild(optionsDiv);
        select.style.display = 'none';
        select.parentNode.insertBefore(wrapper, select.nextSibling);
    });
}
window.addEventListener('click', () => document.querySelectorAll('.custom-select').forEach(s => s.classList.remove('active')));

window.onload = async () => { await fetchData(); initCustomSelects(); };
