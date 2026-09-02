package main

import (
	"context"
	"io"
	"net/http"
	"time"
)

// The automated BTC report occasionally sees transient Binance request stalls.
// Keep retries bounded inside the existing client deadline. This affects data
// retrieval only; research, detector, execution and cost rules are unchanged.
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

type cancelOnCloseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnCloseBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

func (t *boundedRetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	const attempts = 3
	const attemptTimeout = 6 * time.Second

	var lastErr error
	for i := 0; i < attempts; i++ {
		ctx, cancel := context.WithTimeout(req.Context(), attemptTimeout)
		attemptReq := req.Clone(ctx)
		resp, err := t.base.RoundTrip(attemptReq)
		if err == nil {
			// RoundTrip returns before the response body is consumed. Keep the
			// attempt context alive until the caller closes the body; cancelling
			// here would make io.ReadAll fail with "context canceled".
			resp.Body = &cancelOnCloseBody{ReadCloser: resp.Body, cancel: cancel}
			return resp, nil
		}
		cancel()
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
