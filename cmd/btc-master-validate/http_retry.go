package main

import (
	"context"
	"net/http"
	"time"
)

// The automated BTC report occasionally sees a transient Binance header
// timeout. Keep the existing 20s http.Client deadline in fetch15M unchanged,
// but split that budget into a few bounded transport attempts. This affects
// data retrieval only; research, detector, execution and cost rules are unchanged.
func init() {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return
	}
	http.DefaultTransport = &boundedRetryTransport{base: base.Clone()}
}

type boundedRetryTransport struct {
	base http.RoundTripper
}

func (t *boundedRetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	const attempts = 3
	const attemptTimeout = 6 * time.Second

	var lastErr error
	for i := 0; i < attempts; i++ {
		ctx, cancel := context.WithTimeout(req.Context(), attemptTimeout)
		attemptReq := req.Clone(ctx)
		resp, err := t.base.RoundTrip(attemptReq)
		cancel()
		if err == nil {
			return resp, nil
		}
		lastErr = err

		// Do not retry when the parent request itself has already expired or
		// been cancelled. Otherwise use a small bounded backoff before retrying.
		if req.Context().Err() != nil || i == attempts-1 {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	return nil, lastErr
}
