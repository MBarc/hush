package poller

import (
	"net"
	"testing"
)

func TestParseNeighbors(t *testing.T) {
	// A realistic /proc/net/arp: a firewalled desktop that drops probes, the
	// Herupa Pi at a .255 host address (valid in a /22), an incomplete entry,
	// a multicast group, and a host outside the target subnet.
	const arp = `IP address       HW type     Flags       HW address            Mask     Device
192.168.5.124    0x1         0x2         04:d4:c4:53:94:b2     *        wlan0
192.168.5.255    0x1         0x2         e4:5f:01:ce:c5:40     *        wlan0
192.168.5.7      0x1         0x0         00:00:00:00:00:00     *        wlan0
224.0.0.251      0x1         0x2         01:00:5e:00:00:fb     *        wlan0
192.168.9.9      0x1         0x2         aa:bb:cc:dd:ee:ff     *        wlan0
`
	_, ipnet, _ := net.ParseCIDR("192.168.4.0/22")
	got := parseNeighbors(arp, ipnet)

	want := map[string]bool{"192.168.5.124": true, "192.168.5.255": true}
	if len(got) != len(want) {
		t.Fatalf("expected %d neighbors, got %d: %+v", len(want), len(got), got)
	}
	for _, n := range got {
		if !want[n.IP] {
			t.Fatalf("unexpected neighbor %+v (incomplete/multicast/out-of-range should be dropped)", n)
		}
	}
}

func TestUnicastMAC(t *testing.T) {
	cases := map[string]bool{
		"04:d4:c4:53:94:b2": true,  // unicast desktop NIC
		"e4:5f:01:ce:c5:40": true,  // unicast Raspberry Pi
		"ff:ff:ff:ff:ff:ff": false, // broadcast
		"01:00:5e:00:00:fb": false, // multicast
		"00:00:00:00:00:00": false, // placeholder
		"":                  false,
	}
	for mac, want := range cases {
		if got := unicastMAC(mac); got != want {
			t.Errorf("unicastMAC(%q) = %v, want %v", mac, got, want)
		}
	}
}

func TestHosts(t *testing.T) {
	ips, err := Hosts("192.168.1.0/30")
	if err != nil {
		t.Fatal(err)
	}
	// /30 has 4 addresses, 2 usable.
	if len(ips) != 2 || ips[0] != "192.168.1.1" || ips[1] != "192.168.1.2" {
		t.Fatalf("unexpected: %v", ips)
	}
	ips, err = Hosts("10.0.0.0/24")
	if err != nil || len(ips) != 254 {
		t.Fatalf("/24 should give 254 hosts, got %d (%v)", len(ips), err)
	}
	if _, err := Hosts("10.0.0.0/8"); err == nil {
		t.Fatal("/8 should exceed the host cap")
	}
	if _, err := Hosts("not-a-cidr"); err == nil {
		t.Fatal("garbage CIDR should error")
	}
}
