package main

import (
	"maps"
	"sync"
)

// Stats accumulates the tool's own client-side tallies of what it has sent:
// per check-mix scenario and per management operation, split into requests
// that got a response from the service ("sent") and ones that never did
// ("failed", e.g. a network error, or a request skipped before it could be
// sent at all). It's safe for concurrent use from both streams' per-request
// goroutines.
type Stats struct {
	mu          sync.Mutex
	checkSent   map[Scenario]int64
	checkFailed map[Scenario]int64
	mgmtSent    map[string]int64
	mgmtFailed  map[string]int64
}

// NewStats returns an empty Stats.
func NewStats() *Stats {
	return &Stats{
		checkSent:   make(map[Scenario]int64),
		checkFailed: make(map[Scenario]int64),
		mgmtSent:    make(map[string]int64),
		mgmtFailed:  make(map[string]int64),
	}
}

// RecordCheckSent records that a check-stream request for scenario got a
// response from the service, regardless of what status it returned.
func (s *Stats) RecordCheckSent(scenario Scenario) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkSent[scenario]++
}

// RecordCheckFailed records that a check-stream request for scenario never
// got a response — a network-level failure, or the request being skipped
// before it could be sent (e.g. the pool was empty).
func (s *Stats) RecordCheckFailed(scenario Scenario) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkFailed[scenario]++
}

// RecordManagementSent records that a management-stream call for op got a
// response from the service, regardless of what result it returned.
func (s *Stats) RecordManagementSent(op string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mgmtSent[op]++
}

// RecordManagementFailed records that a management-stream call for op never
// got a response, for the same reasons as RecordCheckFailed.
func (s *Stats) RecordManagementFailed(op string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mgmtFailed[op]++
}

// StatsSnapshot is a point-in-time, race-free copy of a Stats's counts, safe
// to read after Stats itself has moved on.
type StatsSnapshot struct {
	CheckSent   map[Scenario]int64
	CheckFailed map[Scenario]int64
	MgmtSent    map[string]int64
	MgmtFailed  map[string]int64
}

// Snapshot returns a copy of s's current counts.
func (s *Stats) Snapshot() StatsSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return StatsSnapshot{
		CheckSent:   cloneScenarioCounts(s.checkSent),
		CheckFailed: cloneScenarioCounts(s.checkFailed),
		MgmtSent:    cloneStringCounts(s.mgmtSent),
		MgmtFailed:  cloneStringCounts(s.mgmtFailed),
	}
}

func cloneScenarioCounts(m map[Scenario]int64) map[Scenario]int64 {
	out := make(map[Scenario]int64, len(m))
	maps.Copy(out, m)
	return out
}

func cloneStringCounts(m map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(m))
	maps.Copy(out, m)
	return out
}
