package main

import (
	"sort"
	"strings"
)

// planInfo is the static, offline description of one CommandCode plan.
//
// CommandCode returns only a machine-readable planId; plan names and included
// monthly allowances come from the `command-code` CLI bundle (npm) and the
// public pricing page. Values are reference data: the API remains the source of
// truth for caps and balances, but the catalog lets the dashboard show a
// human-readable plan name, the included allowance, and a consumption
// percentage.
type planInfo struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	MarketingName   string   `json:"marketing_name,omitempty"`
	MonthlyCredits  float64  `json:"monthly_credits"`
	IncludesCredits bool     `json:"includes_credits"`
	FiveHourCap     *float64 `json:"five_hour_cap,omitempty"`
	WeeklyCap       *float64 `json:"weekly_cap,omitempty"`
}

func floatPtr(v float64) *float64 { return &v }

// planCatalog maps known planIds to their display metadata. Matching is done by
// longest-prefix on the lowercased planId (individual-pro-v1 must not be
// captured by individual-pro).
var planCatalog = []planInfo{
	{ID: "individual-provider", Name: "Provider", MarketingName: "Provider", IncludesCredits: false},
	{ID: "individual-pro-v1", Name: "Pro", MarketingName: "Pro", MonthlyCredits: 80, IncludesCredits: true, FiveHourCap: floatPtr(16), WeeklyCap: floatPtr(40)},
	{ID: "individual-ultra", Name: "Ultra", MarketingName: "Max 20x", MonthlyCredits: 300, IncludesCredits: true, FiveHourCap: floatPtr(90), WeeklyCap: floatPtr(180)},
	{ID: "individual-max", Name: "Max", MarketingName: "Max 10x", MonthlyCredits: 150, IncludesCredits: true, FiveHourCap: floatPtr(45), WeeklyCap: floatPtr(90)},
	{ID: "individual-goat", Name: "GOAT", MarketingName: "GOAT", MonthlyCredits: 70, IncludesCredits: true, FiveHourCap: floatPtr(14), WeeklyCap: floatPtr(35)},
	{ID: "individual-go", Name: "Go", MarketingName: "Go", MonthlyCredits: 10, IncludesCredits: true, FiveHourCap: floatPtr(3), WeeklyCap: floatPtr(6)},
	{ID: "individual-pro", Name: "Pro", MarketingName: "Pro (legacy)", MonthlyCredits: 30, IncludesCredits: true},
	{ID: "teams-pro", Name: "Teams Pro", MarketingName: "Team Pro", MonthlyCredits: 40, IncludesCredits: true, FiveHourCap: floatPtr(12), WeeklyCap: floatPtr(24)},
}

func init() {
	// Sort longest-first so prefix matching is unambiguous regardless of the
	// declaration order above.
	sort.SliceStable(planCatalog, func(i, j int) bool {
		return len(planCatalog[i].ID) > len(planCatalog[j].ID)
	})
}

// lookupPlan resolves a CommandCode planId to its catalog entry. The second
// result is false when the planId is unknown, in which case a synthetic entry
// carrying the raw id is returned.
func lookupPlan(planID string) (planInfo, bool) {
	normalized := strings.ToLower(strings.TrimSpace(planID))
	if normalized == "" {
		return planInfo{}, false
	}
	for _, candidate := range planCatalog {
		if strings.HasPrefix(normalized, candidate.ID) {
			return candidate, true
		}
	}
	return planInfo{ID: strings.TrimSpace(planID), Name: strings.TrimSpace(planID)}, false
}
