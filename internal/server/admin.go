package server

func adminPanelHTML(role string, open bool, activeTab string) string {
	ownerOnly := ""
	if role == "owner" {
		ownerOnly = `<option value="admin">Admin</option><option value="owner">Owner</option>`
	}

	display := "none"
	if open {
		display = "block"
	}

	autoLoad := ""
	if open {
		autoLoad = `
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', function() { loadTab('` + activeTab + `'); });
} else {
  loadTab('` + activeTab + `');
}`
	}

	tabActive := func(name string) string {
		if name == activeTab {
			return " active"
		}
		return ""
	}

	return `
<!-- Admin Panel -->
<div id="admin-panel" class="admin-card" style="display:` + display + `">
  <div class="admin-header">
    <h2>Admin</h2>
    <a href="/" class="admin-close">&times;</a>
  </div>
  <div class="admin-tabs">
    <a href="/?admin&tab=users" class="admin-tab` + tabActive("users") + `" data-tab="users">Users</a>
    <a href="/?admin&tab=services" class="admin-tab` + tabActive("services") + `" data-tab="services">Services</a>
    <a href="/?admin&tab=access" class="admin-tab` + tabActive("access") + `" data-tab="access">Access</a>
  </div>
  <div id="admin-content" class="admin-body">
  </div>
</div>

<style>
.admin-card {
  background: #1e293b;
  border-radius: 12px;
  max-width: 800px;
  margin: 0 auto 1.5rem;
  box-shadow: 0 4px 24px rgba(0,0,0,0.3);
  overflow: hidden;
}
.admin-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 1.5rem;
  border-bottom: 1px solid #334155;
}
.admin-header h2 {
  font-size: 1.125rem;
  font-weight: 600;
  color: #f8fafc;
  margin: 0;
}
.admin-close {
  color: #64748b;
  font-size: 0.875rem;
  text-decoration: none;
  width: 1.75rem;
  height: 1.75rem;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1.5px solid #475569;
  border-radius: 50%;
  transition: color 0.15s, border-color 0.15s, background 0.15s;
}
.admin-close:hover { color: #fff; border-color: #f97316; background: #f97316; }
.admin-tabs {
  display: flex;
  border-bottom: 1px solid #334155;
}
.admin-tab {
  background: none;
  border: none;
  color: #94a3b8;
  padding: 0.75rem 1.25rem;
  font-size: 0.875rem;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  transition: color 0.15s, border-color 0.15s;
  text-decoration: none;
}
.admin-tab:hover { color: #e2e8f0; }
.admin-tab.active { color: #3b82f6; border-bottom-color: #3b82f6; }
.admin-body {
  padding: 1.25rem 1.5rem;
  overflow-x: auto;
}
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
var ROLE = '` + role + `';
var adminData = { users: [], services: [], grants: [] };

function api(method, path, body, callback) {
  var xhr = new XMLHttpRequest();
  xhr.open(method, '/admin/api' + path, true);
  xhr.setRequestHeader('Content-Type', 'application/json');
  xhr.onreadystatechange = function() {
    if (xhr.readyState !== 4) return;
    if (xhr.status === 204) { callback(null, null); return; }
    try {
      var data = JSON.parse(xhr.responseText);
      if (xhr.status >= 200 && xhr.status < 300) {
        callback(null, data);
      } else {
        callback(data.error || 'request failed');
      }
    } catch (e) {
      callback('request failed');
    }
  };
  xhr.send(body ? JSON.stringify(body) : null);
}

function loadTab(tab) {
  var el = document.getElementById('admin-content');
  if (!el) return;
  el.innerHTML = '<div style="color:#64748b;padding:1rem">Loading...</div>';
  if (tab === 'users') {
    api('GET', '/users', null, function(err, data) {
      if (err) { el.innerHTML = '<div class="admin-msg admin-msg-err">' + esc(err) + '</div>'; return; }
      adminData.users = data;
      renderUsers(el);
    });
  } else if (tab === 'services') {
    api('GET', '/services', null, function(err, data) {
      if (err) { el.innerHTML = '<div class="admin-msg admin-msg-err">' + esc(err) + '</div>'; return; }
      adminData.services = data;
      renderServices(el);
    });
  } else if (tab === 'access') {
    api('GET', '/users', null, function(err1, users) {
      if (err1) { el.innerHTML = '<div class="admin-msg admin-msg-err">' + esc(err1) + '</div>'; return; }
      adminData.users = users;
      api('GET', '/services', null, function(err2, services) {
        if (err2) { el.innerHTML = '<div class="admin-msg admin-msg-err">' + esc(err2) + '</div>'; return; }
        adminData.services = services;
        api('GET', '/grants', null, function(err3, grants) {
          if (err3) { el.innerHTML = '<div class="admin-msg admin-msg-err">' + esc(err3) + '</div>'; return; }
          adminData.grants = grants;
          renderAccess(el);
        });
      });
    });
  }
}

function esc(s) {
  var d = document.createElement('div');
  d.textContent = s || '';
  return d.innerHTML;
}

function renderUsers(el) {
  var html = '<table class="admin-tbl"><thead><tr><th style="width:30px"></th><th>Handle</th><th>Username</th><th>Role</th><th>DID</th></tr></thead><tbody>';
  for (var i = 0; i < adminData.users.length; i++) {
    var u = adminData.users[i];
    var canChangeRole = ROLE === 'owner';
    var radio = '<input type="radio" name="sel-user" value="' + u.id + '" style="cursor:pointer;accent-color:#3b82f6" onchange="selectUser(' + u.id + ')">';
    var usernameCell = '<input class="admin-input" style="width:90px;font-size:0.75rem" value="' + esc(u.username || '') + '" onchange="updateUsername(' + u.id + ',this.value)">';
    var roleCell = canChangeRole
      ? '<select class="admin-select" onchange="updateRole(' + u.id + ',this.value)">' +
        '<option value="user"' + (u.role==='user'?' selected':'') + '>User</option>' +
        '<option value="admin"' + (u.role==='admin'?' selected':'') + '>Admin</option>' +
        '<option value="owner"' + (u.role==='owner'?' selected':'') + '>Owner</option></select>'
      : esc(u.role);
    html += '<tr><td>' + radio + '</td><td>' + esc(u.handle || '(no handle)') + '</td><td>' + usernameCell + '</td><td>' + roleCell + '</td><td style="font-size:0.75rem;color:#64748b;max-width:200px;overflow:hidden;text-overflow:ellipsis">' + esc(u.did) + '</td></tr>';
  }
  html += '</tbody></table>';
  html += '<div class="admin-form">' +
    '<input class="admin-input" id="add-handle" placeholder="handle" style="flex:1;min-width:150px" oninput="checkAddUser()">' +
    '<input class="admin-input" id="add-username" placeholder="username" style="width:90px" oninput="checkAddUser()">' +
    '<select class="admin-select" id="add-role" onchange="checkAddUser()"><option value="" disabled selected>role</option><option value="user">User</option>` + ownerOnly + `</select>' +
    '<button class="admin-btn" id="add-user-btn" onclick="addUser()" disabled style="opacity:0.4;cursor:default">Add</button>' +
    '<button class="admin-btn-danger" id="del-user-btn" onclick="deleteSelectedUser()" disabled style="opacity:0.4;cursor:default;padding:0.375rem 0.75rem;font-size:0.8125rem">Delete</button></div>';
  html += '<div id="users-msg"></div>';
  el.innerHTML = html;
  if (selectedUserId) {
    var found = false;
    var radios = el.querySelectorAll('input[name="sel-user"]');
    for (var j = 0; j < radios.length; j++) {
      if (parseInt(radios[j].value) === selectedUserId) {
        radios[j].checked = true;
        found = true;
        break;
      }
    }
    if (found) {
      var btn = document.getElementById('del-user-btn');
      btn.disabled = false;
      btn.style.opacity = '1';
      btn.style.cursor = 'pointer';
    } else {
      selectedUserId = 0;
      selectedUserGrants = {};
      updateGrantDots();
    }
  }
}

function checkAddUser() {
  var h = document.getElementById('add-handle').value.trim();
  var u = document.getElementById('add-username').value.trim();
  var r = document.getElementById('add-role').value;
  var btn = document.getElementById('add-user-btn');
  if (h && u && r) {
    btn.disabled = false;
    btn.style.opacity = '1';
    btn.style.cursor = 'pointer';
  } else {
    btn.disabled = true;
    btn.style.opacity = '0.4';
    btn.style.cursor = 'default';
  }
}

function addUser() {
  var handle = document.getElementById('add-handle').value.trim();
  var username = document.getElementById('add-username').value.trim();
  var role = document.getElementById('add-role').value;
  var msg = document.getElementById('users-msg');
  if (!handle || !username || !role) return;
  api('POST', '/users', { handle: handle, role: role, username: username }, function(err) {
    if (err) { msg.className = 'admin-msg admin-msg-err'; msg.textContent = err; return; }
    document.getElementById('add-handle').value = '';
    document.getElementById('add-username').value = '';
    document.getElementById('add-role').value = '';
    checkAddUser();
    msg.className = 'admin-msg admin-msg-ok'; msg.textContent = 'User added';
    loadTab('users');
  });
}

function updateUsername(id, username) {
  var msg = document.getElementById('users-msg');
  api('PUT', '/users/' + id + '/username', { username: username }, function(err) {
    if (err) { msg.className = 'admin-msg admin-msg-err'; msg.textContent = err; loadTab('users'); return; }
    msg.className = 'admin-msg admin-msg-ok'; msg.textContent = 'Username updated';
    setTimeout(function() { msg.className = ''; msg.textContent = ''; }, 1500);
  });
}

function updateRole(id, role) {
  api('PUT', '/users/' + id + '/role', { role: role }, function(err) {
    if (err) { alert(err); loadTab('users'); }
  });
}

var selectedUserId = 0;
var selectedUserGrants = {};

function selectUser(userId) {
  selectedUserId = userId;
  var btn = document.getElementById('del-user-btn');
  if (btn) {
    btn.disabled = false;
    btn.style.opacity = '1';
    btn.style.cursor = 'pointer';
  }
  api('GET', '/grants', null, function(err, grants) {
    if (err) return;
    selectedUserGrants = {};
    for (var i = 0; i < grants.length; i++) {
      if (grants[i].user_id === userId) {
        selectedUserGrants[grants[i].service_id] = grants[i];
      }
    }
    updateGrantDots();
  });
}

function updateGrantDots() {
  var cards = document.querySelectorAll('.card[data-svc-id]');
  for (var i = 0; i < cards.length; i++) {
    var card = cards[i];
    var svcId = parseInt(card.getAttribute('data-svc-id'));
    var dot = card.querySelector('.grant-dot');
    if (!dot) continue;
    if (!selectedUserId) {
      dot.style.display = 'none';
      dot.onclick = null;
      continue;
    }
    dot.style.display = 'block';
    if (selectedUserGrants[svcId]) {
      dot.className = 'grant-dot granted';
    } else {
      dot.className = 'grant-dot revoked';
    }
    (function(sid) {
      dot.onclick = function(e) {
        e.preventDefault();
        e.stopPropagation();
        toggleCardGrant(sid);
      };
    })(svcId);
  }
}

function toggleCardGrant(svcId) {
  if (!selectedUserId) return;
  var grant = selectedUserGrants[svcId];
  if (grant) {
    api('DELETE', '/grants/' + grant.id, null, function(err) {
      if (err) { alert(err); return; }
      delete selectedUserGrants[svcId];
      var card = document.querySelector('.card[data-svc-id="' + svcId + '"]');
      if (card && card.target && typeof closeTrackedWindow === 'function') {
        closeTrackedWindow(card.target);
      }
      updateGrantDots();
    });
  } else {
    api('POST', '/grants', { user_id: selectedUserId, service_id: svcId, role: 'user' }, function(err) {
      if (err) { alert(err); return; }
      api('GET', '/grants', null, function(err2, grants) {
        if (err2) return;
        selectedUserGrants = {};
        for (var i = 0; i < grants.length; i++) {
          if (grants[i].user_id === selectedUserId) {
            selectedUserGrants[grants[i].service_id] = grants[i];
          }
        }
        updateGrantDots();
      });
    });
  }
}

function deleteSelectedUser() {
  if (!selectedUserId) return;
  if (!confirm('Delete this user?')) return;
  api('DELETE', '/users/' + selectedUserId, null, function(err) {
    if (err) { alert(err); return; }
    selectedUserId = 0;
    selectedUserGrants = {};
    updateGrantDots();
    loadTab('users');
  });
}

function renderServices(el) {
  var html = '<table class="admin-tbl"><thead><tr><th>Name</th><th>Slug</th><th>URL</th><th>Admin Role</th><th></th></tr></thead><tbody>';
  for (var i = 0; i < adminData.services.length; i++) {
    var s = adminData.services[i];
    html += '<tr><td>' + esc(s.name) + '</td><td style="color:#64748b">' + esc(s.slug) + '</td><td style="font-size:0.75rem;color:#64748b">' + esc(s.url) + '</td>' +
      '<td><input class="admin-input" style="width:70px;font-size:0.75rem" value="' + esc(s.admin_role) + '" onchange="updateServiceAdminRole(' + s.id + ',this.value)"></td>' +
      '<td><button class="admin-btn-danger" onclick="deleteService(' + s.id + ')">Delete</button></td></tr>';
  }
  html += '</tbody></table>';
  html += '<div class="admin-form">' +
    '<input class="admin-input" id="svc-name" placeholder="name" style="width:100px" oninput="checkAddService()">' +
    '<input class="admin-input" id="svc-slug" placeholder="slug" style="width:80px" oninput="checkAddService()">' +
    '<input class="admin-input" id="svc-url" placeholder="https://..." style="flex:1;min-width:130px" oninput="checkAddService()">' +
    '<input class="admin-input" id="svc-desc" placeholder="description" style="width:110px">' +
    '<input class="admin-input" id="svc-admin-role" placeholder="admin" style="width:70px">' +
    '<button class="admin-btn" id="add-svc-btn" onclick="addService()" disabled style="opacity:0.4;cursor:default">Add</button></div>';
  html += '<div id="services-msg"></div>';
  el.innerHTML = html;
}

function checkAddService() {
  var n = document.getElementById('svc-name').value.trim();
  var s = document.getElementById('svc-slug').value.trim();
  var u = document.getElementById('svc-url').value.trim();
  var btn = document.getElementById('add-svc-btn');
  if (n && s && u) {
    btn.disabled = false;
    btn.style.opacity = '1';
    btn.style.cursor = 'pointer';
  } else {
    btn.disabled = true;
    btn.style.opacity = '0.4';
    btn.style.cursor = 'default';
  }
}

function addService() {
  var name = document.getElementById('svc-name').value.trim();
  var slug = document.getElementById('svc-slug').value.trim();
  var url = document.getElementById('svc-url').value.trim();
  var desc = document.getElementById('svc-desc').value.trim();
  var adminRole = document.getElementById('svc-admin-role').value.trim() || 'admin';
  var msg = document.getElementById('services-msg');
  if (!name || !slug || !url) { msg.className = 'admin-msg admin-msg-err'; msg.textContent = 'Name, slug, and URL required'; return; }
  api('POST', '/services', { name: name, slug: slug, url: url, description: desc, icon_url: '', admin_role: adminRole }, function(err) {
    if (err) { msg.className = 'admin-msg admin-msg-err'; msg.textContent = err; return; }
    document.getElementById('svc-name').value = '';
    document.getElementById('svc-slug').value = '';
    document.getElementById('svc-url').value = '';
    document.getElementById('svc-desc').value = '';
    document.getElementById('svc-admin-role').value = '';
    checkAddService();
    msg.className = 'admin-msg admin-msg-ok'; msg.textContent = 'Service added';
    loadTab('services');
  });
}

function updateServiceAdminRole(id, adminRole) {
  var svc = null;
  for (var i = 0; i < adminData.services.length; i++) {
    if (adminData.services[i].id === id) { svc = adminData.services[i]; break; }
  }
  if (!svc) return;
  var msg = document.getElementById('services-msg');
  api('PUT', '/services/' + id, { name: svc.name, description: svc.description, url: svc.url, icon_url: svc.icon_url, admin_role: adminRole }, function(err) {
    if (err) { msg.className = 'admin-msg admin-msg-err'; msg.textContent = err; return; }
    svc.admin_role = adminRole;
    msg.className = 'admin-msg admin-msg-ok'; msg.textContent = 'Admin role updated';
    setTimeout(function() { msg.className = ''; msg.textContent = ''; }, 1500);
  });
}

function deleteService(id) {
  if (!confirm('Delete this service? Grants will also be removed.')) return;
  api('DELETE', '/services/' + id, null, function(err) {
    if (err) { alert(err); return; }
    loadTab('services');
  });
}

function renderAccess(el) {
  var users = adminData.users;
  var services = adminData.services;
  var grantMap = {};
  for (var i = 0; i < adminData.grants.length; i++) {
    var g = adminData.grants[i];
    grantMap[g.user_id + ':' + g.service_id] = g;
  }

  var html = '<table class="admin-tbl"><thead><tr><th>User</th>';
  for (var i = 0; i < services.length; i++) {
    html += '<th style="text-align:center;font-size:0.75rem">' + esc(services[i].name) + '</th>';
  }
  html += '</tr></thead><tbody>';
  for (var i = 0; i < users.length; i++) {
    var u = users[i];
    html += '<tr><td>' + esc(u.handle || u.did) + '</td>';
    for (var j = 0; j < services.length; j++) {
      var s = services[j];
      var key = u.id + ':' + s.id;
      var grant = grantMap[key];
      var checked = grant ? ' checked' : '';
      var role = grant ? grant.role : 'user';
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

function toggleGrant(userId, serviceId, checked) {
  var msg = document.getElementById('access-msg');
  if (checked) {
    api('POST', '/grants', { user_id: userId, service_id: serviceId, role: 'user' }, function(err) {
      if (err) { msg.className = 'admin-msg admin-msg-err'; msg.textContent = err; loadTab('access'); return; }
      api('GET', '/grants', null, function(err2, grants) {
        if (!err2) adminData.grants = grants;
        renderAccess(document.getElementById('admin-content'));
      });
    });
  } else {
    var grant = null;
    for (var i = 0; i < adminData.grants.length; i++) {
      var g = adminData.grants[i];
      if (g.user_id === userId && g.service_id === serviceId) { grant = g; break; }
    }
    if (grant) {
      api('DELETE', '/grants/' + grant.id, null, function(err) {
        if (err) { msg.className = 'admin-msg admin-msg-err'; msg.textContent = err; loadTab('access'); return; }
        api('GET', '/grants', null, function(err2, grants) {
          if (!err2) adminData.grants = grants;
          renderAccess(document.getElementById('admin-content'));
        });
      });
    }
  }
}

function updateGrantRole(userId, serviceId, role) {
  var msg = document.getElementById('access-msg');
  api('POST', '/grants', { user_id: userId, service_id: serviceId, role: role }, function(err) {
    if (err) { msg.className = 'admin-msg admin-msg-err'; msg.textContent = err; return; }
    api('GET', '/grants', null, function(err2, grants) {
      if (!err2) adminData.grants = grants;
      msg.className = 'admin-msg admin-msg-ok'; msg.textContent = 'Role updated';
      setTimeout(function() { msg.className = ''; msg.textContent = ''; }, 1500);
    });
  });
}

` + autoLoad + `
</script>`
}
