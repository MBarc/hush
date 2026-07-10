package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/MBarc/hush/internal/api"
)

func loginCmd() *cobra.Command {
	var addr, username string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "log in to a hush server and store a personal token",
		RunE: func(cmd *cobra.Command, args []string) error {
			if addr == "" {
				if c, err := loadConfig(); err == nil && c.Addr != "" {
					addr = c.Addr
				} else {
					return fmt.Errorf("--addr required on first login, like --addr http://vault:4874")
				}
			}
			fmt.Printf("password for %s @ %s: ", username, addr)
			pw, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				return err
			}
			host, _ := os.Hostname()
			tokenName := fmt.Sprintf("cli-%s-%s", username, strings.ToLower(host))
			c := api.New(addr, "")
			token, err := c.LoginToken(username, string(pw), tokenName)
			if err != nil {
				return err
			}
			if err := saveConfig(config{Addr: addr, Token: token}); err != nil {
				return err
			}
			p, _ := configPath()
			fmt.Printf("logged in as %s, token %q saved to %s\n", username, tokenName, p)
			return nil
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "", "server address, like http://vault:4874")
	cmd.Flags().StringVar(&username, "username", "admin", "username")
	return cmd
}

func logoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "forget the saved login",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := configPath()
			if err != nil {
				return err
			}
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return err
			}
			fmt.Println("logged out")
			return nil
		},
	}
}

func whoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "show the current identity",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			me, err := c.Me()
			if err != nil {
				return err
			}
			if jsonOut {
				printJSON(me)
				return nil
			}
			fmt.Printf("%s (%s", me.Username, me.Role)
			if me.TokenType != "" {
				fmt.Printf(", %s token", me.TokenType)
			}
			fmt.Println(")")
			if len(me.Grants) > 0 {
				fmt.Println("grants:", strings.Join(me.Grants, ", "))
			}
			return nil
		},
	}
}

func tokenCmd() *cobra.Command {
	root := &cobra.Command{Use: "token", Short: "manage API tokens"}

	var typ string
	var ttlDays int
	create := &cobra.Command{
		Use:   "create <name | folder/name>",
		Short: "create a token (shown once)",
		Long: "Create a token, shown once. A user token (--type user) is a\n" +
			"personal login for the CLI. An agent token (--type agent) lives in a\n" +
			"folder and may read that folder and everything beneath it; give it as\n" +
			"folder/name, like \"HomeLab/Raspberry Pis/deploy-bot\".",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			name, path := args[0], ""
			if typ == "agent" {
				folder, base := splitLastSegment(args[0])
				if folder == "" {
					return fmt.Errorf("an agent token lives in a folder, like HomeLab/deploy-bot")
				}
				name, path = base, folder
			}
			out, err := c.CreateToken(name, typ, path, ttlDays)
			if err != nil {
				return err
			}
			if jsonOut {
				printJSON(out)
				return nil
			}
			where := ""
			if out.Path != "" {
				where = " in " + out.Path
			}
			fmt.Printf("token %s (%s)%s created. store it now, it is never shown again:\n%s\n",
				out.Name, out.Type, where, out.Token)
			return nil
		},
	}
	create.Flags().StringVar(&typ, "type", "user", "token type: user or agent")
	create.Flags().IntVar(&ttlDays, "ttl-days", 0, "expire after N days (0 = never)")

	ls := &cobra.Command{
		Use:   "ls",
		Short: "list tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			tokens, err := c.ListTokens()
			if err != nil {
				return err
			}
			if jsonOut {
				printJSON(tokens)
				return nil
			}
			var rows [][]string
			for _, t := range tokens {
				rows = append(rows, []string{t.Name, t.Type, t.Owner,
					t.Path, ts(t.ExpiresAt), ts(t.LastUsedAt)})
			}
			table([]string{"NAME", "TYPE", "OWNER", "FOLDER", "EXPIRES", "LAST USED"}, rows)
			return nil
		},
	}

	rm := &cobra.Command{
		Use:   "rm <name>",
		Short: "revoke a token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if err := c.DeleteToken(args[0]); err != nil {
				return err
			}
			fmt.Printf("token %s revoked\n", args[0])
			return nil
		},
	}
	root.AddCommand(create, ls, rm)
	return root
}

func userCmd() *cobra.Command {
	root := &cobra.Command{Use: "user", Short: "manage local accounts"}

	var role, password string
	create := &cobra.Command{
		Use:   "create <username>",
		Short: "create a user (password generated when omitted)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			out, err := c.CreateUser(args[0], password, role)
			if err != nil {
				return err
			}
			if jsonOut {
				printJSON(out)
				return nil
			}
			fmt.Printf("user %s (%s) created\n", out.Username, out.Role)
			if out.Password != "" {
				fmt.Printf("password (shown once): %s\n", out.Password)
			}
			return nil
		},
	}
	create.Flags().StringVar(&role, "role", "readonly", "role: admin or readonly")
	create.Flags().StringVar(&password, "password", "", "initial password (generated when omitted)")

	ls := &cobra.Command{
		Use:   "ls",
		Short: "list users",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			users, err := c.ListUsers()
			if err != nil {
				return err
			}
			if jsonOut {
				printJSON(users)
				return nil
			}
			var rows [][]string
			for _, u := range users {
				rows = append(rows, []string{u.Username, u.Role,
					strings.Join(u.Grants, ","), ts(u.CreatedAt)})
			}
			table([]string{"USERNAME", "ROLE", "GRANTS", "CREATED"}, rows)
			return nil
		},
	}

	rm := &cobra.Command{
		Use:   "rm <username>",
		Short: "delete a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if err := c.DeleteUser(args[0]); err != nil {
				return err
			}
			fmt.Printf("user %s deleted\n", args[0])
			return nil
		},
	}

	var newPassword string
	passwd := &cobra.Command{
		Use:   "passwd <username>",
		Short: "set a user's password (generated when omitted)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			generated, err := c.SetPassword(args[0], newPassword)
			if err != nil {
				return err
			}
			fmt.Printf("password updated for %s\n", args[0])
			if generated != "" {
				fmt.Printf("new password (shown once): %s\n", generated)
			}
			return nil
		},
	}
	passwd.Flags().StringVar(&newPassword, "password", "", "new password (generated when omitted)")

	grant := &cobra.Command{
		Use:   "grant <username> <folder>",
		Short: "grant a user a folder subtree",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if err := c.Grant(args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("%s granted %s/\n", args[0], args[1])
			return nil
		},
	}

	revoke := &cobra.Command{
		Use:   "revoke <username> <folder>",
		Short: "revoke a user's folder grant",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if err := c.Revoke(args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("%s revoked from %s\n", args[1], args[0])
			return nil
		},
	}

	root.AddCommand(create, ls, rm, passwd, grant, revoke)
	return root
}

// splitLastSegment splits "a/b/c" into folder "a/b" and name "c". A bare
// name with no slash returns an empty folder.
func splitLastSegment(p string) (folder, name string) {
	p = strings.Trim(strings.TrimSpace(p), "/")
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return "", p
	}
	return strings.TrimSpace(p[:i]), strings.TrimSpace(p[i+1:])
}

func auditCmd() *cobra.Command {
	var limit, offset int
	var export string
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "show the audit log (newest first)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if export != "" {
				data, err := c.ExportAudit(export)
				if err != nil {
					return err
				}
				os.Stdout.Write(data)
				return nil
			}
			entries, err := c.Audit(limit, offset)
			if err != nil {
				return err
			}
			if jsonOut {
				printJSON(entries)
				return nil
			}
			var rows [][]string
			for _, e := range entries {
				rows = append(rows, []string{
					ts(e.TS), e.Actor, e.Action, e.Path, e.IP, e.Detail,
				})
			}
			table([]string{"TIME", "ACTOR", "ACTION", "PATH", "IP", "DETAIL"}, rows)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "entries to show")
	cmd.Flags().IntVar(&offset, "offset", 0, "entries to skip")
	cmd.Flags().StringVar(&export, "export", "", "export the whole log to stdout as csv or json")
	return cmd
}
