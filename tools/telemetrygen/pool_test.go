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

func TestPool_RandomReturnsAnAddedKey(t *testing.T) {
	pool := NewPool()
	pool.Add("api_aaaaaaaa_secret")

	key, ok := pool.Random(rand.New(rand.NewPCG(1, 1)))
	if !ok {
		t.Fatal("expected Random to return a key")
	}
	if key != "api_aaaaaaaa_secret" {
		t.Fatalf("expected %q, got %q", "api_aaaaaaaa_secret", key)
	}
}

func TestPool_RandomOnlyEverReturnsAddedKeys(t *testing.T) {
	pool := NewPool()
	added := map[string]bool{"a": true, "b": true, "c": true}
	for k := range added {
		pool.Add(k)
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
