package camera

import (
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"strings"
	"time"
)

// MJPEGClient consumes a multipart/x-mixed-replace MJPEG stream and yields JPEG frames.
type MJPEGClient struct {
	URL    string
	Client *http.Client
}

// NewMJPEGClient creates a client with sensible timeouts.
func NewMJPEGClient(url string) *MJPEGClient {
	return &MJPEGClient{
		URL: url,
		Client: &http.Client{
			Timeout: 0, // stream is long-lived; per-req timeouts set via Transport/Context
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				IdleConnTimeout:       90 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				MaxIdleConns:          10,
				MaxConnsPerHost:       10,
			},
		},
	}
}

// Stream connects and continuously sends JPEG frames on frames chan.
// It auto-reconnects on errors with backoff until ctx is done.
func (m *MJPEGClient) Stream(ctx context.Context, frames chan<- []byte) error {
	defer close(frames)
	backoff := 500 * time.Millisecond
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.URL, nil)
		if err != nil {
			return fmt.Errorf("mjpeg request: %w", err)
		}
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("User-Agent", "Garage48-MJPEG-Client/1.0")
		req.Header.Set("Accept", "image/jpeg, multipart/x-mixed-replace, */*")

		resp, err := m.Client.Do(req)
		if err != nil {
			// transient network error, back off and retry
			select {
			case <-time.After(backoff):
				backoff = minDur(backoff*2, 10*time.Second)
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		backoff = 500 * time.Millisecond // reset on successful connect

		ct := resp.Header.Get("Content-Type")
		mediaType, params, err := mime.ParseMediaType(ct)
		if err != nil || !(mediaType == "multipart/x-mixed-replace" || strings.HasPrefix(mediaType, "multipart/")) {
			// Some servers use multipart/mixed or set odd casing; we accept any multipart/*
			// but still require a boundary.
		}
		boundary := strings.TrimSpace(params["boundary"])
		if boundary == "" {
			resp.Body.Close()
			return fmt.Errorf("missing boundary in content-type: %q", ct)
		}
		// Some servers (e.g., Android IP Webcam variants) include leading "--" in boundary parameter.
		// Go's multipart reader expects the boundary without leading dashes.
		if strings.HasPrefix(boundary, "--") {
			boundary = strings.TrimPrefix(boundary, "--")
		}

		mr := multipart.NewReader(resp.Body, boundary)
		for {
			if ctx.Err() != nil {
				resp.Body.Close()
				return ctx.Err()
			}
			part, err := mr.NextPart()
			if err != nil {
				resp.Body.Close()
				// EOF or transient; reconnect
				break
			}
			// Many servers set part headers like:
			// Content-Type: image/jpeg
			// Content-Length: N
			buf, err := io.ReadAll(part)
			_ = part.Close()
			if err != nil {
				// broken frame; continue to next part
				continue
			}
			// Deliver frame
			select {
			case frames <- buf:
			case <-ctx.Done():
				resp.Body.Close()
				return ctx.Err()
			}
		}
		// reconnect loop with backoff
		select {
		case <-time.After(backoff):
			backoff = minDur(backoff*2, 10*time.Second)
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func minDur(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
