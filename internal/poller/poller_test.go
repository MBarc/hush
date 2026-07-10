package poller

import "testing"

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
