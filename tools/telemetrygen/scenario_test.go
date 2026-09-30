package main

import (
	"math/rand/v2"
	"testing"

	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/keys"
)

func TestWeights_ValidateRejectsNegativeWeights(t *testing.T) {
	w := Weights{Valid: 70, Unknown: -1, Expired: 10, Missing: 5}
	if err := w.Validate(); err == nil {
		t.Fatal("expected an error for a negative weight")
	}
}

func TestWeights_ValidateRejectsAllZero(t *testing.T) {
	w := Weights{}
	if err := w.Validate(); err == nil {
		t.Fatal("expected an error when every weight is zero")
	}
}

func TestWeights_ValidateAcceptsDefaults(t *testing.T) {
	if err := DefaultWeights().Validate(); err != nil {
		t.Fatalf("expected default weights to be valid, got %v", err)
	}
}

func TestSelectScenario_ProducesExpectedLongRunDistribution(t *testing.T) {
	weights := DefaultWeights() // 70 / 15 / 10 / 5, out of 100
	rng := rand.New(rand.NewPCG(1, 2))

	const samples = 200_000
	counts := map[Scenario]int{}
	for range samples {
		counts[SelectScenario(rng, weights)]++
	}

	const tolerance = 0.01 // absolute proportion tolerance
	checkProportion := func(scenario Scenario, wantWeight float64) {
		t.Helper()
		got := float64(counts[scenario]) / float64(samples)
		want := wantWeight / weights.total()
		if diff := got - want; diff < -tolerance || diff > tolerance {
			t.Errorf("scenario %s: got proportion %.4f, want %.4f (+/- %.2f)", scenario, got, want, tolerance)
		}
	}

	checkProportion(ScenarioValid, weights.Valid)
	checkProportion(ScenarioUnknown, weights.Unknown)
	checkProportion(ScenarioExpired, weights.Expired)
	checkProportion(ScenarioMissing, weights.Missing)
}

func TestSelectScenario_ZeroWeightScenarioIsNeverSelected(t *testing.T) {
	weights := Weights{Valid: 1, Unknown: 0, Expired: 1, Missing: 1}
	rng := rand.New(rand.NewPCG(3, 4))

	for range 10_000 {
		if SelectScenario(rng, weights) == ScenarioUnknown {
			t.Fatal("expected a zero-weight scenario to never be selected")
		}
	}
}

func TestSelectScenario_SingleNonZeroWeightAlwaysSelected(t *testing.T) {
	weights := Weights{Valid: 0, Unknown: 0, Expired: 5, Missing: 0}
	rng := rand.New(rand.NewPCG(5, 6))

	for range 1_000 {
		if got := SelectScenario(rng, weights); got != ScenarioExpired {
			t.Fatalf("expected ScenarioExpired every time, got %s", got)
		}
	}
}

func TestResolveScenario_ValidDrawsFromThePool(t *testing.T) {
	pool := NewPool()
	pool.Add(PoolKey{KeyID: "aaaaaaaa", Plaintext: "api_aaaaaaaa_secret"})
	rng := rand.New(rand.NewPCG(1, 1))

	key, ok, err := resolveScenario(rng, ScenarioValid, pool, "api_expired0_secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected a key to be available")
	}
	if key != "api_aaaaaaaa_secret" {
		t.Fatalf("expected the pool's key, got %q", key)
	}
}

func TestResolveScenario_ValidReportsNotOkWhenPoolEmpty(t *testing.T) {
	pool := NewPool()
	rng := rand.New(rand.NewPCG(1, 1))

	_, ok, err := resolveScenario(rng, ScenarioValid, pool, "api_expired0_secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false when the pool is empty")
	}
}

func TestResolveScenario_ExpiredReturnsTheDedicatedExpiredKey(t *testing.T) {
	pool := NewPool()
	rng := rand.New(rand.NewPCG(1, 1))

	key, ok, err := resolveScenario(rng, ScenarioExpired, pool, "api_expired0_secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true for the expired scenario")
	}
	if key != "api_expired0_secret" {
		t.Fatalf("expected the dedicated expired key, got %q", key)
	}
}

func TestResolveScenario_MissingReturnsAnEmptyKeyToOmitTheHeader(t *testing.T) {
	pool := NewPool()
	rng := rand.New(rand.NewPCG(1, 1))

	key, ok, err := resolveScenario(rng, ScenarioMissing, pool, "api_expired0_secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true for the missing scenario")
	}
	if key != "" {
		t.Fatalf("expected an empty key to omit the header, got %q", key)
	}
}

func TestResolveScenario_UnknownReturnsAKeyThatIsNeverValid(t *testing.T) {
	pool := NewPool()
	pool.Add(PoolKey{KeyID: "aaaaaaaa", Plaintext: "api_aaaaaaaa_secret"})
	rng := rand.New(rand.NewPCG(9, 9))

	for range 200 {
		key, ok, err := resolveScenario(rng, ScenarioUnknown, pool, "api_expired0_secret")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Fatal("expected ok=true for the unknown scenario")
		}
		if key == "api_aaaaaaaa_secret" {
			t.Fatal("unknown scenario must never reuse a pool key")
		}
		if key == "" {
			t.Fatal("unknown scenario must never be empty (that's the missing scenario)")
		}

		keyID, secret, wellFormed := keys.ParseKey(key)
		if wellFormed {
			// The well-formed branch: it must not be the pool's Key ID.
			if keyID == "aaaaaaaa" {
				t.Fatal("well-formed unknown key must not reuse the pool's Key ID")
			}
			if secret == "" {
				t.Fatal("well-formed unknown key must have a non-empty secret")
			}
		}
	}
}

func TestScenario_ExpectedStatus(t *testing.T) {
	if ScenarioValid.expectedStatus() != 200 {
		t.Errorf("expected valid scenario to expect 200, got %d", ScenarioValid.expectedStatus())
	}
	for _, s := range []Scenario{ScenarioUnknown, ScenarioExpired, ScenarioMissing} {
		if s.expectedStatus() != 401 {
			t.Errorf("expected scenario %s to expect 401, got %d", s, s.expectedStatus())
		}
	}
}
