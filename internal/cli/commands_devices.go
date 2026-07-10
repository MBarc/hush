package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func deviceCmd() *cobra.Command {
	root := &cobra.Command{Use: "device", Short: "manage network devices (hostname-based access)"}

	ls := &cobra.Command{
		Use:   "ls",
		Short: "list discovered and trusted devices",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			devices, err := c.ListDevices()
			if err != nil {
				return err
			}
			if jsonOut {
				printJSON(devices)
				return nil
			}
			if len(devices) == 0 {
				fmt.Println("(no devices seen; set HUSH_NETWORK_CIDR to enable the poller)")
				return nil
			}
			var rows [][]string
			for _, d := range devices {
				write := ""
				if d.AllowWrite {
					write = "+write"
				}
				rows = append(rows, []string{
					deviceName(d.Label, d.Hostname, d.IP), d.IP, d.Status + write,
					strings.Join(d.Grants, ", "), ts(d.LastSeen), ts(d.ExpiresAt),
				})
			}
			table([]string{"NAME", "IP", "STATUS", "GRANTS", "LAST SEEN", "EXPIRES"}, rows)
			return nil
		},
	}

	write := &cobra.Command{
		Use:   "write <hostname> <on|off>",
		Short: "allow or forbid a device writing within its granted paths",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			allow, err := onOffArg(args[1])
			if err != nil {
				return err
			}
			if err := c.SetDeviceWrite(args[0], allow); err != nil {
				return err
			}
			if allow {
				fmt.Printf("device %s may write within its granted paths\n", args[0])
			} else {
				fmt.Printf("device %s is read-only\n", args[0])
			}
			return nil
		},
	}

	grant := &cobra.Command{
		Use:   "grant <hostname> <folder-or-secret>",
		Short: "allow a device to read a folder (cascading) or a secret",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if err := c.GrantDevice(args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("%s granted access to %s\n", args[0], args[1])
			return nil
		},
	}

	revoke := &cobra.Command{
		Use:   "revoke <hostname> <folder-or-secret>",
		Short: "revoke a device's grant on a folder or secret",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if err := c.RevokeDeviceGrant(args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("%s revoked from %s\n", args[1], args[0])
			return nil
		},
	}

	block := &cobra.Command{
		Use:   "block <hostname>",
		Short: "block a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if err := c.BlockDevice(args[0]); err != nil {
				return err
			}
			fmt.Printf("device %s blocked\n", args[0])
			return nil
		},
	}

	unblock := &cobra.Command{
		Use:   "unblock <hostname>",
		Short: "lift a block on a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if err := c.UnblockDevice(args[0]); err != nil {
				return err
			}
			fmt.Printf("device %s unblocked\n", args[0])
			return nil
		},
	}

	rm := &cobra.Command{
		Use:   "rm <hostname>",
		Short: "forget a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if err := c.DeleteDevice(args[0]); err != nil {
				return err
			}
			fmt.Printf("device %s forgotten\n", args[0])
			return nil
		},
	}

	name := &cobra.Command{
		Use:   "name <hostname-or-ip> <label>",
		Short: "give a device a friendly name (use \"\" to clear)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if err := c.NameDevice(args[0], args[1]); err != nil {
				return err
			}
			if args[1] == "" {
				fmt.Printf("cleared name for %s\n", args[0])
			} else {
				fmt.Printf("%s is now named %q\n", args[0], args[1])
			}
			return nil
		},
	}

	root.AddCommand(ls, name, write, grant, revoke, block, unblock, rm)
	return root
}

// onOffArg parses an on/off style flag value.
func onOffArg(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "on", "true", "yes", "1", "allow":
		return true, nil
	case "off", "false", "no", "0", "deny":
		return false, nil
	}
	return false, fmt.Errorf("expected on or off, got %q", s)
}

// deviceName picks what to show: the friendly label, else a real
// reverse-DNS hostname, else a dash (hostname is just the IP).
func deviceName(label, hostname, ip string) string {
	if label != "" {
		return label
	}
	if hostname != ip {
		return hostname
	}
	return "-"
}
