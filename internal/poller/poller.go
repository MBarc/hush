// Package poller discovers devices on the homelab network. It sweeps a CIDR
// with lightweight TCP probes (a refused connection still proves a live
// host), then reads the kernel ARP table to also catch hosts that answer ARP
// but silently drop the probes (a firewalled desktop, say). It resolves
// hostnames via reverse DNS and records sightings in the device inventory.
// The device table is the trust anchor for hostname-based access: a claimed
// hostname must arrive from the IP the poller last saw it at.
package poller

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/MBarc/hush/internal/neigh"
	"github.com/MBarc/hush/internal/store"
)

// probePorts are tried in order until one proves the host is alive. A
// connection refused answer counts: something sent the RST.
var probePorts = []string{"445", "22", "80", "443", "3389", "8080", "139"}

const probeTimeout = 400 * time.Millisecond
const maxHosts = 4096
const concurrency = 64

type Poller struct {
	st       *store.Store
	cidr     string
	interval time.Duration
}

func New(st *store.Store, cidr string, interval time.Duration) *Poller {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &Poller{st: st, cidr: cidr, interval: interval}
}

// Run sweeps immediately, then on every tick, until ctx is done.
func (p *Poller) Run(ctx context.Context) {
	log.Printf("device poller: sweeping %s every %s", p.cidr, p.interval)
	p.sweep(ctx)
	t := time.NewTicker(p.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.sweep(ctx)
		}
	}
}

func (p *Poller) sweep(ctx context.Context) {
	ips, err := Hosts(p.cidr)
	if err != nil {
		log.Printf("device poller: bad cidr %q: %v", p.cidr, err)
		return
	}
	_, ipnet, err := net.ParseCIDR(p.cidr)
	if err != nil {
		log.Printf("device poller: bad cidr %q: %v", p.cidr, err)
		return
	}
	start := time.Now()
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	found := map[string]bool{}
	// record identifies a host (reverse DNS is best-effort: many home
	// networks publish no PTR records, so a nameless host is identified by
	// its IP and authenticates with `X-Hush-Device: <ip>`) with its MAC.
	record := func(ip, mac string) {
		identity := ReverseLookup(ip)
		if identity == "" {
			identity = ip
		}
		if err := p.st.UpsertDeviceSeen(identity, ip, mac); err == nil {
			mu.Lock()
			found[ip] = true
			mu.Unlock()
		}
	}
	for _, ip := range ips {
		select {
		case <-ctx.Done():
			return
		default:
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(ip string) {
			defer wg.Done()
			defer func() { <-sem }()
			// A successful probe (open or refused) means the kernel has
			// already ARP-resolved the host, so its MAC is available now.
			if Alive(ip) {
				record(ip, neigh.MACFor(ip))
			}
		}(ip)
	}
	wg.Wait()
	probed := len(found)

	// Passive pass: probing every address above forced the kernel to
	// ARP-resolve the whole subnet, and a host answers ARP even when it
	// silently drops our TCP probes (a firewalled desktop, say). So anything
	// now in the neighbor table is a real device we would otherwise miss.
	for _, n := range neigh.InCIDR(ipnet) {
		if !found[n.IP] {
			record(n.IP, n.MAC)
		}
	}
	log.Printf("device poller: swept %d addresses in %s, %d live (%d via probe, %d via arp)",
		len(ips), time.Since(start).Round(time.Millisecond), len(found), probed, len(found)-probed)
}

// Alive probes ip with quick TCP dials. Open or refused both mean a live
// host; timeouts and unreachables mean nothing answered.
func Alive(ip string) bool {
	for _, port := range probePorts {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, port), probeTimeout)
		if err == nil {
			conn.Close()
			return true
		}
		if strings.Contains(err.Error(), "refused") {
			return true
		}
	}
	return false
}

// ReverseLookup returns the PTR name for ip, normalized, or "".
func ReverseLookup(ip string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	if err != nil || len(names) == 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSuffix(names[0], "."))
}

// Hosts expands a CIDR into its usable host addresses (capped at maxHosts).
func Hosts(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return nil, fmt.Errorf("only IPv4 CIDRs are supported, got %s", cidr)
	}
	var out []string
	for addr := ip4.Mask(ipnet.Mask); ipnet.Contains(addr); increment(addr) {
		out = append(out, addr.String())
		if len(out) > maxHosts {
			return nil, fmt.Errorf("%s expands past %d hosts, narrow the CIDR", cidr, maxHosts)
		}
	}
	// Trim network and broadcast addresses for real subnets.
	if ones, bits := ipnet.Mask.Size(); bits-ones >= 2 && len(out) > 2 {
		out = out[1 : len(out)-1]
	}
	return out, nil
}

func increment(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}
