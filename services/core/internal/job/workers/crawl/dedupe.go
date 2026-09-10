package crawl

import (
	"net/url"
	"sort"
)

type dedupe struct {
	seen map[string]struct{}
}

func newDedupe() *dedupe {
	return &dedupe{seen: make(map[string]struct{})}
}

func (d *dedupe) claim(rawURL string) bool {
	key := dedupeKey(rawURL)
	if _, ok := d.seen[key]; ok {
		return false
	}
	d.seen[key] = struct{}{}
	return true
}

func dedupeKey(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.Fragment = ""
	u.RawFragment = ""
	u.RawQuery = canonicalQuery(u.Query())
	return u.String()
}

func canonicalQuery(q url.Values) string {
	for _, values := range q {
		sort.Strings(values)
	}
	return q.Encode()
}
