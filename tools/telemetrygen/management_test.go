package main

import "testing"

func TestNextManagementOp_CyclesListCreateRevoke(t *testing.T) {
	want := []string{"list", "create", "revoke", "list", "create", "revoke", "list"}
	for i, want := range want {
		if got := nextManagementOp(i); got != want {
			t.Errorf("tick %d: got %q, want %q", i, got, want)
		}
	}
}
