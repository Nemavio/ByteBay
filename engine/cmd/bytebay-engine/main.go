package main

import (
	"flag"
	"log"
	"os"

	"github.com/bytebay/bytebay/engine/internal/config"
	"github.com/bytebay/bytebay/engine/internal/server"
)

func main() {
	socket := flag.String("socket", config.DefaultSocket, "Unix socket path")
	token := flag.String("token", os.Getenv("BYTEBAY_ENGINE_TOKEN"), "API bearer token (optional)")
	flag.Parse()

	srv := server.New(*socket, *token)
	log.Printf("bytebay-engine listening on %s", *socket)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
