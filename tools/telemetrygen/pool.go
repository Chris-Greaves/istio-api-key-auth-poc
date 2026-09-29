package main

import (
	"math/rand/v2"
	"sync"
)

// Pool is an in-process, mutex-guarded set of the currently-active API Keys
// the check stream draws "valid" requests from. The dedicated pre-expired
// key created at bootstrap is never added here — it's used exclusively for
// the "expired" scenario, kept outside the pool of otherwise-valid keys.
type Pool struct {
	mu   sync.Mutex
	keys []string
}

// NewPool returns an empty Pool.
func NewPool() *Pool {
	return &Pool{}
}

// Add adds key to the pool as active.
func (p *Pool) Add(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.keys = append(p.keys, key)
}

// Random returns a uniformly random active key from the pool using rng, and
// false if the pool is currently empty.
func (p *Pool) Random(rng *rand.Rand) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.keys) == 0 {
		return "", false
	}
	return p.keys[rng.IntN(len(p.keys))], true
}
