package main

import (
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bytebay/bytebay/agent/internal/config"
	"github.com/bytebay/bytebay/agent/internal/housekeeper"
	"github.com/bytebay/bytebay/agent/internal/logbuf"
	"github.com/bytebay/bytebay/agent/internal/mounts"
	"github.com/bytebay/bytebay/agent/internal/server"
	"github.com/bytebay/bytebay/agent/internal/smart"
)

func main() {
	socket := flag.String("socket", config.DefaultSocket, "Unix socket path")
	token := flag.String("token", os.Getenv("BYTEBAY_AGENT_TOKEN"), "API bearer token (optional)")
	flag.Parse()

	log.SetOutput(io.MultiWriter(os.Stderr, logbuf.Writer()))

	if err := os.MkdirAll("/run/bytebay", 0o755); err != nil {
		log.Fatalf("mkdir socket dir: %v", err)
	}
	if err := os.MkdirAll(config.StateDir, 0o755); err != nil {
		log.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.MkdirAll(mounts.VolumesRoot(), 0o755); err != nil {
		log.Fatalf("mkdir volumes root: %v", err)
	}
	if err := mounts.MigrateRaidSources(); err != nil {
		log.Printf("mount source migration: %v", err)
	}
	if err := mounts.Restore(); err != nil {
		log.Printf("mount restore: %v", err)
	}
	mounts.PruneOrphans()

	if report, err := housekeeper.Scan(); err == nil {
		for _, item := range report.Items {
			if item.Severity == housekeeper.SeverityAction {
				log.Printf("housekeeper: action required: %s", item.Message)
			}
		}
	}
	go housekeeper.RunPeriodic(2 * time.Minute)

	smart.LoadPersisted()
	smart.StartMonitor(config.SmartIntervalSec())
	if _, err := smart.ScanAll(); err != nil {
		log.Printf("initial smart scan: %v", err)
	}

	srv := server.New(*socket, *token)
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("agent: %v", err)
		}
	}()

	log.Printf("bytebay-agent listening on %s (group %s)", *socket, config.SocketGroup())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	srv.Shutdown()
}
