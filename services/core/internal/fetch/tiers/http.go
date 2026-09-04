package tiers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pobear/pobear/pkg/fault"
	"github.com/pobear/pobear/pkg/jobspec"
)

func doHTTPFetch(ctx context.Context, client *http.Client, cfg jobspec.Fetch, url string) (*Result, fault.Kind, error) {
	if cfg.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Timeout); err == nil {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, d)
			defer cancel()
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fault.Network, err
	}
	applyHeaders(req, cfg.Headers)

	if cfg.NoRedirects {
		c := *client
		c.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
		client = &c
	}

	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fault.Timeout, err
		}
		return nil, fault.Network, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fault.Network, err
	}

	result := &Result{Status: resp.StatusCode, Body: body, FinalURL: resp.Request.URL.String()}
	kind, classifyErr := classifyStatus(resp.StatusCode, cfg.NoRedirects)
	return result, kind, classifyErr
}

func applyHeaders(req *http.Request, headers map[string]string) {
	for k, v := range headers {
		req.Header.Set(k, v)
	}
}

func classifyStatus(status int, noRedirects bool) (fault.Kind, error) {
	if noRedirects && status >= 300 && status < 400 {
		return "", nil
	}
	kind := classifyDefaultStatus(status)
	if kind == "" {
		return "", nil
	}
	return kind, fmt.Errorf("tiers: http status %d", status)
}

func classifyDefaultStatus(status int) fault.Kind {
	switch {
	case status >= 200 && status < 300:
		return ""
	case status == 403:
		return fault.Blocked
	case status == 429:
		return fault.RateLimit
	default:
		return fault.HTTP
	}
}
