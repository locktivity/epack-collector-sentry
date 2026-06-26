package sentry

import (
	"context"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

const maxAttempts = 5

var retryableStatuses = map[int]bool{
	http.StatusTooManyRequests:     true,
	http.StatusInternalServerError: true,
	http.StatusBadGateway:          true,
	http.StatusServiceUnavailable:  true,
	http.StatusGatewayTimeout:      true,
}

func isRetryable(statusCode int) bool {
	return retryableStatuses[statusCode]
}

func retryDelay(resp *http.Response, attempt int) time.Duration {
	if resp != nil {
		if after := resp.Header.Get("Retry-After"); after != "" {
			if seconds, err := strconv.Atoi(after); err == nil && seconds > 0 {
				return time.Duration(seconds) * time.Second
			}
		}
	}
	return backoff(attempt)
}

func backoff(attempt int) time.Duration {
	base := math.Pow(2, float64(attempt))
	jitter := rand.Float64() * base * 0.5
	return time.Duration(base+jitter) * time.Second
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
