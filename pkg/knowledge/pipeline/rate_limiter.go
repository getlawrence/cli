package pipeline

import (
	"time"
)

// RateLimiter provides rate limiting functionality for API requests
type RateLimiter struct {
	ticker *time.Ticker
	limit  int
	window time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		ticker: time.NewTicker(window / time.Duration(limit)),
		limit:  limit,
		window: window,
	}
}

// Wait waits for the next available time slot
func (r *RateLimiter) Wait() {
	<-r.ticker.C
}

// Stop stops the rate limiter
func (r *RateLimiter) Stop() {
	r.ticker.Stop()
}

// Reset resets the rate limiter with new parameters
func (r *RateLimiter) Reset(limit int, window time.Duration) {
	r.ticker.Stop()
	r.ticker = time.NewTicker(window / time.Duration(limit))
	r.limit = limit
	r.window = window
}
