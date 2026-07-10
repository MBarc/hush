// Package cli implements the hush command line. Every capability of the
// web UI maps to a command here; both drive the same HTTP API.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/MBarc/hush/internal/api"
)

var (
	appVersion string
	jsonOut    bool
)

// Execute runs the CLI.
func Execute(version string) {
	appVersion = version
	root := &cobra.Command{
		Use:           "hush",
		Short:         "hush - a quiet little vault for your homelab",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVar(&jsonOut, "json", false, "output raw JSON")

	root.AddCommand(
		serveCmd(), versionCmd(), loginCmd(), logoutCmd(), whoamiCmd(),
		lsCmd(), catalogCmd(), getCmd(), setCmd(), credCmd(), mvCmd(), rmCmd(), mkdirCmd(), rmdirCmd(),
		versionsCmd(), metaCmd(), rotateCmd(), policyCmd(),
		tokenCmd(), userCmd(), deviceCmd(), auditCmd(),
	)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "print the version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("hush " + appVersion)
		},
	}
}

// --- client resolution ---

type config struct {
	Addr  string `json:"addr"`
	Token string `json:"token"`
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".hush", "config.json"), nil
}

func loadConfig() (config, error) {
	var c config
	p, err := configPath()
	if err != nil {
		return c, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(b, &c)
}

func saveConfig(c config) error {
	p, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(p, b, 0o600)
}

// client picks the connection in priority order: explicit socket, the
// default local socket (present when running inside or next to the
// container), env, then the saved login.
func client() (*api.Client, error) {
	if sock := os.Getenv("HUSH_SOCKET"); sock != "" && sock != "off" {
		return api.NewSocket(sock), nil
	}
	if _, err := os.Stat("/data/hush.sock"); err == nil {
		return api.NewSocket("/data/hush.sock"), nil
	}
	addr, token := os.Getenv("HUSH_ADDR"), os.Getenv("HUSH_TOKEN")
	if addr != "" && token != "" {
		return api.New(addr, token), nil
	}
	if c, err := loadConfig(); err == nil && c.Addr != "" && c.Token != "" {
		return api.New(c.Addr, c.Token), nil
	}
	return nil, fmt.Errorf("not connected: run 'hush login --addr http://<host>:4874', set HUSH_ADDR and HUSH_TOKEN, or run on the vault host (docker exec hush hush ...)")
}

// --- output helpers ---

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func table(header []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(header, "\t"))
	for _, r := range rows {
		fmt.Fprintln(w, strings.Join(r, "\t"))
	}
	w.Flush()
}

func ts(unix int64) string {
	if unix == 0 {
		return "-"
	}
	return time.Unix(unix, 0).Format("2006-01-02 15:04")
}
