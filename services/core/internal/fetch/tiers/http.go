package tiers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html/charset"

	"github.com/Decodo/pobear/pkg/fault"
	"github.com/Decodo/pobear/pkg/jobspec"
)

const bodySnippetLimit = 500

const maxRedirects = 10

func bodySnippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > bodySnippetLimit {
		return s[:bodySnippetLimit] + "…"
	}
	return s
}

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

	switch allow := redirectPolicyFrom(ctx); {
	case cfg.NoRedirects:
		c := *client
		c.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
		client = &c
	case allow != nil:
		c := *client
		c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if !allow(req.URL.String()) {
				return ErrRedirectBlocked
			}
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return nil
		}
		client = &c
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, transportFault(err), err
	}
	defer resp.Body.Close()

	body, err := readBodyAsUTF8(resp)
	if err != nil {
		return nil, transportFault(err), err
	}

	result := &Result{Status: resp.StatusCode, Body: body, FinalURL: resp.Request.URL.String()}
	kind, classifyErr := classifyStatus(resp.StatusCode, cfg.NoRedirects, body)
	return result, kind, classifyErr
}

func transportFault(err error) fault.Kind {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return fault.Timeout
	}
	return fault.Network
}

func applyHeaders(req *http.Request, headers map[string]string) {
	for k, v := range headers {
		req.Header.Set(k, v)
	}
}

var blockPageMarkers = [][]byte{
	[]byte("validateCaptcha"),      // Amazon
	[]byte("_cf_chl_opt"),          // Cloudflare challenge
	[]byte("captcha-delivery.com"), // DataDome
	[]byte("px-captcha"),           // PerimeterX
}

func isBlockPage(body []byte) bool {
	for _, m := range blockPageMarkers {
		if bytes.Contains(body, m) {
			return true
		}
	}
	return false
}

func classifyStatus(status int, noRedirects bool, body []byte) (fault.Kind, error) {
	if noRedirects && status >= 300 && status < 400 {
		return "", nil
	}
	kind := classifyDefaultStatus(status)
	switch {
	case status == http.StatusServiceUnavailable:
		kind = fault.Blocked
	case kind == "" && isBlockPage(body):
		return fault.Blocked, fmt.Errorf("tiers: http status %d: block page detected", status)
	case kind == "":
		return "", nil
	}
	if snippet := bodySnippet(body); snippet != "" {
		return kind, fmt.Errorf("tiers: http status %d: %s", status, snippet)
	}
	return kind, fmt.Errorf("tiers: http status %d", status)
}

func classifyDefaultStatus(status int) fault.Kind {
	switch {
	case status >= 200 && status < 300:
		return ""
	case status == 401 || status == 407:
		return fault.Auth
	case status == 403:
		return fault.Blocked
	case status == 429:
		return fault.RateLimit
	default:
		return fault.HTTP
	}
}

func readBodyAsUTF8(resp *http.Response) ([]byte, error) {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	contentType := resp.Header.Get("Content-Type")
	if !isTextualContentType(contentType) {
		return raw, nil
	}
	r, err := charset.NewReader(bytes.NewReader(raw), contentType)
	if err != nil {
		return raw, nil
	}
	decoded, err := io.ReadAll(r)
	if err != nil {
		return raw, nil
	}
	return decoded, nil
}

func isTextualContentType(contentType string) bool {
	if contentType == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return strings.HasPrefix(mediaType, "text/") ||
		mediaType == "application/xhtml+xml" ||
		mediaType == "application/xml"
}

type redirectPolicyKey struct{}

func WithRedirectPolicy(ctx context.Context, allow func(target string) bool) context.Context {
	if allow == nil {
		return ctx
	}
	return context.WithValue(ctx, redirectPolicyKey{}, allow)
}

func redirectPolicyFrom(ctx context.Context) func(string) bool {
	allow, _ := ctx.Value(redirectPolicyKey{}).(func(string) bool)
	return allow
}

var ErrRedirectBlocked = errors.New("redirect target is not allowed")
