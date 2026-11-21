package services

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"subdeck/internal/database"
	"subdeck/internal/models"
)

const (
	probeConcurrency = 16
	probeTimeout     = 3 * time.Second
)

// ProbeNodes performs lightweight concurrent TCP handshakes against each node's server:port,
// updating LatencyMs (>0 ms on success, -1 on failure) and LastProbed in place.
func ProbeNodes(nodes []*models.Node) (healthyCount, failedCount int) {
	if len(nodes) == 0 {
		return 0, 0
	}

	now := time.Now()
	type probeResult struct {
		latencyMs int
	}
	results := make([]probeResult, len(nodes))

	jobs := make(chan int, len(nodes))
	for i := range nodes {
		jobs <- i
	}
	close(jobs)

	workers := probeConcurrency
	if len(nodes) < workers {
		workers = len(nodes)
	}

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for idx := range jobs {
				n := nodes[idx]
				if n == nil || n.Server == "" || n.Port <= 0 {
					results[idx] = probeResult{latencyMs: -1}
					continue
				}
				addr := net.JoinHostPort(n.Server, strconv.Itoa(n.Port))
				start := time.Now()
				conn, err := net.DialTimeout("tcp", addr, probeTimeout)
				if err != nil {
					results[idx] = probeResult{latencyMs: -1}
					continue
				}
				_ = conn.Close()
				rtt := int(time.Since(start).Milliseconds())
				if rtt < 1 {
					rtt = 1
				}
				results[idx] = probeResult{latencyMs: rtt}
			}
		}()
	}
	wg.Wait()

	for i, n := range nodes {
		if n == nil {
			continue
		}
		lat := results[i].latencyMs
		n.LatencyMs = lat
		t := now
		n.LastProbed = &t
		if lat > 0 {
			healthyCount++
		} else {
			failedCount++
		}
	}
	return healthyCount, failedCount
}

// ProbeSubscription probes all cached nodes in a subscription and persists their TCP RTT results.
func ProbeSubscription(db *database.DB, subID string) ([]*models.Node, int, int, error) {
	sub, err := db.GetSubscription(subID)
	if err != nil || sub == nil {
		return nil, 0, 0, fmt.Errorf("subscription not found")
	}
	healthy, failed := ProbeNodes(sub.CachedNodes)
	if err := db.UpdateSubscriptionNodesLatency(sub.ID, sub.CachedNodes); err != nil {
		return nil, 0, 0, err
	}
	return sub.CachedNodes, healthy, failed, nil
}
