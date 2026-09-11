package crawl

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"sync"

	"github.com/temoto/robotstxt"
)

const robotsUserAgent = "*"

type HTTPRobotsChecker struct {
	Client *http.Client
	mu     sync.Mutex
	cache  map[string]*robotstxt.RobotsData
}

func NewHTTPRobotsChecker(client *http.Client) *HTTPRobotsChecker {
	return &HTTPRobotsChecker{Client: client, cache: make(map[string]*robotstxt.RobotsData)}
}

func (c *HTTPRobotsChecker) Allowed(ctx context.Context, rawURL string) (bool, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false, err
	}

	origin := u.Scheme + "://" + u.Host
	c.mu.Lock()
	data, ok := c.cache[origin]
	c.mu.Unlock()

	if !ok {
		data = c.fetch(ctx, origin+"/robots.txt")
		c.mu.Lock()
		c.cache[origin] = data
		c.mu.Unlock()
	}

	path := u.RequestURI()
	if path == "" {
		path = "/"
	}
	return data.TestAgent(path, robotsUserAgent), nil
}

func (c *HTTPRobotsChecker) fetch(ctx context.Context, robotsURL string) *robotstxt.RobotsData {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, nil)
	if err != nil {
		return blockedRobotsData()
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return blockedRobotsData()
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return blockedRobotsData()
	}
	data, err := robotstxt.FromStatusAndBytes(resp.StatusCode, body)
	if err != nil {
		return blockedRobotsData()
	}
	return data
}

func blockedRobotsData() *robotstxt.RobotsData {
	data, _ := robotstxt.FromStatusAndBytes(http.StatusServiceUnavailable, nil)
	return data
}
