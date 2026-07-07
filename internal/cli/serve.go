package cli

import (
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/MBarc/hush/internal/crypto"
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
