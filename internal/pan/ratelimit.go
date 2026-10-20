package pan

import (
	"sync"
	"time"

	"github.com/ppxb/miyabi/internal/domain"
)

// rateGate is the client-wide circuit breaker for 115's account-level rate
// limit (error 770004). The limit only clears while the account is left
// alone, so the first limited response closes the gate for every later
// request instead of letting independent callers keep the window saturated.
// The backoff doubles per trip, from one minute up to a fifteen-minute cap.
type rateGate struct {
	mu           sync.Mutex
	blockedUntil time.Time
	backoff      time.Duration
}

const (
	rateLimitBaseBackoff = time.Minute
	rateLimitMaxBackoff  = 15 * time.Minute
)

// check fails fast while the gate is closed, reporting the remaining wait
// as a retryable rate-limit error so callers back off rather than queue
// more requests against a limited account.
func (g *rateGate) check() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if remaining := time.Until(g.blockedUntil); remaining > 0 {
		return &domain.RetryError{
			Cause: &apiError{Code: RateLimitCode, Message: "115 rate limit in effect"},
			After: remaining,
		}
	}
	return nil
}

// observe records a request outcome: a rate-limit response trips the gate,
// a success re-arms the backoff for the next episode.
func (g *rateGate) observe(err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if err == nil {
		g.backoff = 0
		return
	}
	if !IsRateLimited(err) {
		return
	}
	if g.backoff < rateLimitBaseBackoff {
		g.backoff = rateLimitBaseBackoff
	} else {
		g.backoff = min(g.backoff*2, rateLimitMaxBackoff)
	}
	g.blockedUntil = time.Now().Add(g.backoff)
}
