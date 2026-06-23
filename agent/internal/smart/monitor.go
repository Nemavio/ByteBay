package smart

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bytebay/bytebay/agent/internal/config"
	"github.com/bytebay/bytebay/agent/internal/disks"
)

type DiskStatus struct {
	Device      string `json:"device"`
	Name        string `json:"name"`
	Model       string `json:"model,omitempty"`
	Healthy     bool   `json:"healthy"`
	TempC       *int   `json:"temp_c,omitempty"`
	Available   bool   `json:"available"`
	LastCheck   string `json:"last_check"`
	Error       string `json:"error,omitempty"`
}

type Alert struct {
	Device    string `json:"device"`
	Message   string `json:"message"`
	Severity  string `json:"severity"`
	Timestamp string `json:"timestamp"`
}

var (
	mu      sync.RWMutex
	alerts  []Alert
	lastRun time.Time
)

func ScanAll() ([]DiskStatus, error) {
	diskList, err := disks.List()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var out []DiskStatus
	var newAlerts []Alert

	for _, d := range diskList {
		st := DiskStatus{
			Device:    d.Path,
			Name:      d.Name,
			Model:     d.Model,
			LastCheck: now,
		}
		info, err := Query(d.Name)
		if err != nil {
			st.Available = false
			st.Healthy = false
			st.Error = err.Error()
			newAlerts = append(newAlerts, Alert{
				Device: d.Path, Message: err.Error(), Severity: "warning", Timestamp: now,
			})
		} else {
			st.Available = info.Available
			st.Healthy = info.Healthy
			st.TempC = info.TempC
			if info.Model != "" {
				st.Model = info.Model
			}
			if !info.Healthy {
				newAlerts = append(newAlerts, Alert{
					Device: d.Path, Message: "SMART status failed", Severity: "critical", Timestamp: now,
				})
			}
			if info.TempC != nil && *info.TempC >= 55 {
				newAlerts = append(newAlerts, Alert{
					Device: d.Path, Message: fmt.Sprintf("high temperature: %d°C", *info.TempC),
					Severity: "warning", Timestamp: now,
				})
			}
		}
		out = append(out, st)
	}

	mu.Lock()
	alerts = mergeAlerts(alerts, newAlerts, 50)
	lastRun = time.Now()
	mu.Unlock()
	_ = persistAlerts()

	return out, nil
}

func GetAlerts() []Alert {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Alert, len(alerts))
	copy(out, alerts)
	return out
}

func LastRun() string {
	mu.RLock()
	defer mu.RUnlock()
	if lastRun.IsZero() {
		return ""
	}
	return lastRun.UTC().Format(time.RFC3339)
}

func StartMonitor(intervalSec int) {
	if intervalSec <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if _, err := ScanAll(); err != nil {
				continue
			}
		}
	}()
}

func mergeAlerts(old, fresh []Alert, max int) []Alert {
	seen := make(map[string]bool)
	var out []Alert
	for _, a := range append(fresh, old...) {
		key := a.Device + "|" + a.Message
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, a)
		if len(out) >= max {
			break
		}
	}
	return out
}

func persistAlerts() error {
	if err := os.MkdirAll(config.StateDir, 0o755); err != nil {
		return err
	}
	mu.RLock()
	data, err := json.MarshalIndent(alerts, "", "  ")
	mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(config.StateDir, "smart-alerts.json"), data, 0o644)
}

func LoadPersisted() {
	path := filepath.Join(config.StateDir, "smart-alerts.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var loaded []Alert
	if json.Unmarshal(b, &loaded) == nil {
		mu.Lock()
		alerts = loaded
		mu.Unlock()
	}
}

// ponytail: skip USB sticks by size heuristic above; Query unchanged from smart.go

func QueryDeviceName(name string) (*Info, error) {
	dev := name
	if !strings.HasPrefix(dev, "/dev/") {
		dev = "/dev/" + dev
	}
	return Query(strings.TrimPrefix(dev, "/dev/"))
}
