package server

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bytebay/bytebay/engine/internal/config"
	"github.com/bytebay/bytebay/engine/internal/files"
	"github.com/bytebay/bytebay/engine/internal/shares"
	"github.com/bytebay/bytebay/engine/internal/users"
	"github.com/bytebay/bytebay/engine/internal/volumes"
)

type Server struct {
	socket string
	token  string
	http   *http.Server
	ln     net.Listener
}

func New(socket, token string) *Server {
	s := &Server{socket: socket, token: token}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /api/v1/shares", s.auth(s.handleSharesList))
	mux.HandleFunc("PUT /api/v1/shares/{kind}", s.auth(s.handleSharesPut))
	mux.HandleFunc("POST /api/v1/shares/apply", s.auth(s.handleSharesApply))
	mux.HandleFunc("POST /api/v1/users/sync", s.auth(s.handleUsersSync))
	mux.HandleFunc("GET /api/v1/files", s.auth(s.handleFilesList))
	mux.HandleFunc("POST /api/v1/files/mkdir", s.auth(s.handleFilesMkdir))
	mux.HandleFunc("POST /api/v1/files/upload", s.auth(s.handleFilesUpload))
	mux.HandleFunc("GET /api/v1/files/download", s.auth(s.handleFilesDownload))
	mux.HandleFunc("GET /api/v1/volumes", s.auth(s.handleVolumesList))
	s.http = &http.Server{Handler: mux, ReadHeaderTimeout: 30 * time.Second}
	return s
}

func (s *Server) ListenAndServe() error {
	_ = os.Remove(s.socket)
	if err := os.MkdirAll(filepath.Dir(s.socket), 0o775); err != nil {
		return err
	}
	ln, err := net.Listen("unix", s.socket)
	if err != nil {
		return err
	}
	s.ln = ln
	_ = os.Chmod(s.socket, 0o666)
	return s.http.Serve(ln)
}

func (s *Server) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.http.Shutdown(ctx)
	if s.ln != nil {
		_ = s.ln.Close()
	}
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.token != "" {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") || strings.TrimPrefix(h, "Bearer ") != s.token {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"role":    "engine",
		"data":    config.DataRoot,
		"volumes": config.VolumesRoot,
	})
}

func (s *Server) handleSharesList(w http.ResponseWriter, _ *http.Request) {
	cfg, err := shares.Load()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleSharesPut(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	var body json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	cfg, err := shares.Update(kind, body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleSharesApply(w http.ResponseWriter, _ *http.Request) {
	res, err := shares.Reapply()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleUsersSync(w http.ResponseWriter, r *http.Request) {
	var p users.SyncPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := users.Sync(p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "synced"})
}

func (s *Server) handleFilesList(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	list, err := files.List(path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleFilesMkdir(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := files.Mkdir(body.Path); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleFilesUpload(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	name := r.URL.Query().Get("name")
	if path == "" || name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path and name required"})
		return
	}
	full := strings.TrimRight(path, "/") + "/" + name
	if err := files.SaveUpload(full, r.Body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "uploaded", "path": full})
}

func (s *Server) handleFilesDownload(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if err := files.Serve(w, path); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
}

func (s *Server) handleVolumesList(w http.ResponseWriter, _ *http.Request) {
	list, err := volumes.List()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	log.Printf("engine api: %v", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}
