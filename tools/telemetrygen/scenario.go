package main

import (
	"fmt"
	"math/rand/v2"
	"net/http"

	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/keys"
)

// Scenario identifies which of the check endpoint's decision reasons a
// single check-stream request is aimed at producing.
type Scenario int

const (
	ScenarioValid Scenario = iota
	ScenarioUnknown
	ScenarioExpired
	ScenarioMissing
)

// String returns a lowercase, log-friendly name for the scenario.
func (s Scenario) String() string {
	switch s {
	case ScenarioValid:
		return "valid"
	case ScenarioUnknown:
		return "unknown"
	case ScenarioExpired:
		return "expired"
	case ScenarioMissing:
		return "missing"
	default:
		return fmt.Sprintf("scenario(%d)", int(s))
	}
}

// expectedStatus returns the check endpoint's HTTP status for a request
// built from scenario, when the service is behaving correctly.
func (s Scenario) expectedStatus() int {
	if s == ScenarioValid {
		return http.StatusOK
	}
	return http.StatusUnauthorized
}

// Weights configures the relative likelihood of each check-mix scenario.
// Values are proportional, not required to sum to 100 — only their ratios
// matter.
type Weights struct {
	Valid   float64
	Unknown float64
	Expired float64
	Missing float64
}

// DefaultWeights returns the tool's default check-mix: 70% valid, 15%
// unknown-or-malformed, 10% expired, 5% missing-header.
func DefaultWeights() Weights {
	return Weights{Valid: 70, Unknown: 15, Expired: 10, Missing: 5}
}

// Validate reports an error if w can't be used to select a scenario: any
// negative weight, or all weights zero.
func (w Weights) Validate() error {
	if w.Valid < 0 || w.Unknown < 0 || w.Expired < 0 || w.Missing < 0 {
		return fmt.Errorf("check-mix weights must not be negative, got %+v", w)
	}
	if w.total() <= 0 {
		return fmt.Errorf("check-mix weights must not all be zero, got %+v", w)
	}
	return nil
}

func (w Weights) total() float64 {
	return w.Valid + w.Unknown + w.Expired + w.Missing
}

// SelectScenario chooses a check-mix scenario using rng, proportionally to
// w. It's a pure function of rng and w: given the same rng state, it always
// returns the same scenario, which is what makes the check-mix distribution
// unit-testable without a live service.
func SelectScenario(rng *rand.Rand, w Weights) Scenario {
	draw := rng.Float64() * w.total()

	if draw < w.Valid {
		return ScenarioValid
	}
	draw -= w.Valid

	if draw < w.Unknown {
		return ScenarioUnknown
	}
	draw -= w.Unknown

	if draw < w.Expired {
		return ScenarioExpired
	}
	return ScenarioMissing
}

// resolveScenario turns scenario into the key value a check request should
// present. Per apiClient.check's convention, an empty string omits the
// X-API-Key header entirely — used here for ScenarioMissing. ok is false
// only when scenario is ScenarioValid but pool is currently empty; err is
// set only if generating an unknown-key candidate fails.
func resolveScenario(rng *rand.Rand, scenario Scenario, pool *Pool, expiredKey string) (key string, ok bool, err error) {
	switch scenario {
	case ScenarioValid:
		key, found := pool.Random(rng)
		return key, found, nil
	case ScenarioExpired:
		return expiredKey, true, nil
	case ScenarioMissing:
		return "", true, nil
	case ScenarioUnknown:
		key, err := unknownKey(rng)
		if err != nil {
			return "", false, err
		}
		return key, true, nil
	default:
		return "", false, fmt.Errorf("resolving scenario: %v is not a known scenario", scenario)
	}
}

// malformedKeyCandidates are plaintext values that fail keys.ParseKey's
// api_<Key ID>_<secret> shape check, for the "malformed" half of the
// "unknown" check-mix scenario.
var malformedKeyCandidates = []string{
	"not-an-api-key",
	"api_",
	"api_noSecretHere",
	"Bearer sometoken",
}

// unknownKey returns a key for the "unknown-or-malformed" check-mix
// scenario: with equal probability, either a structurally malformed string
// or a well-formed api_<Key ID>_<secret> key that was never issued by the
// service (generated locally, never sent to the management API).
func unknownKey(rng *rand.Rand) (string, error) {
	if rng.IntN(2) == 0 {
		return malformedKeyCandidates[rng.IntN(len(malformedKeyCandidates))], nil
	}

	generated, err := keys.Generate()
	if err != nil {
		return "", fmt.Errorf("generating well-formed-but-nonexistent key: %w", err)
	}
	return generated.Plaintext, nil
}
