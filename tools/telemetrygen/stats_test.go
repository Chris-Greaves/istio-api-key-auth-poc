package main

import (
	"sync"
	"testing"
)

func TestStats_SnapshotStartsAtZero(t *testing.T) {
	snap := NewStats().Snapshot()

	if got := snap.CheckSent[ScenarioValid]; got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
	if got := snap.MgmtFailed["revoke"]; got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestStats_RecordCheckSentAndFailedAreTrackedPerScenario(t *testing.T) {
	stats := NewStats()

	stats.RecordCheckSent(ScenarioValid)
	stats.RecordCheckSent(ScenarioValid)
	stats.RecordCheckFailed(ScenarioValid)
	stats.RecordCheckSent(ScenarioMissing)

	snap := stats.Snapshot()
	if snap.CheckSent[ScenarioValid] != 2 {
		t.Errorf("expected 2 valid sent, got %d", snap.CheckSent[ScenarioValid])
	}
	if snap.CheckFailed[ScenarioValid] != 1 {
		t.Errorf("expected 1 valid failed, got %d", snap.CheckFailed[ScenarioValid])
	}
	if snap.CheckSent[ScenarioMissing] != 1 {
		t.Errorf("expected 1 missing sent, got %d", snap.CheckSent[ScenarioMissing])
	}
	if snap.CheckSent[ScenarioExpired] != 0 {
		t.Errorf("expected 0 expired sent, got %d", snap.CheckSent[ScenarioExpired])
	}
}

func TestStats_RecordManagementSentAndFailedAreTrackedPerOp(t *testing.T) {
	stats := NewStats()

	stats.RecordManagementSent("list")
	stats.RecordManagementFailed("revoke")
	stats.RecordManagementFailed("revoke")

	snap := stats.Snapshot()
	if snap.MgmtSent["list"] != 1 {
		t.Errorf("expected 1 list sent, got %d", snap.MgmtSent["list"])
	}
	if snap.MgmtFailed["revoke"] != 2 {
		t.Errorf("expected 2 revoke failed, got %d", snap.MgmtFailed["revoke"])
	}
	if snap.MgmtSent["create"] != 0 {
		t.Errorf("expected 0 create sent, got %d", snap.MgmtSent["create"])
	}
}

func TestStats_SnapshotIsIndependentOfLaterWrites(t *testing.T) {
	stats := NewStats()
	stats.RecordCheckSent(ScenarioValid)

	snap := stats.Snapshot()
	stats.RecordCheckSent(ScenarioValid)

	if snap.CheckSent[ScenarioValid] != 1 {
		t.Errorf("expected the earlier snapshot to stay at 1, got %d", snap.CheckSent[ScenarioValid])
	}
}

func TestStats_SafeForConcurrentUse(t *testing.T) {
	stats := NewStats()
	var wg sync.WaitGroup
	const n = 500

	for range n {
		wg.Add(2)
		go func() {
			defer wg.Done()
			stats.RecordCheckSent(ScenarioValid)
		}()
		go func() {
			defer wg.Done()
			stats.RecordManagementSent("create")
		}()
	}
	wg.Wait()

	snap := stats.Snapshot()
	if snap.CheckSent[ScenarioValid] != n {
		t.Errorf("expected %d, got %d", n, snap.CheckSent[ScenarioValid])
	}
	if snap.MgmtSent["create"] != n {
		t.Errorf("expected %d, got %d", n, snap.MgmtSent["create"])
	}
}
