//go:build debug

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"sshit/internal/compress"
)

var viteDevServer = flag.String(
	"vite-dev-server",
	"http://127.0.0.1:5173",
	"Vite development server to proxy in a debug build",
)

func configureDebugFlags() {}

func logDebugMode() {
	log.Printf("debug build: proxying Web UI and HMR to %s", *viteDevServer)
}

func newViteProxy(rawURL string) (http.Handler, error) {
	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse Vite dev server URL: %w", err)
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, fmt.Errorf("Vite dev server URL must use http or https")
	}
	if target.Host == "" {
		return nil, fmt.Errorf("Vite dev server URL must include a host")
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Vite dev server proxy error for %s: %v", r.URL.Path, err)
		http.Error(w, "Vite dev server is unavailable; start it with `pnpm --dir web dev`", http.StatusBadGateway)
	}
	return proxy, nil
}

func newHTTPHandler(hub *webHub) (http.Handler, error) {
	frontend, err := newViteProxy(*viteDevServer)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", webSocketShell(hub))
	mux.HandleFunc("/collab", webSocketCollab(hub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/ws") || strings.HasPrefix(r.URL.Path, "/collab") {
			http.NotFound(w, r)
			return
		}
		frontend.ServeHTTP(w, r)
	})
	return compress.Middleware(mux), nil
}
