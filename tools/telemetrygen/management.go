package main

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// managementStreamKeyOwner labels keys the management stream creates, so
// they're identifiable (e.g. via the Web UI) as telemetrygen's own
// dynamically-created keys rather than real consumers.
const managementStreamKeyOwner = "telemetrygen-managed"

// managementOps is the fixed cycle of management-API calls the stream
// rotates through: list, create, revoke.
var managementOps = [...]string{"list", "create", "revoke"}

// nextManagementOp returns the management-API call to make on the i-th tick
// (0-indexed) of the management stream, cycling through managementOps in
// order. Pulled out as a pure function so the cycling logic is
// unit-testable without a ticker or a live service.
func nextManagementOp(i int) string {
	return managementOps[i%len(managementOps)]
}

// runManagementStream sends a steady, rate-controlled stream of list/create/
// revoke calls against the management API at the given interval, cycling
// through managementOps in order, until ctx is cancelled. It runs
// independently of the check stream: create calls add newly created keys
// into pool so the check stream can subsequently draw them as valid, and
// revoke calls pick a random active key from pool, revoke it via the
// management API, and remove it from pool so the check stream stops drawing
// it as valid from that point on.
//
// Like the check stream, this is open-loop — ticker-driven against interval
// rather than tied to response latency — so a slow or hanging call never
// throttles the stream's rate; each call runs in its own goroutine.
func runManagementStream(ctx context.Context, logger *slog.Logger, client *apiClient, pool *Pool, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	rng := newRNG()
	var wg sync.WaitGroup
	tick := 0

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case <-ticker.C:
			switch nextManagementOp(tick) {
			case "list":
				wg.Go(func() {
					if _, err := client.listKeys(ctx); err != nil {
						if ctx.Err() != nil {
							return
						}
						logger.Warn("management list call failed", "error", err)
					}
				})

			case "create":
				wg.Go(func() {
					key, err := client.createKey(ctx, managementStreamKeyOwner, nil)
					if err != nil {
						if ctx.Err() != nil {
							return
						}
						logger.Warn("management create call failed", "error", err)
						return
					}
					pool.Add(key)
				})

			case "revoke":
				keyID, ok := pool.RandomKeyID(rng)
				if !ok {
					logger.Warn("pool is empty, skipping revoke")
					break
				}

				wg.Go(func() {
					if err := client.revokeKey(ctx, keyID); err != nil {
						if ctx.Err() != nil {
							return
						}
						if !errors.Is(err, ErrKeyNotFound) {
							logger.Warn("management revoke call failed", "key_id", keyID, "error", err)
						}
						return
					}
					pool.Remove(keyID)
				})
			}
			tick++
		}
	}
}
