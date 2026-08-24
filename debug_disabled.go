//go:build !debug

package main

import (
	"io/fs"
	"net/http"

	"sshit/internal/web"
)

// configureDebugFlags intentionally registers no development-only flags in a
// release build. Compile with `-tags debug` to enable the Vite proxy workflow.
func configureDebugFlags() {}

func logDebugMode() {}

func newHTTPHandler(hub *webHub) (http.Handler, error) {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return nil, err
	}

	files := http.FileServer(http.FS(dist))
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", webSocketShell(hub))
	mux.HandleFunc("/collab", webSocketCollab(hub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws" || r.URL.Path == "/collab" {
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
	return mux, nil
}
