package main

import (
	"flag"
	"io"
	"log"
	"os"

	"github.com/bytebay/bytebay/engine/internal/config"
	"github.com/bytebay/bytebay/engine/internal/logbuf"
	"github.com/bytebay/bytebay/engine/internal/server"
	"github.com/bytebay/bytebay/engine/internal/shares"
	"github.com/bytebay/bytebay/engine/internal/users"
)

func main() {
	socket := flag.String("socket", config.DefaultSocket, "Unix socket path")
	token := flag.String("token", os.Getenv("BYTEBAY_ENGINE_TOKEN"), "API bearer token (optional)")
	flag.Parse()

	log.SetOutput(io.MultiWriter(os.Stderr, logbuf.Writer()))
	if err := users.RestorePersisted(); err != nil {
		log.Printf("restore samba/ftp users: %v", err)
	}
	if err := shares.RestorePersisted(); err != nil {
		log.Printf("restore shares: %v", err)
	}
	srv := server.New(*socket, *token)
	log.Printf("bytebay-engine listening on %s", *socket)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
