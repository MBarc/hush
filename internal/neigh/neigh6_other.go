//go:build !linux

package neigh

// neigh6 is Linux-only (netlink). Elsewhere the IPv6 neighbor cache is not
// read, which is fine: discovery needs host networking on Linux anyway.
func neigh6() []Entry { return nil }
