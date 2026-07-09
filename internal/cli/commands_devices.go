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
				// No reverse-DNS name: identity is the IP, so don't repeat it.
				name := d.Hostname
				if name == d.IP {
					name = "-"
				}
				rows = append(rows, []string{
					name, d.IP, d.Status + write,
					strings.Join(d.Scopes, ","), ts(d.LastSeen), ts(d.ExpiresAt),
				})
			}
			table([]string{"NAME", "IP", "STATUS", "SCOPES", "LAST SEEN", "EXPIRES"}, rows)
			return nil
		},
	}

	var scopes []string
	var allowWrite bool
	var ttlDays int
	trust := &cobra.Command{
		Use:   "trust <hostname>",
		Short: "allow a device to fetch secrets by hostname",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if err := c.TrustDevice(args[0], scopes, allowWrite, ttlDays); err != nil {
				return err
			}
			fmt.Printf("device %s trusted for %s", args[0], strings.Join(scopes, ", "))
			if allowWrite {
				fmt.Print(" (read and write)")
			}
			if ttlDays > 0 {
				fmt.Printf(" for %d days", ttlDays)
			}
			fmt.Println()
			return nil
		},
	}
	trust.Flags().StringArrayVar(&scopes, "scope", nil, "path scope, like infra/nas/* (repeatable, required)")
	trust.Flags().BoolVar(&allowWrite, "allow-write", false, "also allow writes within scope")
	trust.Flags().IntVar(&ttlDays, "ttl-days", 0, "trust expires after N days (0 = never)")

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

	root.AddCommand(ls, trust, block, rm)
	return root
}
