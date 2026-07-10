//go:build linux

package neigh

import (
	"encoding/binary"
	"net"
	"syscall"
)

// struct ndmsg is 12 bytes: family(1) pad1(1) pad2(2) ifindex(4) state(2)
// flags(1) type(1). The resolved MAC lives in the NDA_LLADDR attribute that
// follows; the address is in NDA_DST.
const ndmsgLen = 12

// nudValid is the set of neighbor states that carry a usable MAC: REACHABLE,
// STALE, DELAY, PROBE, PERMANENT. INCOMPLETE and FAILED are excluded.
const nudValid = 0x02 | 0x04 | 0x08 | 0x10 | 0x80

// rtnetlink neighbor attribute types.
const (
	ndaDST    = 1
	ndaLLADDR = 2
)

// neigh6 dumps the IPv6 neighbor cache via netlink. Reading it needs no
// privileges. Returns nothing on any error.
func neigh6() []Entry {
	data, err := syscall.NetlinkRIB(syscall.RTM_GETNEIGH, syscall.AF_INET6)
	if err != nil {
		return nil
	}
	msgs, err := syscall.ParseNetlinkMessage(data)
	if err != nil {
		return nil
	}
	var out []Entry
	for _, m := range msgs {
		if m.Header.Type != syscall.RTM_NEWNEIGH || len(m.Data) < ndmsgLen {
			continue
		}
		if state := binary.LittleEndian.Uint16(m.Data[8:10]); state&nudValid == 0 {
			continue
		}
		var ip, mac string
		for b := m.Data[ndmsgLen:]; len(b) >= 4; {
			l := int(binary.LittleEndian.Uint16(b[0:2]))
			typ := binary.LittleEndian.Uint16(b[2:4])
			if l < 4 || l > len(b) {
				break
			}
			switch v := b[4:l]; typ {
			case ndaDST:
				if len(v) == net.IPv6len {
					ip = net.IP(v).String()
				}
			case ndaLLADDR:
				if len(v) == 6 {
					mac = net.HardwareAddr(v).String()
				}
			}
			adv := (l + 3) &^ 3 // rtattr entries are 4-byte aligned
			if adv <= 0 || adv > len(b) {
				break
			}
			b = b[adv:]
		}
		if ip != "" && unicastMAC(mac) {
			out = append(out, Entry{IP: ip, MAC: mac})
		}
	}
	return out
}
