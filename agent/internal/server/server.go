package server

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/bytebay/bytebay/agent/internal/config"
	"github.com/bytebay/bytebay/agent/internal/disks"
	"github.com/bytebay/bytebay/agent/internal/mounts"
	"github.com/bytebay/bytebay/agent/internal/network"
	"github.com/bytebay/bytebay/agent/internal/raid"
	"github.com/bytebay/bytebay/agent/internal/smart"
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
	mux.HandleFunc("GET /api/v1/disks", s.auth(s.handleDisks))
	mux.HandleFunc("GET /api/v1/disks/{device}/smart", s.auth(s.handleDiskSmart))
	mux.HandleFunc("GET /api/v1/smart", s.auth(s.handleSmartAll))
	mux.HandleFunc("GET /api/v1/smart/alerts", s.auth(s.handleSmartAlerts))
	mux.HandleFunc("GET /api/v1/raid", s.auth(s.handleRaidList))
	mux.HandleFunc("GET /api/v1/raid/{name}", s.auth(s.handleRaidDetail))
	mux.HandleFunc("POST /api/v1/raid", s.auth(s.handleRaidCreate))
	mux.HandleFunc("DELETE /api/v1/raid/{name}", s.auth(s.handleRaidStop))
	mux.HandleFunc("POST /api/v1/raid/{name}/add", s.auth(s.handleRaidAdd))
	mux.HandleFunc("GET /api/v1/mounts", s.auth(s.handleMountsList))
	mux.HandleFunc("POST /api/v1/mounts", s.auth(s.handleMountsCreate))
	mux.HandleFunc("GET /api/v1/mounts/jobs/{id}", s.auth(s.handleMountJob))
	mux.HandleFunc("DELETE /api/v1/mounts/{name}", s.auth(s.handleMountsDelete))
	mux.HandleFunc("GET /api/v1/network", s.auth(s.handleNetworkGet))
	mux.HandleFunc("PUT /api/v1/network", s.auth(s.handleNetworkPut))
	mux.HandleFunc("POST /api/v1/network/apply", s.auth(s.handleNetworkApply))
	s.http = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	return s
}

func (s *Server) ListenAndServe() error {
	_ = os.Remove(s.socket)
	ln, err := net.Listen("unix", s.socket)
	if err != nil {
		return err
	}
	s.ln = ln
	setSocketPerms(s.socket)
	return s.http.Serve(ln)
}

func setSocketPerms(path string) {
	group := config.SocketGroup()
	g, err := user.LookupGroup(group)
	if err != nil {
		log.Printf("socket: group %q not found, mode 0666: %v", group, err)
		_ = os.Chmod(path, 0o666)
		return
	}
	gid, _ := strconv.Atoi(g.Gid)
	_ = os.Chown(path, 0, gid)
	_ = os.Chmod(path, 0o660)
	log.Printf("socket: %s owned root:%s (0660)", path, group)
}

func (s *Server) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.http.Shutdown(ctx)
	if s.ln != nil {
		_ = s.ln.Close()
	}
	_ = os.Remove(s.socket)
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
		"status":    "ok",
		"role":      "host-agent",
		"socket":    s.socket,
		"smart_run": smart.LastRun(),
	})
}

func (s *Server) handleDisks(w http.ResponseWriter, _ *http.Request) {
	list, err := disks.List()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleDiskSmart(w http.ResponseWriter, r *http.Request) {
	dev := r.PathValue("device")
	info, err := smart.Query(dev)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleSmartAll(w http.ResponseWriter, _ *http.Request) {
	list, err := smart.ScanAll()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"disks":     list,
		"last_scan": smart.LastRun(),
	})
}

func (s *Server) handleSmartAlerts(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"alerts":    smart.GetAlerts(),
		"last_scan": smart.LastRun(),
	})
}

func (s *Server) handleRaidList(w http.ResponseWriter, _ *http.Request) {
	list, err := raid.List()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleRaidDetail(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	detail, err := raid.Detail(name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleRaidCreate(w http.ResponseWriter, r *http.Request) {
	var req raid.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	arr, err := raid.Create(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, arr)
}

func (s *Server) handleRaidStop(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := raid.Stop(name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (s *Server) handleRaidAdd(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req raid.AddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	arr, err := raid.Add(name, req.Device)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, arr)
}

func (s *Server) handleMountsList(w http.ResponseWriter, _ *http.Request) {
	list, err := mounts.List()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleMountsCreate(w http.ResponseWriter, r *http.Request) {
	var req mounts.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Format {
		job, err := mounts.StartJob(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, job)
		return
	}
	mp, err := mounts.Create(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, mp)
}

func (s *Server) handleMountJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, err := mounts.GetJob(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleMountsDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := mounts.Remove(name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (s *Server) handleNetworkGet(w http.ResponseWriter, _ *http.Request) {
	st, err := network.GetStatus()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleNetworkPut(w http.ResponseWriter, r *http.Request) {
	var cfg network.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := network.Apply(cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	st, err := network.GetStatus()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "applied"})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleNetworkApply(w http.ResponseWriter, _ *http.Request) {
	if err := network.Reapply(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "applied"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	log.Printf("api error: %v", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}
