package main

import "testing"

func TestLookupPlanLongestPrefix(t *testing.T) {
	cases := []struct {
		planID     string
		wantName   string
		wantKnown  bool
		wantCredit float64
	}{
		{"individual-go", "Go", true, 10},
		{"individual-goat", "GOAT", true, 70},
		{"individual-pro", "Pro", true, 30},
		{"individual-pro-v1", "Pro", true, 80},
		{"individual-provider", "Provider", true, 0},
		{"individual-max", "Max", true, 150},
		{"individual-ultra", "Ultra", true, 300},
		{"teams-pro", "Teams Pro", true, 40},
		{"INDIVIDUAL-GOAT", "GOAT", true, 70},
		{"unknown-plan", "unknown-plan", false, 0},
	}
	for _, tc := range cases {
		info, known := lookupPlan(tc.planID)
		if known != tc.wantKnown {
			t.Errorf("lookupPlan(%q) known = %v, want %v", tc.planID, known, tc.wantKnown)
			continue
		}
		if info.Name != tc.wantName {
			t.Errorf("lookupPlan(%q) name = %q, want %q", tc.planID, info.Name, tc.wantName)
		}
		if info.MonthlyCredits != tc.wantCredit {
			t.Errorf("lookupPlan(%q) credits = %v, want %v", tc.planID, info.MonthlyCredits, tc.wantCredit)
		}
	}
}

// TestLookupPlanProV1NotCapturedByPro guards the longest-prefix rule: a naive
// first-match loop would resolve individual-pro-v1 to the legacy Pro plan.
func TestLookupPlanProV1NotCapturedByPro(t *testing.T) {
	info, known := lookupPlan("individual-pro-v1")
	if !known || info.ID != "individual-pro-v1" || info.MonthlyCredits != 80 {
		t.Fatalf("individual-pro-v1 resolved to %+v (known=%v)", info, known)
	}
	provider, known := lookupPlan("individual-provider")
	if !known || provider.ID != "individual-provider" || provider.IncludesCredits {
		t.Fatalf("individual-provider resolved to %+v (known=%v)", provider, known)
	}
}

func TestLookupPlanEmpty(t *testing.T) {
	if _, known := lookupPlan("   "); known {
		t.Fatal("empty planId must not resolve")
	}
}
