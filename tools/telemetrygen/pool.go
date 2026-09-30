package main

import (
	"math/rand/v2"
	"sync"
)

// PoolKey is a single API Key tracked in the shared pool: its plaintext
// value (what the check stream presents via X-API-Key) and its Key ID (what
// the management API's revoke endpoint identifies it by).
type PoolKey struct {
	KeyID     string
	Plaintext string
}

// Pool is an in-process, mutex-guarded set of the currently-active API Keys
// the check stream draws "valid" requests from, and the management stream
// mutates over time via Add (on create) and Remove (on revoke). The
// dedicated pre-expired key created at bootstrap is never added here — it's
// used exclusively for the "expired" check-mix scenario, kept outside the
// pool of otherwise-valid keys.
type Pool struct {
	mu   sync.Mutex
	keys []PoolKey
}

// NewPool returns an empty Pool.
func NewPool() *Pool {
	return &Pool{}
}

// Add adds key to the pool as active.
func (p *Pool) Add(key PoolKey) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.keys = append(p.keys, key)
}

// Remove removes the active key with the given Key ID, if present, and
// reports whether it was found. Once removed, the check stream can no
// longer draw it as valid.
func (p *Pool) Remove(keyID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, key := range p.keys {
		if key.KeyID == keyID {
			p.keys = append(p.keys[:i], p.keys[i+1:]...)
			return true
		}
	}
	return false
}

// Random returns the plaintext of a uniformly random active key from the
// pool using rng, and false if the pool is currently empty.
func (p *Pool) Random(rng *rand.Rand) (string, bool) {
	key, ok := p.random(rng)
	return key.Plaintext, ok
}

// RandomKeyID returns the Key ID of a uniformly random active key from the
// pool using rng, and false if the pool is currently empty. The management
// stream uses this to pick which key to revoke.
func (p *Pool) RandomKeyID(rng *rand.Rand) (string, bool) {
	key, ok := p.random(rng)
	return key.KeyID, ok
}

// random returns a uniformly random active PoolKey using rng, and false if
// the pool is currently empty.
func (p *Pool) random(rng *rand.Rand) (PoolKey, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.keys) == 0 {
		return PoolKey{}, false
	}
	return p.keys[rng.IntN(len(p.keys))], true
}
