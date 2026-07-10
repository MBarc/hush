// Package neigh reads the kernel's neighbor tables (the IPv4 ARP cache and
// the IPv6 neighbor cache) to map an IP address to the hardware (MAC) address
// behind it. Hush uses this two ways: the poller enumerates the local segment
// to build its device inventory, and device auth confirms that a request from
// a new address really comes from a device it already knows, by MAC.
package neigh

import (
	"net"
	"os"
	"strconv"
	"strings"
)

// Entry is one neighbor: an IP and the MAC the kernel resolved for it.
type Entry struct {
	IP  string
	MAC string
}

// MACFor returns the hardware address the kernel currently has for ip (IPv4
// via /proc/net/arp, IPv6 via netlink), or "" if there is no complete entry.
// An active connection always has one, since the peer had to be resolved at
// layer 2 to exchange packets.
func MACFor(ip string) string {
	ip = StripZone(ip)
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ""
	}
	var table []Entry
	if parsed.To4() != nil {
		table = arp4()
	} else {
		table = neigh6()
	}
	for _, e := range table {
		if e.IP == ip {
			return e.MAC
		}
	}
	return ""
}

// InCIDR returns every complete, unicast neighbor whose IP falls inside ipnet.
// The poller uses it to record hosts on the local segment.
func InCIDR(ipnet *net.IPNet) []Entry {
	var out []Entry
	for _, e := range append(arp4(), neigh6()...) {
		if p := net.ParseIP(e.IP); p != nil && ipnet.Contains(p) {
			out = append(out, e)
		}
	}
	return out
}

// StripZone removes the "%iface" scope suffix from a link-local IPv6 address
// so it compares equal to the zone-less form the neighbor table stores.
func StripZone(ip string) string {
	if i := strings.IndexByte(ip, '%'); i >= 0 {
		return ip[:i]
	}
	return ip
}

// arp4 reads the IPv4 ARP table. On any platform without /proc/net/arp it
// simply returns nothing.
func arp4() []Entry {
	data, err := os.ReadFile("/proc/net/arp")
	if err != nil {
		return nil
	}
	return parseARP(string(data))
}

// parseARP extracts complete, unicast entries from the text of /proc/net/arp.
// Columns: IPaddress HWtype Flags HWaddress Mask Device.
func parseARP(data string) []Entry {
	var out []Entry
	for i, line := range strings.Split(data, "\n") {
		if i == 0 { // header row
			continue
		}
		f := strings.Fields(line)
		if len(f) < 6 {
			continue
		}
		ip, flags, mac := f[0], f[2], f[3]
		fl, err := strconv.ParseUint(strings.TrimPrefix(flags, "0x"), 16, 16)
		if err != nil || fl&0x2 == 0 { // ATF_COM: the entry has a resolved MAC
			continue
		}
		if net.ParseIP(ip) == nil || !unicastMAC(mac) {
			continue
		}
		out = append(out, Entry{IP: ip, MAC: strings.ToLower(mac)})
	}
	return out
}

// unicastMAC reports whether mac is a real unicast address, filtering the
// all-zero placeholder, broadcast (ff:..), and multicast (whose first octet
// has the low bit set).
func unicastMAC(mac string) bool {
	if len(mac) < 2 || mac == "00:00:00:00:00:00" {
		return false
	}
	first, err := strconv.ParseUint(mac[:2], 16, 8)
	if err != nil {
		return false
	}
	return first&1 == 0
}
