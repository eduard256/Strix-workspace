package probe

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"probe/pkg/probe"
)

const probeTimeout = 100 * time.Millisecond

var (
	db       *sql.DB
	ports    []int
	hasICMP  bool

	detectors []func(*probe.Response) string
)

func RegisterDetector(fn func(*probe.Response) string) {
	detectors = append(detectors, fn)
}

func RegisterAPI(database *sql.DB, scanPorts []int) {
	db = database
	ports = scanPorts
	hasICMP = probe.CanICMP()

	if hasICMP {
		log.Println("[probe] ICMP available")
	} else {
		log.Println("[probe] ICMP not available, using port scan only")
	}

	http.HandleFunc("/api/probe", apiProbe)
}

func apiProbe(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		return
	}

	ip := r.URL.Query().Get("ip")
	if ip == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing ip parameter"})
		return
	}

	if net.ParseIP(ip) == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid ip: " + ip})
		return
	}

	result := runProbe(r.Context(), ip)
	json.NewEncoder(w).Encode(result)
}

func runProbe(parent context.Context, ip string) *probe.Response {
	ctx, cancel := context.WithTimeout(parent, probeTimeout)
	defer cancel()

	resp := &probe.Response{IP: ip}
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Step 1. All probers in parallel
	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}

	run(func() {
		r, _ := probe.ScanPorts(ctx, ip, ports)
		mu.Lock()
		resp.Probes.Ports = r
		mu.Unlock()
	})
	run(func() {
		r, _ := probe.ReverseDNS(ctx, ip)
		mu.Lock()
		resp.Probes.DNS = r
		mu.Unlock()
	})
	run(func() {
		mac := probe.LookupARP(ip)
		if mac == "" {
			return
		}
		vendor := probe.LookupOUI(db, mac)
		mu.Lock()
		resp.Probes.ARP = &probe.ARPResult{MAC: mac, Vendor: vendor}
		mu.Unlock()
	})
	run(func() {
		r, _ := probe.QueryHAP(ctx, ip)
		mu.Lock()
		resp.Probes.MDNS = r
		mu.Unlock()
	})
	run(func() {
		r, _ := probe.ProbeHTTP(ctx, ip, nil)
		mu.Lock()
		resp.Probes.HTTP = r
		mu.Unlock()
	})

	if hasICMP {
		run(func() {
			r, _ := probe.Ping(ctx, ip)
			mu.Lock()
			resp.Probes.Ping = r
			mu.Unlock()
		})
	}

	wg.Wait()

	// Step 2. Determine reachable
	resp.Reachable = resp.Probes.Ports != nil && len(resp.Probes.Ports.Open) > 0
	if !resp.Reachable && resp.Probes.Ping != nil {
		resp.Reachable = true
	}

	if resp.Reachable && resp.Probes.Ping != nil {
		resp.LatencyMs = resp.Probes.Ping.LatencyMs
	}

	// Step 3. Determine type
	resp.Type = "standard"
	if !resp.Reachable {
		resp.Type = "unreachable"
	} else {
		for _, detect := range detectors {
			if t := detect(resp); t != "" {
				resp.Type = t
				break
			}
		}
	}

	return resp
}
