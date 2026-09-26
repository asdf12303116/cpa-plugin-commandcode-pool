package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

// CommandCode exposes its usage data through undocumented but CLI-stable routes
// under /alpha/*. They authenticate with the same provider API key as
// inference, via either `Authorization: Bearer <key>` or `x-api-key: <key>`.
const (
	pathWhoami        = "/alpha/whoami"
	pathSubscriptions = "/alpha/billing/subscriptions"
	pathCredits       = "/alpha/billing/credits"
	pathUsageSummary  = "/alpha/usage/summary"
)

// httpDoer executes a host-style HTTP request. It defaults to the CPA host
// callback and is replaceable so the client can be exercised outside CPA.
type httpDoer func(pluginapi.HTTPRequest) (pluginapi.HTTPResponse, error)

type apiClient struct {
	baseURL   string
	userAgent string
	do        httpDoer
}

func newAPIClient(cfg settings) *apiClient {
	return newAPIClientWithDoer(cfg, hostHTTPDo)
}

func newAPIClientWithDoer(cfg settings, do httpDoer) *apiClient {
	if do == nil {
		do = hostHTTPDo
	}
	return &apiClient{
		baseURL:   strings.TrimRight(strings.TrimSpace(cfg.APIBaseURL), "/"),
		userAgent: cfg.UserAgent,
		do:        do,
	}
}

// apiError describes a structured CommandCode error response.
type apiError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *apiError) Error() string {
	switch {
	case e.Code != "" && e.Message != "":
		return fmt.Sprintf("HTTP %d %s: %s", e.StatusCode, e.Code, e.Message)
	case e.Message != "":
		return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
	default:
		return fmt.Sprintf("HTTP %d", e.StatusCode)
	}
}

// get performs an authenticated GET against the CommandCode API and returns the
// raw body, or an *apiError for non-2xx responses.
func (c *apiClient) get(apiKey, path string, query url.Values) ([]byte, error) {
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	resp, errDo := c.do(pluginapi.HTTPRequest{
		Method: http.MethodGet,
		URL:    target,
		Headers: http.Header{
			"Authorization": []string{"Bearer " + apiKey},
			"X-Api-Key":     []string{apiKey},
			"Accept":        []string{"application/json"},
			"User-Agent":    []string{c.userAgent},
		},
	})
	if errDo != nil {
		return nil, fmt.Errorf("request %s: %w", path, errDo)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp.StatusCode, resp.Body)
	}
	return resp.Body, nil
}

// parseAPIError decodes both documented error shapes: the wrapped
// {"success":false,"error":{...}} form and the flat
// {"success":false,"status":...,"message":...} form used for unknown routes.
func parseAPIError(status int, body []byte) error {
	out := &apiError{StatusCode: status}
	var wrapped struct {
		Success bool `json:"success"`
		Error   *struct {
			Code    string `json:"code"`
			Status  int    `json:"status"`
			Message string `json:"message"`
		} `json:"error"`
		Status  int    `json:"status"`
		Message string `json:"message"`
	}
	if errUnmarshal := json.Unmarshal(body, &wrapped); errUnmarshal == nil {
		if wrapped.Error != nil {
			out.Code = strings.TrimSpace(wrapped.Error.Code)
			out.Message = strings.TrimSpace(wrapped.Error.Message)
			if wrapped.Error.Status > 0 {
				out.StatusCode = wrapped.Error.Status
			}
		} else if strings.TrimSpace(wrapped.Message) != "" {
			out.Message = strings.TrimSpace(wrapped.Message)
		}
	}
	if out.Message == "" {
		snippet := strings.TrimSpace(string(body))
		if len(snippet) > 160 {
			snippet = snippet[:160] + "…"
		}
		if snippet == "" {
			snippet = "empty response body"
		}
		out.Message = snippet
	}
	return out
}

type whoamiResponse struct {
	Success bool            `json:"success"`
	User    whoamiUser      `json:"user"`
	Org     *whoamiOrg      `json:"org"`
	Limits  json.RawMessage `json:"limits,omitempty"`
}

type whoamiUser struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	UserName string `json:"userName"`
}

type whoamiOrg struct {
	ID string `json:"id"`
}

type subscriptionEnvelope struct {
	Success bool              `json:"success"`
	Data    *subscriptionData `json:"data"`
}

type subscriptionData struct {
	ID                 string            `json:"id"`
	Status             string            `json:"status"`
	UserID             string            `json:"userId"`
	OrgID              *string           `json:"orgId"`
	CreatedAt          string            `json:"createdAt"`
	PriceID            string            `json:"priceId"`
	Metadata           map[string]string `json:"metadata"`
	Quantity           int               `json:"quantity"`
	CancelAtPeriodEnd  bool              `json:"cancelAtPeriodEnd"`
	CurrentPeriodStart string            `json:"currentPeriodStart"`
	CurrentPeriodEnd   string            `json:"currentPeriodEnd"`
	EndedAt            *string           `json:"endedAt"`
	CancelAt           *string           `json:"cancelAt"`
	CanceledAt         *string           `json:"canceledAt"`
	PlanID             string            `json:"planId"`
	PendingPhase       json.RawMessage   `json:"pendingPhase"`
}

type creditsResponse struct {
	Credits        *creditBalances `json:"credits"`
	WindowLimits   *windowLimits   `json:"windowLimits"`
	SandboxAccess  *bool           `json:"sandboxAccess"`
	SandboxMinutes *float64        `json:"sandboxMinutes"`
}

type creditBalances struct {
	BelowThreshold   bool    `json:"belowThreshold"`
	CreditThreshold  float64 `json:"creditThreshold"`
	MonthlyCredits   float64 `json:"monthlyCredits"`
	PurchasedCredits float64 `json:"purchasedCredits"`
	FreeCredits      float64 `json:"freeCredits"`
}

type windowLimits struct {
	Limited  *bool        `json:"limited"`
	Exceeded *bool        `json:"exceeded"`
	FiveHour *limitWindow `json:"fiveHour"`
	Weekly   *limitWindow `json:"weekly"`
}

type limitWindow struct {
	Used     float64 `json:"used"`
	Cap      float64 `json:"cap"`
	Exceeded *bool   `json:"exceeded"`
	ResetAt  int64   `json:"resetAt"`
}

type usageSummary struct {
	TotalCount            int64   `json:"totalCount"`
	TotalCost             float64 `json:"totalCost"`
	AverageCost           float64 `json:"averageCost"`
	SuccessRate           float64 `json:"successRate"`
	CompletedCount        int64   `json:"completedCount"`
	FailedCount           int64   `json:"failedCount"`
	TotalTokensIn         int64   `json:"totalTokensIn"`
	TotalTokensOut        int64   `json:"totalTokensOut"`
	TotalTokens           int64   `json:"totalTokens"`
	TotalCredits          float64 `json:"totalCredits"`
	TotalFreeCredits      float64 `json:"totalFreeCredits"`
	TotalMonthlyCredits   float64 `json:"totalMonthlyCredits"`
	TotalPurchasedCredits float64 `json:"totalPurchasedCredits"`
	PeriodBasis           string  `json:"periodBasis"`
}

func (c *apiClient) whoami(apiKey string) (*whoamiResponse, error) {
	body, errGet := c.get(apiKey, pathWhoami, url.Values{"limits": []string{"1"}})
	if errGet != nil {
		return nil, errGet
	}
	var out whoamiResponse
	if errUnmarshal := json.Unmarshal(body, &out); errUnmarshal != nil {
		return nil, fmt.Errorf("parse whoami response: %w", errUnmarshal)
	}
	if !out.Success {
		return nil, fmt.Errorf("whoami reported success=false")
	}
	return &out, nil
}

// subscription returns the active subscription, or (nil, nil) when the account
// has none. The caller must not fabricate a planId in that case.
func (c *apiClient) subscription(apiKey, orgID string) (*subscriptionData, error) {
	body, errGet := c.get(apiKey, pathSubscriptions, orgQuery(orgID))
	if errGet != nil {
		return nil, errGet
	}
	var env subscriptionEnvelope
	if errUnmarshal := json.Unmarshal(body, &env); errUnmarshal != nil {
		return nil, fmt.Errorf("parse subscriptions response: %w", errUnmarshal)
	}
	if env.Data == nil {
		return nil, nil
	}
	return env.Data, nil
}

func (c *apiClient) credits(apiKey, orgID string) (*creditsResponse, error) {
	body, errGet := c.get(apiKey, pathCredits, orgQuery(orgID))
	if errGet != nil {
		return nil, errGet
	}
	var out creditsResponse
	if errUnmarshal := json.Unmarshal(body, &out); errUnmarshal != nil {
		return nil, fmt.Errorf("parse credits response: %w", errUnmarshal)
	}
	return &out, nil
}

func (c *apiClient) summary(apiKey, orgID, since string) (*usageSummary, error) {
	query := orgQuery(orgID)
	if strings.TrimSpace(since) != "" {
		query.Set("since", strings.TrimSpace(since))
	}
	body, errGet := c.get(apiKey, pathUsageSummary, query)
	if errGet != nil {
		return nil, errGet
	}
	var out usageSummary
	if errUnmarshal := json.Unmarshal(body, &out); errUnmarshal != nil {
		return nil, fmt.Errorf("parse usage summary response: %w", errUnmarshal)
	}
	return &out, nil
}

// orgQuery builds the optional orgId query parameter. Personal accounts must
// omit it: an empty value is a 400 UUID validation error.
func orgQuery(orgID string) url.Values {
	query := url.Values{}
	if id := strings.TrimSpace(orgID); id != "" {
		query.Set("orgId", id)
	}
	return query
}

// epochMillisToTime converts CommandCode's millisecond epoch resetAt values.
func epochMillisToTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}
