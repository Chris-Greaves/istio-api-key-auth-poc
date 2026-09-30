package main

import (
	"math/rand/v2"
	"testing"
)

func TestPool_RandomReportsNoKeyWhenEmpty(t *testing.T) {
	pool := NewPool()

	if _, ok := pool.Random(rand.New(rand.NewPCG(1, 1))); ok {
		t.Fatal("expected Random to report no key available from an empty pool")
	}
}

func TestPool_RandomKeyIDReportsNoKeyWhenEmpty(t *testing.T) {
	pool := NewPool()

	if _, ok := pool.RandomKeyID(rand.New(rand.NewPCG(1, 1))); ok {
		t.Fatal("expected RandomKeyID to report no key available from an empty pool")
	}
}

func TestPool_RandomReturnsAnAddedKey(t *testing.T) {
	pool := NewPool()
	pool.Add(PoolKey{KeyID: "aaaaaaaa", Plaintext: "api_aaaaaaaa_secret"})

	key, ok := pool.Random(rand.New(rand.NewPCG(1, 1)))
	if !ok {
		t.Fatal("expected Random to return a key")
	}
	if key != "api_aaaaaaaa_secret" {
		t.Fatalf("expected %q, got %q", "api_aaaaaaaa_secret", key)
	}
}

func TestPool_RandomKeyIDReturnsAnAddedKeysID(t *testing.T) {
	pool := NewPool()
	pool.Add(PoolKey{KeyID: "aaaaaaaa", Plaintext: "api_aaaaaaaa_secret"})

	keyID, ok := pool.RandomKeyID(rand.New(rand.NewPCG(1, 1)))
	if !ok {
		t.Fatal("expected RandomKeyID to return a key ID")
	}
	if keyID != "aaaaaaaa" {
		t.Fatalf("expected %q, got %q", "aaaaaaaa", keyID)
	}
}

func TestPool_RandomOnlyEverReturnsAddedKeys(t *testing.T) {
	pool := NewPool()
	added := map[string]bool{"a": true, "b": true, "c": true}
	for k := range added {
		pool.Add(PoolKey{KeyID: k, Plaintext: k})
	}

	rng := rand.New(rand.NewPCG(42, 7))
	for range 100 {
		key, ok := pool.Random(rng)
		if !ok {
			t.Fatal("expected Random to return a key from a non-empty pool")
		}
		if !added[key] {
			t.Fatalf("got unexpected key %q", key)
		}
	}
}

func TestPool_RemoveDropsTheKeyAndReportsFound(t *testing.T) {
	pool := NewPool()
	pool.Add(PoolKey{KeyID: "aaaaaaaa", Plaintext: "api_aaaaaaaa_secret"})
	pool.Add(PoolKey{KeyID: "bbbbbbbb", Plaintext: "api_bbbbbbbb_secret"})

	if ok := pool.Remove("aaaaaaaa"); !ok {
		t.Fatal("expected Remove to report the key was found")
	}

	rng := rand.New(rand.NewPCG(1, 1))
	for range 20 {
		keyID, ok := pool.RandomKeyID(rng)
		if !ok {
			t.Fatal("expected RandomKeyID to still find the remaining key")
		}
		if keyID == "aaaaaaaa" {
			t.Fatal("removed key should never be drawn again")
		}
	}
}

func TestPool_RemoveReportsNotFoundForAnUnknownOrAlreadyRemovedKey(t *testing.T) {
	pool := NewPool()
	pool.Add(PoolKey{KeyID: "aaaaaaaa", Plaintext: "api_aaaaaaaa_secret"})

	if ok := pool.Remove("zzzzzzzz"); ok {
		t.Fatal("expected Remove to report the key was not found")
	}

	pool.Remove("aaaaaaaa")
	if ok := pool.Remove("aaaaaaaa"); ok {
		t.Fatal("expected Remove to report not found for an already-removed key")
	}
}

func TestPool_RemoveEmptiesThePool(t *testing.T) {
	pool := NewPool()
	pool.Add(PoolKey{KeyID: "aaaaaaaa", Plaintext: "api_aaaaaaaa_secret"})
	pool.Remove("aaaaaaaa")

	if _, ok := pool.Random(rand.New(rand.NewPCG(1, 1))); ok {
		t.Fatal("expected Random to report no key available after removing the only key")
	}
}
