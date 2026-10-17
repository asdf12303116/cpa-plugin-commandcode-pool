package main

import (
	"encoding/json"
	"testing"
	"time"
)

const liveCreditsPayload = `{"credits":{"belowThreshold":false,"creditThreshold":0,"monthlyCredits":49.489632881,"purchasedCredits":0,"freeCredits":0},"windowLimits":{"limited":true,"exceeded":null,"fiveHour":{"used":0.115137764,"cap":14,"exceeded":false,"resetAt":1790370899831},"weekly":{"used":0.401068892,"cap":35,"exceeded":false,"resetAt":1790798433255}},"sandboxAccess":true,"sandboxMinutes":{"usedMinutes":0,"limitMinutes":0,"overageUsd":0,"resetAt":1793491200000}}`

// sandboxMinutes was a nullable scalar (`null`) until 2026-09 and became an
// object in 2026-10. The old shape must still decode (to nil) so the credits
// response never fails to parse over this field.
const legacyCreditsPayload = `{"credits":{"belowThreshold":false,"creditThreshold":0,"monthlyCredits":49.489632881,"purchasedCredits":0,"freeCredits":0},"windowLimits":{"limited":true,"exceeded":null,"fiveHour":{"used":0.115137764,"cap":14,"exceeded":false,"resetAt":1790370899831},"weekly":{"used":0.401068892,"cap":35,"exceeded":false,"resetAt":1790798433255}},"sandboxAccess":false,"sandboxMinutes":null}`

func TestParseCreditsPayloadKeepsNullExceeded(t *testing.T) {
	var cr creditsResponse
	if errUnmarshal := json.Unmarshal([]byte(liveCreditsPayload), &cr); errUnmarshal != nil {
		t.Fatal(errUnmarshal)
	}
	if cr.Credits == nil || cr.WindowLimits == nil {
		t.Fatal("credits/windowLimits must be present")
	}
	if cr.Credits.MonthlyCredits != 49.489632881 {
		t.Errorf("monthlyCredits = %v", cr.Credits.MonthlyCredits)
	}
	// exceeded is null at the top level: it must stay nil, never false.
	if cr.WindowLimits.Exceeded != nil {
		t.Errorf("top-level exceeded should be nil (null), got %v", *cr.WindowLimits.Exceeded)
	}
	if cr.WindowLimits.FiveHour == nil || cr.WindowLimits.FiveHour.Cap != 14 {
		t.Fatalf("fiveHour = %+v", cr.WindowLimits.FiveHour)
	}
	if cr.WindowLimits.Weekly == nil || cr.WindowLimits.Weekly.Cap != 35 {
		t.Fatalf("weekly = %+v", cr.WindowLimits.Weekly)
	}
}

// TestParseCreditsPayloadDecodesSandboxMinutesObject guards the 2026-10 shape
// change: a scalar->object field must not fail the whole credits decode.
func TestParseCreditsPayloadDecodesSandboxMinutesObject(t *testing.T) {
	var cr creditsResponse
	if errUnmarshal := json.Unmarshal([]byte(liveCreditsPayload), &cr); errUnmarshal != nil {
		t.Fatal(errUnmarshal)
	}
	if cr.SandboxMinutes == nil {
		t.Fatal("sandboxMinutes object must decode, not be dropped")
	}
	if cr.SandboxMinutes.ResetAt != 1793491200000 {
		t.Errorf("sandboxMinutes.resetAt = %d", cr.SandboxMinutes.ResetAt)
	}
	if cr.SandboxMinutes.UsedMinutes != 0 || cr.SandboxMinutes.LimitMinutes != 0 || cr.SandboxMinutes.OverageUSD != 0 {
		t.Errorf("sandboxMinutes = %+v", cr.SandboxMinutes)
	}
	if cr.Credits == nil || cr.WindowLimits == nil {
		t.Fatal("balance/window fields must survive a changed sandboxMinutes")
	}
}

func TestParseCreditsPayloadToleratesLegacyNullSandboxMinutes(t *testing.T) {
	var cr creditsResponse
	if errUnmarshal := json.Unmarshal([]byte(legacyCreditsPayload), &cr); errUnmarshal != nil {
		t.Fatal(errUnmarshal)
	}
	if cr.SandboxMinutes != nil {
		t.Errorf("null sandboxMinutes must decode to nil, got %+v", cr.SandboxMinutes)
	}
	if cr.SandboxAccess == nil || *cr.SandboxAccess {
		t.Errorf("sandboxAccess = %v", cr.SandboxAccess)
	}
}

func TestEpochMillisToTime(t *testing.T) {
	got := epochMillisToTime(1790370899831)
	want := time.UnixMilli(1790370899831).UTC()
	if !got.Equal(want) {
		t.Fatalf("epochMillisToTime = %v, want %v", got, want)
	}
	if !epochMillisToTime(0).IsZero() {
		t.Fatal("zero epoch must map to zero time")
	}
}

func TestParseAPIErrorShapes(t *testing.T) {
	wrapped := parseAPIError(401, []byte(`{"success":false,"error":{"code":"UNAUTHORIZED","status":401,"message":"Invalid 'Authorization' header or token.","docs":"https://commandcode.ai/docs"}}`))
	apiErr, ok := wrapped.(*apiError)
	if !ok {
		t.Fatalf("expected *apiError, got %T", wrapped)
	}
	if apiErr.Code != "UNAUTHORIZED" || apiErr.StatusCode != 401 {
		t.Errorf("wrapped error = %+v", apiErr)
	}
	if apiErr.Message == "" {
		t.Error("wrapped error message must not be empty")
	}

	flat := parseAPIError(404, []byte(`{"success":false,"status":404,"message":"404 Not found.","cause":"not a registered API route"}`))
	flatErr, ok := flat.(*apiError)
	if !ok {
		t.Fatalf("expected *apiError, got %T", flat)
	}
	if flatErr.StatusCode != 404 || flatErr.Message != "404 Not found." {
		t.Errorf("flat error = %+v", flatErr)
	}

	empty := parseAPIError(500, nil)
	emptyErr, ok := empty.(*apiError)
	if !ok || emptyErr.Message == "" {
		t.Fatalf("empty body must still produce a message: %+v", empty)
	}
}

func TestSubscriptionEnvelopeWithoutData(t *testing.T) {
	var env subscriptionEnvelope
	if errUnmarshal := json.Unmarshal([]byte(`{"success":true,"data":null}`), &env); errUnmarshal != nil {
		t.Fatal(errUnmarshal)
	}
	if env.Data != nil {
		t.Fatal("null data must decode to no subscription")
	}
}

func TestOrgQueryOmitsEmpty(t *testing.T) {
	if got := orgQuery("").Encode(); got != "" {
		t.Fatalf("empty orgId must be omitted, got %q", got)
	}
	if got := orgQuery(" 11111111-2222-3333-4444-555555555555 ").Get("orgId"); got != "11111111-2222-3333-4444-555555555555" {
		t.Fatalf("orgId = %q", got)
	}
}
