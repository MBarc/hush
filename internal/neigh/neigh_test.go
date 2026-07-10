package neigh

import "testing"

func TestParseARP(t *testing.T) {
	// A firewalled desktop, the Herupa Pi at a .255 host (valid in a /22), an
	// incomplete entry, a multicast group, and a short/garbage line.
	const arp = `IP address       HW type     Flags       HW address            Mask     Device
192.168.5.124    0x1         0x2         04:D4:C4:53:94:B2     *        wlan0
192.168.5.255    0x1         0x2         e4:5f:01:ce:c5:40     *        wlan0
192.168.5.7      0x1         0x0         00:00:00:00:00:00     *        wlan0
224.0.0.251      0x1         0x2         01:00:5e:00:00:fb     *        wlan0
garbage
`
	got := parseARP(arp)
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d: %+v", len(got), got)
	}
	// MACs are normalized to lowercase for stable comparison.
	if got[0].IP != "192.168.5.124" || got[0].MAC != "04:d4:c4:53:94:b2" {
		t.Fatalf("first entry wrong: %+v", got[0])
	}
	if got[1].IP != "192.168.5.255" {
		t.Fatalf("second entry wrong: %+v", got[1])
	}
}

func TestUnicastMAC(t *testing.T) {
	cases := map[string]bool{
		"04:d4:c4:53:94:b2": true,
		"e4:5f:01:ce:c5:40": true,
		"ff:ff:ff:ff:ff:ff": false,
		"01:00:5e:00:00:fb": false,
		"00:00:00:00:00:00": false,
		"":                  false,
	}
	for mac, want := range cases {
		if got := unicastMAC(mac); got != want {
			t.Errorf("unicastMAC(%q) = %v, want %v", mac, got, want)
		}
	}
}

func TestStripZone(t *testing.T) {
	if got := StripZone("fe80::1%wlan0"); got != "fe80::1" {
		t.Fatalf("zone not stripped: %q", got)
	}
	if got := StripZone("192.168.5.124"); got != "192.168.5.124" {
		t.Fatalf("plain address changed: %q", got)
	}
}
