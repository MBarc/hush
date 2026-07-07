package cli

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/MBarc/hush/internal/crypto"
	"github.com/MBarc/hush/internal/poller"
	"github.com/MBarc/hush/internal/server"
	"github.com/MBarc/hush/internal/store"
)

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "start the hush server",
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := os.Getenv("HUSH_LISTEN")
			if addr == "" {
				addr = ":4874"
			}
			dataDir := os.Getenv("HUSH_DATA")
			if dataDir == "" {
				dataDir = "./data"
			}
			if err := os.MkdirAll(dataDir, 0o700); err != nil {
				return err
			}
			key, err := crypto.LoadOrCreateMasterKey(dataDir)
			if err != nil {
				return err
			}
			st, err := store.Open(dataDir, key)
			if err != nil {
				return err
			}
			defer st.Close()

			srv, err := server.New(st, appVersion)
			if err != nil {
				return err
			}
			st.Audit(store.AuditEntry{ActorType: "system", Actor: "hush", Action: "server.start"})
			srv.StartRotationLoop(context.Background(), 15*time.Minute)

			if cidr := os.Getenv("HUSH_NETWORK_CIDR"); cidr != "" {
				interval := 5 * time.Minute
				if v := os.Getenv("HUSH_POLL_INTERVAL"); v != "" {
					if d, err := time.ParseDuration(v); err == nil {
						interval = d
					} else {
						log.Printf("warning: bad HUSH_POLL_INTERVAL %q, using %s", v, interval)
					}
				}
				go poller.New(st, cidr, interval).Run(context.Background())
			} else {
				log.Printf("device poller off (set HUSH_NETWORK_CIDR, like 192.168.1.0/24, to enable)")
			}

			socketPath := os.Getenv("HUSH_SOCKET")
			if socketPath == "" {
				socketPath = filepath.Join(dataDir, "hush.sock")
			}
			if socketPath == "off" {
				socketPath = ""
			}
			log.Fatal(srv.Run(addr, socketPath))
			return nil
		},
	}
}
