package main

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"time"
)

// isAuthenticated checks if the request has a valid session cookie
func isAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie("portproxy_session")
	if err != nil {
		return false
	}
	// Simple validation: check if the cookie value matches our expectation
	// In a real app, we'd use a more robust session management
	expected := fmt.Sprintf("%x", sha256.Sum256([]byte(adminUser+adminPass+sessionSecret)))
	return cookie.Value == expected
}

// authRequired is a middleware-like check for route handlers
func authRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			if r.Header.Get("Accept") == "application/json" {
				jsonErr(w, "Unauthorized", 401)
				return
			}
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next(w, r)
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		user := r.FormValue("username")
		pass := r.FormValue("password")

		if user == adminUser && pass == adminPass {
			// Set session cookie
			val := fmt.Sprintf("%x", sha256.Sum256([]byte(adminUser+adminPass+sessionSecret)))
			http.SetCookie(w, &http.Cookie{
				Name:     "portproxy_session",
				Value:    val,
				Path:     "/",
				HttpOnly: true,
				Expires:  time.Now().Add(24 * time.Hour),
			})
			http.Redirect(w, r, "/adminwebui", http.StatusFound)
			return
		}
		// Fallthrough to show login with error (simplified)
		http.Redirect(w, r, "/login?error=1", http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, loginHTML)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:    "portproxy_session",
		Value:   "",
		Path:    "/",
		Expires: time.Unix(0, 0),
	})
	http.Redirect(w, r, "/login", http.StatusFound)
}

var loginHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Login — HandlerPortProxy</title>
<style>
  :root {
    --bg:#0f1117;--bg2:#1a1d27;--border:#2e3147;
    --accent:#6c63ff;--danger:#ff4d6d;
    --text:#e2e8f0;--muted:#64748b;
    --font:'SF Pro Display',-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;
  }
  *{box-sizing:border-box;margin:0;padding:0}
  body{background:var(--bg);color:var(--text);font-family:var(--font);
    display:flex;align-items:center;justify-content:center;min-height:100vh}
  .login-card {
    background:var(--bg2);border:1px solid var(--border);border-radius:16px;
    padding:40px;width:100%;max-width:400px;
    box-shadow:0 20px 40px rgba(0,0,0,0.4);
    animation:fadeIn 0.5s ease;
  }
  @keyframes fadeIn{from{opacity:0;transform:translateY(10px)}to{opacity:1;transform:translateY(0)}}
  .logo-icon{width:48px;height:48px;background:var(--accent);border-radius:12px;
    display:flex;align-items:center;justify-content:center;font-size:24px;margin:0 auto 20px}
  h1{font-size:20px;font-weight:700;text-align:center;margin-bottom:8px}
  p{font-size:14px;color:var(--muted);text-align:center;margin-bottom:32px}
  .form-group{margin-bottom:20px;display:flex;flex-direction:column;gap:8px}
  label{font-size:12px;font-weight:600;color:var(--muted);text-transform:uppercase;letter-spacing:0.5px}
  input{background:var(--bg);border:1px solid var(--border);color:var(--text);
    border-radius:8px;padding:12px 16px;font-size:15px;outline:none;transition:border-color 0.2s}
  input:focus{border-color:var(--accent)}
  .btn-login{background:var(--accent);color:#fff;border:none;border-radius:8px;
    padding:14px;font-size:15px;font-weight:600;cursor:pointer;margin-top:12px;
    transition:transform 0.1s, background 0.2s}
  .btn-login:hover{background:#7c74ff}
  .btn-login:active{transform:scale(0.98)}
  .error-msg{background:rgba(255,77,109,0.1);border:1px solid var(--danger);
    color:var(--danger);padding:12px;border-radius:8px;font-size:13px;margin-bottom:20px;text-align:center}
</style>
</head>
<body>
  <div class="login-card">
    <div class="logo-icon">&#9889;</div>
    <h1>Masuk ke Dashboard</h1>
    <p>Silakan login untuk mengelola reverse proxy.</p>
    
    <script>
      const urlParams = new URLSearchParams(window.location.search);
      if (urlParams.has('error')) {
        document.write('<div class="error-msg">Username atau Password salah!</div>');
      }
    </script>

    <form method="POST" action="/login">
      <div class="form-group">
        <label>Username</label>
        <input type="text" name="username" required autofocus placeholder="mogagacor">
      </div>
      <div class="form-group">
        <label>Password</label>
        <input type="password" name="password" required placeholder="••••••••">
      </div>
      <button type="submit" class="btn-login">Login Sekarang</button>
    </form>
  </div>
</body>
</html>`
