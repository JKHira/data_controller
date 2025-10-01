package restapi

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

// EndpointType represents different REST API endpoint categories
type EndpointType string

const (
	EndpointCandles EndpointType = "candles"
	EndpointTrades  EndpointType = "trades"
	EndpointTickers EndpointType = "tickers"
)

// SafeRateLimiter manages rate limiting with safety buffers
type SafeRateLimiter struct {
	limiters map[EndpointType]*rate.Limiter
}

// NewSafeRateLimiter creates a rate limiter with 20% safety buffer
// Bitfinex limits:
// - Candles: 30 req/min → 24 req/min (80%)
// - Trades: 15 req/min → 12 req/min (80%)
// - Tickers: 10 req/min → 8 req/min (80%)
func NewSafeRateLimiter() *SafeRateLimiter {
	const safetyFactor = 0.8 // 20% buffer

	return &SafeRateLimiter{
		limiters: map[EndpointType]*rate.Limiter{
			// Candles: 30/min * 0.8 = 24/min = 2.5 seconds per request
			EndpointCandles: rate.NewLimiter(rate.Every(time.Duration(float64(time.Minute)/24.0)), 1),

			// Trades: 15/min * 0.8 = 12/min = 5 seconds per request
			EndpointTrades: rate.NewLimiter(rate.Every(time.Duration(float64(time.Minute)/12.0)), 1),

			// Tickers: 10/min * 0.8 = 8/min = 7.5 seconds per request
			EndpointTickers: rate.NewLimiter(rate.Every(time.Duration(float64(time.Minute)/8.0)), 1),
		},
	}
}

// Wait waits for the rate limiter to allow the request
func (s *SafeRateLimiter) Wait(ctx context.Context, endpoint EndpointType) error {
	limiter, ok := s.limiters[endpoint]
	if !ok {
		// Unknown endpoint, use most conservative limit (trades)
		limiter = s.limiters[EndpointTrades]
	}

	return limiter.Wait(ctx)
}

// Allow checks if a request is allowed without waiting
func (s *SafeRateLimiter) Allow(endpoint EndpointType) bool {
	limiter, ok := s.limiters[endpoint]
	if !ok {
		return false
	}

	return limiter.Allow()
}

// GetLimitInfo returns human-readable rate limit info
func (s *SafeRateLimiter) GetLimitInfo(endpoint EndpointType) string {
	switch endpoint {
	case EndpointCandles:
		return "24 req/min (30 req/min with 20% buffer)"
	case EndpointTrades:
		return "12 req/min (15 req/min with 20% buffer)"
	case EndpointTickers:
		return "8 req/min (10 req/min with 20% buffer)"
	default:
		return "Unknown endpoint"
	}
}

// ResetBurst resets burst capacity (useful for testing)
func (s *SafeRateLimiter) ResetBurst() {
	for _, limiter := range s.limiters {
		limiter.SetBurst(1)
	}
}
