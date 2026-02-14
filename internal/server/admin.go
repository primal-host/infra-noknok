package server

func adminPanelHTML(role string) string {
	ownerOnly := ""
	if role == "owner" {
		ownerOnly = `
        <option value="admin">Admin</option>
        <option value="owner">Owner</option>`
	}

	return `
<!-- Admin Modal -->
<div id="admin-overlay" style="display:none;position:fixed;top:0;right:0;bottom:0;left:0;background:rgba(0,0,0,0.6);z-index:1000;align-items:center;justify-content:center">
<div style="background:#1e293b;border-radius:12px;width:100%;max-width:720px;max-height:85vh;display:flex;flex-direction:column;box-shadow:0 8px 32px rgba(0,0,0,0.4)">
  <div style="display:flex;justify-content:space-between;align-items:center;padding:1.25rem 1.5rem;border-bottom:1px solid #334155">
    <h2 style="font-size:1.125rem;font-weight:600;color:#f8fafc;margin:0">Admin</h2>
    <button onclick="closeAdmin()" style="background:none;border:none;color:#94a3b8;font-size:1.25rem;cursor:pointer;padding:0.25rem">&times;</button>
  </div>
  <div style="display:flex;border-bottom:1px solid #334155">
    <button class="admin-tab active" data-tab="users" onclick="switchTab('users')">Users</button>
    <button class="admin-tab" data-tab="services" onclick="switchTab('services')">Services</button>
    <button class="admin-tab" data-tab="access" onclick="switchTab('access')">Access</button>
  </div>
  <div id="admin-content" style="padding:1.25rem 1.5rem;overflow-y:auto;flex:1">
  </div>
</div>
</div>

<style>
.admin-tab {
  background:none;border:none;color:#94a3b8;padding:0.75rem 1.25rem;font-size:0.875rem;cursor:pointer;
  border-bottom:2px solid transparent;transition:color 0.15s,border-color 0.15s;
}
.admin-tab:hover { color:#e2e8f0; }
.admin-tab.active { color:#3b82f6;border-bottom-color:#3b82f6; }
.admin-tbl { width:100%;border-collapse:collapse;font-size:0.8125rem; }
.admin-tbl th { text-align:left;color:#94a3b8;font-weight:500;padding:0.5rem 0.75rem;border-bottom:1px solid #334155; }
.admin-tbl td { padding:0.5rem 0.75rem;color:#e2e8f0;border-bottom:1px solid #1e293b; }
.admin-tbl tr:hover td { background:#263044; }
.admin-input {
  background:#0f172a;border:1px solid #334155;border-radius:6px;color:#f8fafc;padding:0.375rem 0.625rem;
  font-size:0.8125rem;outline:none;transition:border-color 0.15s;
}
.admin-input:focus { border-color:#3b82f6; }
.admin-select {
  background:#0f172a;border:1px solid #334155;border-radius:6px;color:#f8fafc;padding:0.375rem 0.625rem;
  font-size:0.8125rem;outline:none;
}
.admin-btn {
  background:#3b82f6;color:#fff;border:none;border-radius:6px;padding:0.375rem 0.75rem;
  font-size:0.8125rem;cursor:pointer;transition:background 0.15s;
}
.admin-btn:hover { background:#2563eb; }
.admin-btn-danger {
  background:#dc2626;color:#fff;border:none;border-radius:6px;padding:0.25rem 0.5rem;
  font-size:0.75rem;cursor:pointer;transition:background 0.15s;
}
.admin-btn-danger:hover { background:#b91c1c; }
.admin-form { display:flex;gap:0.5rem;align-items:center;margin-top:1rem;flex-wrap:wrap; }
.admin-msg { font-size:0.8125rem;padding:0.5rem;border-radius:6px;margin-bottom:0.75rem; }
.admin-msg-ok { background:#14532d;color:#86efac; }
.admin-msg-err { background:#7f1d1d;color:#fca5a5; }
.access-check { width:18px;height:18px;cursor:pointer;accent-color:#3b82f6; }
</style>

<script>
const ROLE = '` + role + `';
let currentTab = 'users';
let adminData = { users: [], services: [], grants: [] };

function openAdmin() {
  document.getElementById('admin-overlay').style.display = 'flex';
  loadTab('users');
}
function closeAdmin() {
  document.getElementById('admin-overlay').style.display = 'none';
}
document.addEventListener('keydown', e => { if (e.key === 'Escape') closeAdmin(); });

function switchTab(tab) {
  currentTab = tab;
  document.querySelectorAll('.admin-tab').forEach(t => {
    t.classList.toggle('active', t.dataset.tab === tab);
  });
  loadTab(tab);
}

async function api(method, path, body) {
  const opts = { method, headers: { 'Content-Type': 'application/json' } };
  if (body) opts.body = JSON.stringify(body);
  const r = await fetch('/admin/api' + path, opts);
  if (r.status === 204) return null;
  const data = await r.json();
  if (!r.ok) throw new Error(data.error || 'request failed');
  return data;
}

async function loadTab(tab) {
  const el = document.getElementById('admin-content');
  try {
    if (tab === 'users') {
      adminData.users = await api('GET', '/users');
      renderUsers(el);
    } else if (tab === 'services') {
      adminData.services = await api('GET', '/services');
      renderServices(el);
    } else if (tab === 'access') {
      adminData.users = await api('GET', '/users');
      adminData.services = await api('GET', '/services');
      adminData.grants = await api('GET', '/grants');
      renderAccess(el);
    }
  } catch (e) {
    el.innerHTML = '<div class="admin-msg admin-msg-err">' + esc(e.message) + '</div>';
  }
}

function esc(s) {
  const d = document.createElement('div');
  d.textContent = s || '';
  return d.innerHTML;
}

function renderUsers(el) {
  let html = '<table class="admin-tbl"><thead><tr><th>Handle</th><th>Username</th><th>Role</th><th>DID</th><th></th></tr></thead><tbody>';
  for (const u of adminData.users) {
    const canChangeRole = ROLE === 'owner';
    const usernameCell = '<input class="admin-input" style="width:90px;font-size:0.75rem" value="' + esc(u.username || '') + '" onchange="updateUsername(' + u.id + ',this.value)">';
    const roleCell = canChangeRole
      ? '<select class="admin-select" onchange="updateRole(' + u.id + ',this.value)">' +
        '<option value="user"' + (u.role==='user'?' selected':'') + '>User</option>' +
        '<option value="admin"' + (u.role==='admin'?' selected':'') + '>Admin</option>' +
        '<option value="owner"' + (u.role==='owner'?' selected':'') + '>Owner</option></select>'
      : esc(u.role);
    const del = '<button class="admin-btn-danger" onclick="deleteUser(' + u.id + ')">Delete</button>';
    html += '<tr><td>' + esc(u.handle || '(no handle)') + '</td><td>' + usernameCell + '</td><td>' + roleCell + '</td><td style="font-size:0.75rem;color:#64748b;max-width:200px;overflow:hidden;text-overflow:ellipsis">' + esc(u.did) + '</td><td>' + del + '</td></tr>';
  }
  html += '</tbody></table>';
  html += '<div class="admin-form">' +
    '<input class="admin-input" id="add-handle" placeholder="handle" style="flex:1;min-width:150px">' +
    '<input class="admin-input" id="add-username" placeholder="username" style="width:90px">' +
    '<select class="admin-select" id="add-role"><option value="user">User</option>` + ownerOnly + `</select>' +
    '<button class="admin-btn" onclick="addUser()">Add</button></div>';
  html += '<div id="users-msg"></div>';
  el.innerHTML = html;
}

async function addUser() {
  const handle = document.getElementById('add-handle').value.trim();
  const username = document.getElementById('add-username').value.trim();
  const role = document.getElementById('add-role').value;
  const msg = document.getElementById('users-msg');
  if (!handle) return;
  try {
    await api('POST', '/users', { handle, role, username });
    document.getElementById('add-handle').value = '';
    document.getElementById('add-username').value = '';
    msg.className = 'admin-msg admin-msg-ok'; msg.textContent = 'User added';
    loadTab('users');
  } catch (e) {
    msg.className = 'admin-msg admin-msg-err'; msg.textContent = e.message;
  }
}

async function updateUsername(id, username) {
  const msg = document.getElementById('users-msg');
  try {
    await api('PUT', '/users/' + id + '/username', { username });
    msg.className = 'admin-msg admin-msg-ok'; msg.textContent = 'Username updated';
    setTimeout(() => { msg.className = ''; msg.textContent = ''; }, 1500);
  } catch (e) {
    msg.className = 'admin-msg admin-msg-err'; msg.textContent = e.message;
    loadTab('users');
  }
}

async function updateRole(id, role) {
  try {
    await api('PUT', '/users/' + id + '/role', { role });
  } catch (e) {
    alert(e.message);
    loadTab('users');
  }
}

async function deleteUser(id) {
  if (!confirm('Delete this user?')) return;
  try {
    await api('DELETE', '/users/' + id);
    loadTab('users');
  } catch (e) { alert(e.message); }
}

function renderServices(el) {
  let html = '<table class="admin-tbl"><thead><tr><th>Name</th><th>Slug</th><th>URL</th><th>Admin Role</th><th></th></tr></thead><tbody>';
  for (const s of adminData.services) {
    html += '<tr><td>' + esc(s.name) + '</td><td style="color:#64748b">' + esc(s.slug) + '</td><td style="font-size:0.75rem;color:#64748b">' + esc(s.url) + '</td>' +
      '<td><input class="admin-input" style="width:70px;font-size:0.75rem" value="' + esc(s.admin_role) + '" onchange="updateServiceAdminRole(' + s.id + ',this.value)"></td>' +
      '<td><button class="admin-btn-danger" onclick="deleteService(' + s.id + ')">Delete</button></td></tr>';
  }
  html += '</tbody></table>';
  html += '<div class="admin-form">' +
    '<input class="admin-input" id="svc-name" placeholder="Name" style="width:100px">' +
    '<input class="admin-input" id="svc-slug" placeholder="slug" style="width:80px">' +
    '<input class="admin-input" id="svc-url" placeholder="https://..." style="flex:1;min-width:130px">' +
    '<input class="admin-input" id="svc-desc" placeholder="Description" style="width:110px">' +
    '<input class="admin-input" id="svc-admin-role" placeholder="admin" style="width:70px">' +
    '<button class="admin-btn" onclick="addService()">Add</button></div>';
  html += '<div id="services-msg"></div>';
  el.innerHTML = html;
}

async function addService() {
  const name = document.getElementById('svc-name').value.trim();
  const slug = document.getElementById('svc-slug').value.trim();
  const url = document.getElementById('svc-url').value.trim();
  const desc = document.getElementById('svc-desc').value.trim();
  const adminRole = document.getElementById('svc-admin-role').value.trim() || 'admin';
  const msg = document.getElementById('services-msg');
  if (!name || !slug || !url) { msg.className = 'admin-msg admin-msg-err'; msg.textContent = 'Name, slug, and URL required'; return; }
  try {
    await api('POST', '/services', { name, slug, url, description: desc, icon_url: '', admin_role: adminRole });
    document.getElementById('svc-name').value = '';
    document.getElementById('svc-slug').value = '';
    document.getElementById('svc-url').value = '';
    document.getElementById('svc-desc').value = '';
    document.getElementById('svc-admin-role').value = '';
    msg.className = 'admin-msg admin-msg-ok'; msg.textContent = 'Service added';
    loadTab('services');
  } catch (e) {
    msg.className = 'admin-msg admin-msg-err'; msg.textContent = e.message;
  }
}

async function updateServiceAdminRole(id, adminRole) {
  const svc = adminData.services.find(s => s.id === id);
  if (!svc) return;
  const msg = document.getElementById('services-msg');
  try {
    await api('PUT', '/services/' + id, { name: svc.name, description: svc.description, url: svc.url, icon_url: svc.icon_url, admin_role: adminRole });
    svc.admin_role = adminRole;
    msg.className = 'admin-msg admin-msg-ok'; msg.textContent = 'Admin role updated';
    setTimeout(() => { msg.className = ''; msg.textContent = ''; }, 1500);
  } catch (e) {
    msg.className = 'admin-msg admin-msg-err'; msg.textContent = e.message;
  }
}

async function deleteService(id) {
  if (!confirm('Delete this service? Grants will also be removed.')) return;
  try {
    await api('DELETE', '/services/' + id);
    loadTab('services');
  } catch (e) { alert(e.message); }
}

function renderAccess(el) {
  const users = adminData.users;
  const services = adminData.services;
  const grantMap = {};
  for (const g of adminData.grants) {
    grantMap[g.user_id + ':' + g.service_id] = g;
  }

  let html = '<table class="admin-tbl"><thead><tr><th>User</th>';
  for (const s of services) {
    html += '<th style="text-align:center;font-size:0.75rem">' + esc(s.name) + '</th>';
  }
  html += '</tr></thead><tbody>';
  for (const u of users) {
    html += '<tr><td>' + esc(u.handle || u.did) + '</td>';
    for (const s of services) {
      const key = u.id + ':' + s.id;
      const grant = grantMap[key];
      const checked = grant ? ' checked' : '';
      const role = grant ? grant.role : 'user';
      html += '<td style="text-align:center">' +
        '<input type="checkbox" class="access-check"' + checked +
        ' onchange="toggleGrant(' + u.id + ',' + s.id + ',this.checked)">' +
        '<br><input class="admin-input" style="width:60px;font-size:0.6875rem;margin-top:2px;text-align:center" ' +
        'value="' + esc(role) + '" ' +
        'onchange="updateGrantRole(' + u.id + ',' + s.id + ',this.value)"' +
        (grant ? '' : ' disabled') + '></td>';
    }
    html += '</tr>';
  }
  html += '</tbody></table>';
  html += '<div id="access-msg"></div>';
  el.innerHTML = html;
}

async function toggleGrant(userId, serviceId, checked) {
  const msg = document.getElementById('access-msg');
  try {
    if (checked) {
      await api('POST', '/grants', { user_id: userId, service_id: serviceId, role: 'user' });
    } else {
      const key = userId + ':' + serviceId;
      const grant = adminData.grants.find(g => g.user_id === userId && g.service_id === serviceId);
      if (grant) {
        await api('DELETE', '/grants/' + grant.id);
      }
    }
    adminData.grants = await api('GET', '/grants');
    renderAccess(document.getElementById('admin-content'));
    msg.className = ''; msg.textContent = '';
  } catch (e) {
    msg.className = 'admin-msg admin-msg-err'; msg.textContent = e.message;
    loadTab('access');
  }
}

async function updateGrantRole(userId, serviceId, role) {
  const msg = document.getElementById('access-msg');
  try {
    await api('POST', '/grants', { user_id: userId, service_id: serviceId, role: role });
    adminData.grants = await api('GET', '/grants');
    msg.className = 'admin-msg admin-msg-ok'; msg.textContent = 'Role updated';
    setTimeout(() => { msg.className = ''; msg.textContent = ''; }, 1500);
  } catch (e) {
    msg.className = 'admin-msg admin-msg-err'; msg.textContent = e.message;
  }
}
</script>`
}
