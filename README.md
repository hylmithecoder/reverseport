# HandlerPortProxy ⚡

A lightweight, Go-based reverse proxy manager designed for simple yet secure port forwarding, primarily used for exposure via reverse tunneling proxies to a VPS.

## 🌟 Features
- **Web Dashboard**: Intuitive UI to manage your proxy routes.
- **Secure Authentication**: Protected by login with `.env` based credentials.
- **Nginx Integration**: Automatically generates snippet configuration files for Nginx.
- **Systemd Ready**: Easily deployable as a background service.
- **Real-time Updates**: Apply configuration changes to Nginx with a single click.

## 🛠️ Installation

### 1. Build from Source
Ensure you have Go installed on your system.
```bash
git clone <your-repo-link>
cd handlerportproxy
go build -o portproxy ./src/
```

### 2. Configuration
Create a `.env` file in the project root to set your credentials:
```env
ADMIN_USER=mogagacor
ADMIN_PASS=mogagacor09
SESSION_SECRET=your-random-secret-key
```

### 3. Deploy as a Service (Systemd)
Create a service file at `/etc/systemd/system/portproxy.service`:
```ini
[Unit]
Description=HandlerPortProxy Local Manager
After=network.target

[Service]
User=mogagacor
WorkingDirectory=/home/mogagacor/Project/handlerportproxy
ExecStart=/home/mogagacor/Project/handlerportproxy/portproxy
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
```
Then enable and start the service:
```bash
sudo systemctl enable portproxy
sudo systemctl start portproxy
```

## 🌐 Nginx Setup
To connect your routes to Nginx, add the following line inside your `server { }` block in your primary Nginx config file:

```nginx
include /home/mogagacor/Project/handlerportproxy/portproxy-routes.conf;
```

## 🎛️ Usage
1. Open the Admin Dashboard at `http://localhost:8001/adminwebui`.
2. Login using the credentials defined in your `.env` file.
3. Add your local application ports (e.g., `http://localhost:3000`) and assign them a public path prefix (e.g., `/my-app`).
4. Click **Generate & Reload Nginx** to apply changes instantly.

## 🔒 Security
Since this tool is intended to be used with reverse tunneling to a VPS, security is paramount.
- The dashboard is protected by **SHA256 hashed session cookies**.
- Environment variables are kept in a `.env` file (ensure this is in your `.gitignore`).
- It is recommended to use HTTPS on your VPS-side Nginx proxy.

## 📄 License
MIT License. Feel free to use and modify for your own needs.
