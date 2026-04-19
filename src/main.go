package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
)

// AppConfig holds the full app configuration
type AppConfig struct {
	AdminPort       int     `json:"admin_port"`
	NginxSnippetPath string `json:"nginx_snippet_path"`
	Routes          []Route `json:"routes"`
}

// Route represents a single proxy route
type Route struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Target      string `json:"target"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

var (
	configPath = "/home/mogagacor/Project/handlerportproxy/routes.json"
	appConfig  AppConfig
	configMu   sync.RWMutex
	mux        *http.ServeMux
	muxMu      sync.RWMutex

	// Auth settings
	adminUser     string
	adminPass     string
	sessionSecret string
)

func loadEnv() {
	envPath := "/home/mogagacor/Project/handlerportproxy/.env"
	content, err := os.ReadFile(envPath)
	if err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				os.Setenv(key, val)
			}
		}
	}

	adminUser = os.Getenv("ADMIN_USER")
	if adminUser == "" {
		adminUser = "mogagacor"
	}
	adminPass = os.Getenv("ADMIN_PASS")
	if adminPass == "" {
		adminPass = "mogagacor09"
	}
	sessionSecret = os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		sessionSecret = "secret-mogagacor-default"
	}
}

func loadConfig() error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	configMu.Lock()
	defer configMu.Unlock()
	return json.Unmarshal(data, &appConfig)
}

func saveConfig() error {
	configMu.RLock()
	defer configMu.RUnlock()
	data, err := json.MarshalIndent(appConfig, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func rebuildMux() {
	newMux := http.NewServeMux()

	// Register admin UI routes
	newMux.HandleFunc("/adminwebui", authRequired(handleAdminUI))
	newMux.HandleFunc("/adminwebui/", authRequired(handleAdminUI))
	newMux.HandleFunc("/api/routes", authRequired(handleRoutesAPI))
	newMux.HandleFunc("/api/routes/", authRequired(handleRoutesAPI))
	newMux.HandleFunc("/api/nginx/apply", authRequired(handleNginxApply))
	newMux.HandleFunc("/api/nginx/snippet", authRequired(handleNginxSnippet))
	newMux.HandleFunc("/api/reload", authRequired(handleReload))

	// Auth routes
	newMux.HandleFunc("/login", handleLogin)
	newMux.HandleFunc("/logout", handleLogout)

	// Register proxy routes
	configMu.RLock()
	routes := appConfig.Routes
	configMu.RUnlock()

	for _, route := range routes {
		if !route.Enabled {
			continue
		}
		r := route // capture
		handler := CreateProxyHandler(r.Target, r.Path)
		newMux.Handle(r.Path+"/", handler)
		log.Printf("✓ Proxy: %s → %s", r.Path, r.Target)
	}

	// Default: / handler
	newMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			if !isAuthenticated(r) {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			http.Redirect(w, r, "/adminwebui", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})

	muxMu.Lock()
	mux = newMux
	muxMu.Unlock()
}

func main() {
	loadEnv()
	if err := loadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	rebuildMux()

	// Generate initial nginx snippet
	if err := GenerateNginxSnippet(); err != nil {
		log.Printf("⚠ Could not generate nginx snippet: %v", err)
	}

	port := appConfig.AdminPort
	if port == 0 {
		port = 8001
	}

	// Use a dynamic handler wrapper
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		muxMu.RLock()
		m := mux
		muxMu.RUnlock()
		m.ServeHTTP(w, r)
	})

	log.Printf("🚀 HandlerPortProxy running on http://localhost:%d", port)
	log.Printf("🎛  Admin dashboard: http://localhost:%d/adminwebui", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), handler); err != nil {
		log.Fatal(err)
	}
}
