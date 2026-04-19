package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ── API Response helpers ─────────────────────────────────────────────────────

func jsonOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "data": data})
}

func jsonErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": msg})
}

// ── API Handlers ─────────────────────────────────────────────────────────────

func handleRoutesAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	sub := strings.TrimPrefix(r.URL.Path, "/api/routes")
	sub = strings.TrimPrefix(sub, "/")

	switch r.Method {
	case http.MethodGet:
		configMu.RLock()
		routes := appConfig.Routes
		configMu.RUnlock()
		jsonOK(w, routes)

	case http.MethodPost:
		var newRoute Route
		if err := json.NewDecoder(r.Body).Decode(&newRoute); err != nil {
			jsonErr(w, "Invalid JSON: "+err.Error(), 400)
			return
		}
		if newRoute.Path == "" || newRoute.Target == "" {
			jsonErr(w, "path and target are required", 400)
			return
		}
		if !strings.HasPrefix(newRoute.Path, "/") {
			newRoute.Path = "/" + newRoute.Path
		}
		configMu.Lock()
		for _, existing := range appConfig.Routes {
			if existing.Path == newRoute.Path {
				configMu.Unlock()
				jsonErr(w, fmt.Sprintf("Route %q already exists", newRoute.Path), 409)
				return
			}
		}
		appConfig.Routes = append(appConfig.Routes, newRoute)
		configMu.Unlock()
		if err := saveConfig(); err != nil {
			jsonErr(w, "Save failed: "+err.Error(), 500)
			return
		}
		rebuildMux()
		jsonOK(w, newRoute)

	case http.MethodPut:
		var updated Route
		if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
			jsonErr(w, "Invalid JSON: "+err.Error(), 400)
			return
		}
		targetPath := "/" + sub
		configMu.Lock()
		found := false
		for i, existing := range appConfig.Routes {
			if existing.Path == targetPath {
				appConfig.Routes[i] = updated
				found = true
				break
			}
		}
		configMu.Unlock()
		if !found {
			jsonErr(w, "Route not found: "+targetPath, 404)
			return
		}
		if err := saveConfig(); err != nil {
			jsonErr(w, "Save failed: "+err.Error(), 500)
			return
		}
		rebuildMux()
		jsonOK(w, updated)

	case http.MethodDelete:
		targetPath := "/" + sub
		configMu.Lock()
		newRoutes := []Route{}
		found := false
		for _, existing := range appConfig.Routes {
			if existing.Path == targetPath {
				found = true
			} else {
				newRoutes = append(newRoutes, existing)
			}
		}
		appConfig.Routes = newRoutes
		configMu.Unlock()
		if !found {
			jsonErr(w, "Route not found: "+targetPath, 404)
			return
		}
		if err := saveConfig(); err != nil {
			jsonErr(w, "Save failed: "+err.Error(), 500)
			return
		}
		rebuildMux()
		jsonOK(w, map[string]string{"deleted": targetPath})

	default:
		jsonErr(w, "Method not allowed", 405)
	}
}

func handleNginxApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErr(w, "Method not allowed", 405)
		return
	}
	if err := GenerateNginxSnippet(); err != nil {
		jsonErr(w, "Generate snippet failed: "+err.Error(), 500)
		return
	}
	msg, reloadErr := ReloadNginx()
	status := "ok"
	if reloadErr != nil {
		status = "manual_required"
	}
	jsonOK(w, map[string]string{"status": status, "message": msg})
}

func handleNginxSnippet(w http.ResponseWriter, r *http.Request) {
	snippet, err := GetNginxSnippet()
	if err != nil {
		jsonErr(w, err.Error(), 500)
		return
	}
	jsonOK(w, map[string]string{"snippet": snippet})
}

func handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErr(w, "Method not allowed", 405)
		return
	}
	if err := loadConfig(); err != nil {
		jsonErr(w, "Reload config failed: "+err.Error(), 500)
		return
	}
	rebuildMux()
	jsonOK(w, map[string]string{"message": "Proxy routes reloaded"})
}

// ── Admin UI ─────────────────────────────────────────────────────────────────

func handleAdminUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, adminHTML)
}

var adminHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>HandlerPortProxy — Dashboard</title>
<style>
  :root {
    --bg:#0f1117;--bg2:#1a1d27;--bg3:#252836;--border:#2e3147;
    --accent:#6c63ff;--accent2:#4ecdc4;--danger:#ff4d6d;
    --success:#2dd4bf;--warn:#fbbf24;--text:#e2e8f0;--muted:#64748b;
    --font:'SF Pro Display',-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;
    --mono:'SF Mono','Fira Code',monospace;
  }
  *{box-sizing:border-box;margin:0;padding:0}
  body{background:var(--bg);color:var(--text);font-family:var(--font);min-height:100vh}
  .topbar{background:var(--bg2);border-bottom:1px solid var(--border);padding:0 24px;
    display:flex;align-items:center;height:56px;gap:12px}
  .topbar-logo{display:flex;align-items:center;gap:10px;font-weight:700;font-size:15px;
    color:var(--text);text-decoration:none}
  .logo-icon{width:32px;height:32px;background:var(--accent);border-radius:8px;
    display:flex;align-items:center;justify-content:center;font-size:16px}
  .topbar-status{display:flex;align-items:center;gap:6px;font-size:12px;color:var(--success)}
  .dot{width:7px;height:7px;background:var(--success);border-radius:50%;animation:pulse 2s infinite}
  @keyframes pulse{0%,100%{opacity:1}50%{opacity:.4}}
  .topbar-sub{font-size:12px;color:var(--muted);margin-left:auto;display:flex;align-items:center;gap:16px}
  .logout-link{color:var(--danger);text-decoration:none;font-weight:600;display:flex;align-items:center;gap:4px}
  .logout-link:hover{opacity:0.8}
  .container{max-width:960px;margin:0 auto;padding:32px 24px}
  .card{background:var(--bg2);border:1px solid var(--border);border-radius:12px;
    padding:20px;margin-bottom:20px}
  .card-header{display:flex;align-items:center;justify-content:space-between;margin-bottom:16px}
  .card-title{font-size:14px;font-weight:600;color:var(--text);display:flex;align-items:center;gap:8px}
  .badge{background:var(--bg3);border:1px solid var(--border);color:var(--muted);
    font-size:12px;font-weight:400;padding:2px 8px;border-radius:20px}
  .route-list{display:flex;flex-direction:column;gap:8px}
  .route-item{background:var(--bg3);border:1px solid var(--border);border-radius:8px;
    padding:12px 16px;display:flex;align-items:center;gap:12px;transition:border-color 0.2s}
  .route-item:hover{border-color:var(--accent)}
  .route-item.disabled{opacity:0.5}
  .route-path{font-family:var(--mono);font-size:13px;color:var(--accent2);
    background:rgba(78,205,196,0.08);padding:3px 8px;border-radius:4px;min-width:130px}
  .route-arrow{color:var(--muted);font-size:14px}
  .route-target{font-family:var(--mono);font-size:12px;color:var(--muted);flex:1}
  .route-name{font-size:13px;color:var(--text);font-weight:500;min-width:110px}
  .route-desc{font-size:12px;color:var(--muted);flex:1}
  .route-actions{display:flex;gap:6px;margin-left:auto}
  .btn{display:inline-flex;align-items:center;gap:6px;padding:6px 14px;border-radius:6px;
    font-size:13px;font-weight:500;cursor:pointer;border:1px solid transparent;
    transition:all 0.15s;text-decoration:none;background:none}
  .btn:disabled{opacity:0.5;cursor:not-allowed}
  .btn-primary{background:var(--accent);color:#fff}
  .btn-primary:hover:not(:disabled){background:#7c74ff}
  .btn-success{border-color:var(--success);color:var(--success)}
  .btn-success:hover:not(:disabled){background:rgba(45,212,191,0.1)}
  .btn-danger{border-color:var(--danger);color:var(--danger)}
  .btn-danger:hover:not(:disabled){background:rgba(255,77,109,0.1)}
  .btn-muted{background:var(--bg3);border-color:var(--border);color:var(--muted)}
  .btn-muted:hover:not(:disabled){border-color:var(--text);color:var(--text)}
  .btn-sm{padding:4px 10px;font-size:12px}
  .btn-lg{padding:10px 20px;font-size:14px}
  .toggle{position:relative;display:inline-block;width:36px;height:20px;cursor:pointer}
  .toggle input{display:none}
  .toggle-slider{position:absolute;inset:0;background:var(--bg);border:1px solid var(--border);
    border-radius:20px;transition:0.2s}
  .toggle-slider:before{content:'';position:absolute;width:14px;height:14px;background:var(--muted);
    border-radius:50%;top:2px;left:2px;transition:0.2s}
  .toggle input:checked + .toggle-slider{background:var(--accent);border-color:var(--accent)}
  .toggle input:checked + .toggle-slider:before{background:#fff;transform:translateX(16px)}
  .form-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}
  .form-group{display:flex;flex-direction:column;gap:6px}
  .form-group.full{grid-column:1/-1}
  label{font-size:12px;color:var(--muted);font-weight:500}
  input{background:var(--bg);border:1px solid var(--border);color:var(--text);
    border-radius:6px;padding:8px 12px;font-size:13px;font-family:var(--font);
    transition:border-color 0.15s;outline:none;width:100%}
  input:focus{border-color:var(--accent)}
  input::placeholder{color:var(--muted)}
  .input-hint{font-size:11px;color:var(--muted)}
  #toast{position:fixed;bottom:24px;right:24px;max-width:420px;z-index:9999;
    display:flex;flex-direction:column;gap:8px}
  .toast-item{background:var(--bg2);border:1px solid var(--border);border-radius:8px;
    padding:12px 16px;font-size:13px;box-shadow:0 8px 24px rgba(0,0,0,.4);
    animation:slideIn 0.2s ease;display:flex;gap:10px;align-items:flex-start}
  .toast-item.success{border-left:3px solid var(--success)}
  .toast-item.error{border-left:3px solid var(--danger)}
  .toast-item.warn{border-left:3px solid var(--warn)}
  @keyframes slideIn{from{transform:translateX(20px);opacity:0}}
  .code-block{background:var(--bg);border:1px solid var(--border);border-radius:8px;
    padding:16px;font-family:var(--mono);font-size:12px;line-height:1.6;color:#94a3b8;
    white-space:pre;overflow-x:auto;max-height:340px;overflow-y:auto}
  .modal-overlay{display:none;position:fixed;inset:0;background:rgba(0,0,0,.7);
    z-index:100;align-items:center;justify-content:center}
  .modal-overlay.open{display:flex}
  .modal{background:var(--bg2);border:1px solid var(--border);border-radius:12px;
    width:640px;max-width:95vw;max-height:90vh;display:flex;flex-direction:column}
  .modal-header{padding:16px 20px;border-bottom:1px solid var(--border);
    display:flex;align-items:center;justify-content:space-between;font-weight:600;font-size:15px}
  .modal-body{padding:20px;overflow-y:auto;flex:1}
  .modal-footer{padding:12px 20px;border-top:1px solid var(--border);
    display:flex;justify-content:flex-end;gap:8px}
  .setup-box{background:rgba(108,99,255,.08);border:1px solid rgba(108,99,255,.3);
    border-radius:8px;padding:14px 16px}
  .setup-box h4{font-size:13px;color:var(--accent);margin-bottom:8px;
    display:flex;align-items:center;gap:6px}
  .setup-box p{font-size:12px;color:var(--muted);line-height:1.6;margin-bottom:6px}
  .setup-box code{font-family:var(--mono);background:var(--bg);
    padding:2px 6px;border-radius:4px;font-size:11px;color:var(--accent2)}
  .flex{display:flex}.gap-8{gap:8px}.ml-auto{margin-left:auto}
  .empty-state{text-align:center;padding:32px;color:var(--muted);font-size:14px}
  .tag{font-size:11px;padding:2px 7px;border-radius:4px;font-weight:500}
  .tag-on{background:rgba(45,212,191,.1);color:var(--success)}
  .tag-off{background:rgba(100,116,139,.1);color:var(--muted)}
  hr{border:none;border-top:1px solid var(--border);margin:16px 0}
  p{font-size:13px;color:var(--muted);margin-bottom:14px}
</style>
</head>
<body>

<div class="topbar">
  <a class="topbar-logo" href="/adminwebui">
    <div class="logo-icon">&#9889;</div>
    HandlerPortProxy
  </a>
  <div class="topbar-status">
    <div class="dot"></div>
    Running on port <strong id="serverPort">...</strong>
  </div>
  <div class="topbar-sub">
    <span id="routeCount">Loading...</span>
    <a href="/logout" class="logout-link">&#128682; Logout</a>
  </div>
</div>

<div class="container">
  <div class="card">
    <div class="card-header">
      <div class="card-title">&#128256; Active Routes <span id="routeBadge" class="badge">0</span></div>
      <div class="flex gap-8">
        <button class="btn btn-muted btn-sm" onclick="showNginxModal()">&#128203; Lihat Nginx Config</button>
        <button class="btn btn-primary btn-sm" onclick="showAddModal()">+ Tambah Route</button>
      </div>
    </div>
    <div class="route-list" id="routeList">
      <div class="empty-state">Loading routes...</div>
    </div>
  </div>

  <div class="card">
    <div class="card-header">
      <div class="card-title">&#128295; Apply ke Nginx</div>
    </div>
    <p>Generate nginx location blocks dari routes di atas, lalu reload nginx otomatis.</p>
    <div class="flex gap-8" style="flex-wrap:wrap">
      <button class="btn btn-success btn-lg" onclick="applyNginx()" id="applyBtn">
        &#9889; Generate &amp; Reload Nginx
      </button>
      <button class="btn btn-muted" onclick="showNginxModal()">&#128065; Preview Config</button>
    </div>
    <div id="applyResult" style="margin-top:14px;display:none"></div>
    <hr>
    <div class="setup-box">
      <h4>&#128204; One-time Setup (pertama kali saja)</h4>
      <p>Tambahkan baris ini ke dalam <code>server { }</code> block di <code>homeserver.conf</code>,
         letakkan <strong>sebelum</strong> <code>location /</code>:</p>
      <div class="code-block">    include /home/mogagacor/Project/handlerportproxy/portproxy-routes.conf;</div>
      <p style="margin-top:8px">Setelah itu semua perubahan route cukup klik <em>Generate &amp; Reload</em>.</p>
      <p>Untuk allow nginx reload tanpa password (opsional):</p>
      <div class="code-block">echo "mogagacor ALL=(ALL) NOPASSWD: /usr/sbin/nginx -s reload" | sudo tee /etc/sudoers.d/nginx-reload</div>
    </div>
  </div>
</div>

<!-- Add/Edit Modal -->
<div class="modal-overlay" id="routeModal">
  <div class="modal">
    <div class="modal-header">
      <span id="modalTitle">Tambah Route Baru</span>
      <button class="btn btn-muted btn-sm" onclick="closeModal('routeModal')">&#x2715;</button>
    </div>
    <div class="modal-body">
      <div class="form-grid">
        <div class="form-group">
          <label>Nama Project</label>
          <input type="text" id="f_name" placeholder="My Next.js App">
        </div>
        <div class="form-group">
          <label>Path (URL prefix)</label>
          <input type="text" id="f_path" placeholder="/myproject">
          <span class="input-hint">Contoh: /project1, /dashboard</span>
        </div>
        <div class="form-group">
          <label>Target (upstream URL)</label>
          <input type="text" id="f_target" placeholder="http://localhost:3004">
          <span class="input-hint">Port lokal tempat app berjalan</span>
        </div>
        <div class="form-group">
          <label>Deskripsi (opsional)</label>
          <input type="text" id="f_desc" placeholder="Next.js project untuk ...">
        </div>
        <div class="form-group full" style="flex-direction:row;align-items:center;gap:10px">
          <label class="toggle">
            <input type="checkbox" id="f_enabled" checked>
            <span class="toggle-slider"></span>
          </label>
          <span style="font-size:13px">Aktifkan route ini</span>
        </div>
      </div>
      <div id="formError" style="color:var(--danger);font-size:12px;margin-top:10px;display:none"></div>
    </div>
    <div class="modal-footer">
      <button class="btn btn-muted" onclick="closeModal('routeModal')">Batal</button>
      <button class="btn btn-primary" onclick="saveRoute()" id="saveRouteBtn">Simpan Route</button>
    </div>
  </div>
</div>

<!-- Nginx snippet modal -->
<div class="modal-overlay" id="nginxModal">
  <div class="modal">
    <div class="modal-header">
      <span>&#128203; Nginx Config Preview</span>
      <button class="btn btn-muted btn-sm" onclick="closeModal('nginxModal')">&#x2715;</button>
    </div>
    <div class="modal-body">
      <p>File: <code>/home/mogagacor/Project/handlerportproxy/portproxy-routes.conf</code></p>
      <pre class="code-block" id="nginxSnippetContent">Loading...</pre>
    </div>
    <div class="modal-footer">
      <button class="btn btn-muted" onclick="copySnippet()">&#128203; Copy</button>
      <button class="btn btn-primary" onclick="applyNginx();closeModal('nginxModal')">&#9889; Apply Sekarang</button>
    </div>
  </div>
</div>

<div id="toast"></div>

<script>
var allRoutes = [];
var editingPath = null;

function init() {
  fetch('/api/routes').then(function(r){ return r.json(); }).then(function(json){
    if (json.ok) {
      allRoutes = json.data || [];
      renderRoutes();
      document.getElementById('serverPort').textContent = location.port || '80';
      var active = allRoutes.filter(function(r){ return r.enabled; }).length;
      document.getElementById('routeCount').textContent = active + ' active / ' + allRoutes.length + ' total routes';
    }
  }).catch(function(e){ toast('Gagal load routes: ' + e.message, 'error'); });
}

function renderRoutes() {
  var list = document.getElementById('routeList');
  document.getElementById('routeBadge').textContent = allRoutes.length;
  if (allRoutes.length === 0) {
    list.innerHTML = '<div class="empty-state">Belum ada route. Klik <strong>+ Tambah Route</strong> untuk mulai.</div>';
    return;
  }
  var html = '';
  for (var i = 0; i < allRoutes.length; i++) {
    var r = allRoutes[i];
    var encodedPath = encodeURIComponent(r.path);
    html += '<div class="route-item ' + (r.enabled ? '' : 'disabled') + '" id="route-' + encodedPath + '">';
    html += '<span class="tag ' + (r.enabled ? 'tag-on' : 'tag-off') + '">' + (r.enabled ? 'ON' : 'OFF') + '</span>';
    html += '<span class="route-name">' + esc(r.name || r.path) + '</span>';
    html += '<span class="route-path">' + esc(r.path) + '/</span>';
    html += '<span class="route-arrow">&#8594;</span>';
    html += '<span class="route-target">' + esc(r.target) + '</span>';
    if (r.description) { html += '<span class="route-desc">' + esc(r.description) + '</span>'; }
    html += '<div class="route-actions">';
    html += '<button class="btn btn-muted btn-sm" onclick="editRoute(\'' + esc(r.path) + '\')">&#9998; Edit</button>';
    html += '<button class="btn btn-danger btn-sm" onclick="deleteRoute(\'' + esc(r.path) + '\')">&#128465;</button>';
    html += '</div></div>';
  }
  list.innerHTML = html;
}

function showAddModal() {
  editingPath = null;
  document.getElementById('modalTitle').textContent = 'Tambah Route Baru';
  document.getElementById('f_name').value = '';
  document.getElementById('f_path').value = '';
  document.getElementById('f_target').value = 'http://localhost:';
  document.getElementById('f_desc').value = '';
  document.getElementById('f_enabled').checked = true;
  document.getElementById('formError').style.display = 'none';
  document.getElementById('saveRouteBtn').textContent = 'Simpan Route';
  openModal('routeModal');
  setTimeout(function(){ document.getElementById('f_name').focus(); }, 100);
}

function editRoute(path) {
  var route = null;
  for (var i = 0; i < allRoutes.length; i++) {
    if (allRoutes[i].path === path) { route = allRoutes[i]; break; }
  }
  if (!route) return;
  editingPath = path;
  document.getElementById('modalTitle').textContent = 'Edit Route';
  document.getElementById('f_name').value = route.name || '';
  document.getElementById('f_path').value = route.path;
  document.getElementById('f_target').value = route.target;
  document.getElementById('f_desc').value = route.description || '';
  document.getElementById('f_enabled').checked = route.enabled !== false;
  document.getElementById('formError').style.display = 'none';
  document.getElementById('saveRouteBtn').textContent = 'Update Route';
  openModal('routeModal');
}

function saveRoute() {
  var name = document.getElementById('f_name').value.trim();
  var path = document.getElementById('f_path').value.trim();
  var target = document.getElementById('f_target').value.trim();
  var desc = document.getElementById('f_desc').value.trim();
  var enabled = document.getElementById('f_enabled').checked;
  var errEl = document.getElementById('formError');
  if (!path || !target) {
    errEl.textContent = 'Path dan Target wajib diisi!';
    errEl.style.display = 'block';
    return;
  }
  if (path.charAt(0) !== '/') path = '/' + path;
  var btn = document.getElementById('saveRouteBtn');
  btn.disabled = true;
  btn.textContent = 'Menyimpan...';
  var body = JSON.stringify({name:name,path:path,target:target,description:desc,enabled:enabled});
  var url = editingPath ? '/api/routes' + editingPath : '/api/routes';
  var method = editingPath ? 'PUT' : 'POST';
  fetch(url, {method:method,headers:{'Content-Type':'application/json'},body:body})
    .then(function(r){ return r.json(); })
    .then(function(json){
      if (!json.ok) {
        errEl.textContent = json.error || 'Gagal menyimpan';
        errEl.style.display = 'block';
        btn.disabled = false;
        btn.textContent = editingPath ? 'Update Route' : 'Simpan Route';
        return;
      }
      closeModal('routeModal');
      toast((editingPath ? 'Route diupdate' : 'Route ditambahkan') + ' \u2713', 'success');
      init();
    })
    .catch(function(e){
      errEl.textContent = e.message;
      errEl.style.display = 'block';
      btn.disabled = false;
      btn.textContent = editingPath ? 'Update Route' : 'Simpan Route';
    });
}

function deleteRoute(path) {
  if (!confirm('Hapus route ' + path + '?')) return;
  fetch('/api/routes' + path, {method:'DELETE'})
    .then(function(r){ return r.json(); })
    .then(function(json){
      if (json.ok) { toast('Route dihapus \u2713', 'success'); init(); }
      else { toast('Gagal hapus: ' + json.error, 'error'); }
    })
    .catch(function(e){ toast('Error: ' + e.message, 'error'); });
}

function applyNginx() {
  var btn = document.getElementById('applyBtn');
  var result = document.getElementById('applyResult');
  btn.disabled = true;
  btn.textContent = 'Applying...';
  result.style.display = 'none';
  fetch('/api/nginx/apply', {method:'POST'})
    .then(function(r){ return r.json(); })
    .then(function(json){
      result.style.display = 'block';
      if (json.ok) {
        var d = json.data;
        var isOk = d.status === 'ok';
        var color = isOk ? 'var(--success)' : 'var(--warn)';
        var icon = isOk ? '\u2705' : '\u26a0\ufe0f';
        result.innerHTML = '<div style="background:var(--bg3);border:1px solid ' + color +
          ';border-radius:8px;padding:12px 14px"><div style="font-size:13px;color:' + color + '">' +
          icon + ' ' + esc(d.message) + '</div>' +
          (!isOk ? '<hr><div style="font-size:12px;color:var(--muted)">Setup sudoers untuk auto-reload (lihat panduan di bawah).</div>' : '') +
          '</div>';
        toast(isOk ? 'Nginx berhasil di-reload! \u2713' : 'Snippet di-generate, reload manual diperlukan', isOk ? 'success' : 'warn');
      }
    })
    .catch(function(e){ toast('Error: ' + e.message, 'error'); })
    .finally(function(){ btn.disabled = false; btn.textContent = '\u26a1 Generate & Reload Nginx'; });
}

function showNginxModal() {
  openModal('nginxModal');
  document.getElementById('nginxSnippetContent').textContent = 'Loading...';
  fetch('/api/nginx/snippet')
    .then(function(r){ return r.json(); })
    .then(function(json){
      if (json.ok) document.getElementById('nginxSnippetContent').textContent = json.data.snippet;
    })
    .catch(function(e){ document.getElementById('nginxSnippetContent').textContent = 'Error: ' + e.message; });
}

function copySnippet() {
  var text = document.getElementById('nginxSnippetContent').textContent;
  navigator.clipboard.writeText(text).then(function(){ toast('Copied! \u2713', 'success'); });
}

function openModal(id){ document.getElementById(id).classList.add('open'); }
function closeModal(id){ document.getElementById(id).classList.remove('open'); }

document.querySelectorAll('.modal-overlay').forEach(function(m){
  m.addEventListener('click', function(e){ if(e.target===m) m.classList.remove('open'); });
});

function toast(msg, type) {
  type = type || 'success';
  var el = document.createElement('div');
  el.className = 'toast-item ' + type;
  var icons = {success:'\u2705',error:'\u274c',warn:'\u26a0\ufe0f'};
  el.innerHTML = '<span>' + (icons[type]||'\u2139\ufe0f') + '</span><span>' + esc(msg) + '</span>';
  document.getElementById('toast').appendChild(el);
  setTimeout(function(){ el.remove(); }, 4000);
}

function esc(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

document.addEventListener('keydown', function(e){
  if (e.key === 'Escape') document.querySelectorAll('.modal-overlay.open').forEach(function(m){ m.classList.remove('open'); });
});

init();
</script>
</body>
</html>`
