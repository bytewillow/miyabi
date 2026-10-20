package pan

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/ppxb/miyabi/internal/domain"
)

func TestRateGateTripsOnRateLimitAndFailsFast(t *testing.T) {
	gate := &rateGate{}
	if err := gate.check(); err != nil {
		t.Fatalf("open gate blocked a request: %v", err)
	}
	gate.observe(&apiError{Code: RateLimitCode, Message: "已达到当前访问上限"})
	err := gate.check()
	var retryErr *domain.RetryError
	if !errors.As(err, &retryErr) {
		t.Fatalf("closed gate error = %v, want a RetryError", err)
	}
	if retryErr.After <= 0 || retryErr.After > rateLimitBaseBackoff {
		t.Fatalf("gate wait = %v, want within (0, %v]", retryErr.After, rateLimitBaseBackoff)
	}
	if !IsRateLimited(err) {
		t.Fatalf("gate error not recognized as rate limit: %v", err)
	}
}

func TestRateGateBackoffDoublesAndCaps(t *testing.T) {
	gate := &rateGate{}
	limited := &apiError{Code: RateLimitCode}
	gate.observe(limited)
	if gate.backoff != rateLimitBaseBackoff {
		t.Fatalf("first backoff = %v, want %v", gate.backoff, rateLimitBaseBackoff)
	}
	gate.blockedUntil = time.Now().Add(-time.Second)
	gate.observe(limited)
	if gate.backoff != 2*rateLimitBaseBackoff {
		t.Fatalf("second backoff = %v, want %v", gate.backoff, 2*rateLimitBaseBackoff)
	}
	gate.backoff = rateLimitMaxBackoff
	gate.blockedUntil = time.Now().Add(-time.Second)
	gate.observe(limited)
	if gate.backoff != rateLimitMaxBackoff {
		t.Fatalf("backoff = %v, want cap %v", gate.backoff, rateLimitMaxBackoff)
	}
}

func TestRateGateSuccessRearmsAndOtherErrorsDoNotTrip(t *testing.T) {
	gate := &rateGate{}
	gate.observe(errors.New("boom"))
	if err := gate.check(); err != nil {
		t.Fatalf("non-limit error tripped the gate: %v", err)
	}
	gate.observe(&apiError{Code: RateLimitCode})
	gate.observe(nil)
	if gate.backoff != 0 {
		t.Fatalf("success did not re-arm the backoff: %v", gate.backoff)
	}
}

// One rate-limited response must stop the next request before it reaches
// the network: that pause is what lets the account limit clear.
func TestAPIRequestGateFailsFastWithoutNetwork(t *testing.T) {
	var hits atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"state":false,"code":770004,"message":"已达到当前访问上限，请稍后再试","data":[]}`))
	}))
	defer ts.Close()
	client := &Client{http: resty.New()}
	if _, err := apiRequest[accountShapeWire](client, client.http.R().SetContext(context.Background()), http.MethodGet, ts.URL, "account"); !IsRateLimited(err) {
		t.Fatalf("first request error = %v, want rate limit", err)
	}
	if _, err := apiRequest[accountShapeWire](client, client.http.R().SetContext(context.Background()), http.MethodGet, ts.URL, "account"); !IsRateLimited(err) {
		t.Fatalf("second request error = %v, want rate limit", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("server hits = %d, want 1 (second request must fail at the gate)", hits.Load())
	}
}
