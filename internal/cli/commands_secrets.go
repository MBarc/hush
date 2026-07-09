package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MBarc/hush/internal/auth"
	"github.com/MBarc/hush/internal/store"
)

func lsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls [path]",
		Short: "list folders and secrets",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			path := ""
			if len(args) == 1 {
				path = args[0]
			}
			tree, err := c.Tree(path)
			if err != nil {
				return err
			}
			if jsonOut {
				printJSON(tree)
				return nil
			}
			var rows [][]string
			for _, f := range tree.Folders {
				rows = append(rows, []string{f.Name + "/", "", "", ""})
			}
			for _, s := range tree.Secrets {
				rows = append(rows, []string{
					s.Name, "v" + strconv.Itoa(s.CurrentVersion),
					"agents:" + onOff(s.AgentAccess), ts(s.UpdatedAt),
				})
			}
			if len(rows) == 0 {
				fmt.Println("(empty)")
				return nil
			}
			table([]string{"NAME", "VERSION", "AGENT ACCESS", "UPDATED"}, rows)
			return nil
		},
	}
}

func getCmd() *cobra.Command {
	var version int
	var meta bool
	var field string
	cmd := &cobra.Command{
		Use:   "get <path>",
		Short: "print a secret's value (value only, script friendly)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			v, err := c.GetSecret(args[0], version)
			if err != nil {
				return err
			}
			if jsonOut {
				printJSON(v)
				return nil
			}
			if meta {
				table([]string{"PATH", "TYPE", "VERSION", "AGENT ACCESS", "CREATED", "UPDATED"},
					[][]string{{v.Meta.Path, v.Meta.Type, strconv.Itoa(v.Version),
						onOff(v.Meta.AgentAccess), ts(v.Meta.CreatedAt), ts(v.Meta.UpdatedAt)}})
				return nil
			}
			if v.Credential != nil {
				if field != "" {
					fmt.Println(credField(v.Credential, field))
					return nil
				}
				printCredential(v.Credential)
				return nil
			}
			if field != "" {
				return fmt.Errorf("--field applies only to credential entries")
			}
			fmt.Println(v.Value)
			return nil
		},
	}
	cmd.Flags().IntVar(&version, "version", 0, "read a specific version")
	cmd.Flags().BoolVar(&meta, "meta", false, "show metadata instead of the value")
	cmd.Flags().StringVar(&field, "field", "", "for a credential, print one field: username|password|url|notes")
	return cmd
}

func credField(c *store.Credential, f string) string {
	switch strings.ToLower(f) {
	case "username", "user":
		return c.Username
	case "password", "pass":
		return c.Password
	case "url":
		return c.URL
	case "notes", "note":
		return c.Notes
	}
	return ""
}

func printCredential(c *store.Credential) {
	var rows [][]string
	if c.Username != "" {
		rows = append(rows, []string{"username", c.Username})
	}
	rows = append(rows, []string{"password", c.Password})
	if c.URL != "" {
		rows = append(rows, []string{"url", c.URL})
	}
	if c.Notes != "" {
		rows = append(rows, []string{"notes", c.Notes})
	}
	table([]string{"FIELD", "VALUE"}, rows)
}

func setCmd() *cobra.Command {
	var generate int
	var agentAccess string
	cmd := &cobra.Command{
		Use:   "set <path> [value|-]",
		Short: "write a secret (new version); '-' reads the value from stdin",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			var value string
			switch {
			case generate > 0:
				if value, err = auth.GeneratePassword(generate); err != nil {
					return err
				}
			case len(args) == 2 && args[1] == "-":
				b, err := io.ReadAll(os.Stdin)
				if err != nil {
					return err
				}
				value = trimNewline(string(b))
			case len(args) == 2:
				value = args[1]
			default:
				return fmt.Errorf("provide a value, '-' for stdin, or --generate N")
			}
			var agentPtr *bool
			if agentAccess != "" {
				b, err := parseOnOff(agentAccess)
				if err != nil {
					return err
				}
				agentPtr = &b
			}
			version, err := c.SetSecret(args[0], value, agentPtr)
			if err != nil {
				return err
			}
			if jsonOut {
				printJSON(map[string]any{"path": args[0], "version": version})
				return nil
			}
			fmt.Printf("%s v%d written\n", args[0], version)
			if generate > 0 {
				fmt.Println(value)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&generate, "generate", 0, "generate a random value of this length")
	cmd.Flags().StringVar(&agentAccess, "agent-access", "", "set agent access: on or off")
	return cmd
}

func credCmd() *cobra.Command {
	root := &cobra.Command{Use: "cred", Short: "manage credential entries (username/password/url/notes)"}

	var username, password, url, notes string
	var generate int
	set := &cobra.Command{
		Use:   "set <path>",
		Short: "create or update a credential",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if generate > 0 {
				if password, err = auth.GeneratePassword(generate); err != nil {
					return err
				}
			}
			cred := store.Credential{Username: username, Password: password, URL: url, Notes: notes}
			version, err := c.SetCredential(args[0], cred)
			if err != nil {
				return err
			}
			fmt.Printf("%s credential v%d written\n", args[0], version)
			if generate > 0 {
				fmt.Printf("password: %s\n", password)
			}
			return nil
		},
	}
	set.Flags().StringVar(&username, "username", "", "username")
	set.Flags().StringVar(&password, "password", "", "password")
	set.Flags().IntVar(&generate, "generate", 0, "generate a random password of this length")
	set.Flags().StringVar(&url, "url", "", "url")
	set.Flags().StringVar(&notes, "notes", "", "notes")

	root.AddCommand(set)
	return root
}

func mvCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mv <from> <to>",
		Short: "rename a secret or move it to another folder (keeps history)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if err := c.MoveSecret(args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("%s -> %s\n", args[0], args[1])
			return nil
		},
	}
}

func rmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <path>",
		Short: "delete a secret and all its versions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if err := c.DeleteSecret(args[0]); err != nil {
				return err
			}
			fmt.Printf("%s deleted\n", args[0])
			return nil
		},
	}
}

func mkdirCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mkdir <path>",
		Short: "create a folder (parents included)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if err := c.CreateFolder(args[0]); err != nil {
				return err
			}
			fmt.Printf("%s/ created\n", args[0])
			return nil
		},
	}
}

func rmdirCmd() *cobra.Command {
	var recursive bool
	cmd := &cobra.Command{
		Use:   "rmdir <path>",
		Short: "delete a folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if err := c.DeleteFolder(args[0], recursive); err != nil {
				return err
			}
			fmt.Printf("%s/ deleted\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "delete contents too")
	return cmd
}

func versionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "versions <path>",
		Short: "list a secret's version history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			versions, err := c.Versions(args[0])
			if err != nil {
				return err
			}
			if jsonOut {
				printJSON(versions)
				return nil
			}
			var rows [][]string
			for _, v := range versions {
				rows = append(rows, []string{"v" + strconv.Itoa(v.Version), ts(v.CreatedAt), v.CreatedBy})
			}
			table([]string{"VERSION", "CREATED", "BY"}, rows)
			return nil
		},
	}
}

func metaCmd() *cobra.Command {
	var agentAccess string
	cmd := &cobra.Command{
		Use:   "meta <path>",
		Short: "show or change a secret's metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if agentAccess == "" {
				v, err := c.GetSecret(args[0], 0)
				if err != nil {
					return err
				}
				if jsonOut {
					printJSON(v.Meta)
					return nil
				}
				table([]string{"PATH", "VERSION", "AGENT ACCESS", "ROTATION", "UPDATED"},
					[][]string{{v.Meta.Path, strconv.Itoa(v.Meta.CurrentVersion),
						onOff(v.Meta.AgentAccess), v.Meta.Rotation, ts(v.Meta.UpdatedAt)}})
				return nil
			}
			b, err := parseOnOff(agentAccess)
			if err != nil {
				return err
			}
			meta, err := c.SetSecretMeta(args[0], &b, nil)
			if err != nil {
				return err
			}
			fmt.Printf("%s agent access %s\n", meta.Path, onOff(meta.AgentAccess))
			return nil
		},
	}
	cmd.Flags().StringVar(&agentAccess, "agent-access", "", "set agent access: on or off")
	return cmd
}

func rotateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rotate <path>",
		Short: "rotate a secret now (new generated value per its policy)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			version, err := c.Rotate(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("%s rotated to v%d\n", args[0], version)
			return nil
		},
	}
}

func policyCmd() *cobra.Command {
	var intervalDays, length int
	var charset, webhookURL, webhookSecret string
	var includeValue bool
	cmd := &cobra.Command{
		Use:   "policy <path>",
		Short: "show or set a secret's rotation policy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("interval-days") && !cmd.Flags().Changed("length") &&
				!cmd.Flags().Changed("charset") && !cmd.Flags().Changed("webhook-url") &&
				!cmd.Flags().Changed("webhook-secret") && !cmd.Flags().Changed("webhook-include-value") {
				v, err := c.GetSecret(args[0], 0)
				if err != nil {
					return err
				}
				fmt.Println(v.Meta.Rotation)
				return nil
			}
			policy := map[string]any{}
			if length > 0 {
				policy["length"] = length
			}
			if charset != "" {
				policy["charset"] = charset
			}
			if intervalDays > 0 {
				policy["intervalDays"] = intervalDays
			}
			if webhookURL != "" {
				policy["webhookUrl"] = webhookURL
			}
			if webhookSecret != "" {
				policy["webhookSecret"] = webhookSecret
			}
			if includeValue {
				policy["includeValue"] = true
			}
			raw, _ := json.Marshal(policy)
			meta, err := c.SetSecretMeta(args[0], nil, raw)
			if err != nil {
				return err
			}
			fmt.Printf("%s rotation policy set: %s\n", meta.Path, meta.Rotation)
			return nil
		},
	}
	cmd.Flags().IntVar(&intervalDays, "interval-days", 0, "auto-rotate every N days (0 = manual only)")
	cmd.Flags().IntVar(&length, "length", 0, "generated value length (default 32)")
	cmd.Flags().StringVar(&charset, "charset", "", "full, alnum, hex, digits, or literal characters")
	cmd.Flags().StringVar(&webhookURL, "webhook-url", "", "POST here after each rotation")
	cmd.Flags().StringVar(&webhookSecret, "webhook-secret", "", "HMAC key for the X-Hush-Signature header")
	cmd.Flags().BoolVar(&includeValue, "webhook-include-value", false, "include the new value in the webhook payload")
	return cmd
}

func parseOnOff(s string) (bool, error) {
	switch s {
	case "on", "true", "1", "yes":
		return true, nil
	case "off", "false", "0", "no":
		return false, nil
	}
	return false, fmt.Errorf("expected on or off, got %q", s)
}

func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
