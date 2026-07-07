package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/MBarc/hush/internal/crypto"
	"github.com/MBarc/hush/internal/server"
	"github.com/MBarc/hush/internal/store"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}
	switch os.Args[1] {
	case "serve":
		serve()
	case "version":
		fmt.Println("hush " + version)
	default:
		usage()
	}
}

func usage() {
	fmt.Print(`hush - a quiet little vault for your homelab

Usage:
  hush serve      start the server
  hush version    print the version

The full command set arrives with the CLI milestone.
`)
}

func serve() {
	addr := os.Getenv("HUSH_LISTEN")
	if addr == "" {
		addr = ":4874"
	}
	dataDir := os.Getenv("HUSH_DATA")
	if dataDir == "" {
		dataDir = "./data"
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		log.Fatalf("creating data dir %s: %v", dataDir, err)
	}
	key, err := crypto.LoadOrCreateMasterKey(dataDir)
	if err != nil {
		log.Fatalf("master key: %v", err)
	}
	st, err := store.Open(dataDir, key)
	if err != nil {
		log.Fatalf("opening vault: %v", err)
	}
	defer st.Close()

	srv, err := server.New(st, version)
	if err != nil {
		log.Fatalf("starting server: %v", err)
	}
	st.Audit(store.AuditEntry{ActorType: "system", Actor: "hush", Action: "server.start"})

	socketPath := os.Getenv("HUSH_SOCKET")
	if socketPath == "" {
		socketPath = filepath.Join(dataDir, "hush.sock")
	}
	if socketPath == "off" {
		socketPath = ""
	}
	log.Fatal(srv.Run(addr, socketPath))
}
