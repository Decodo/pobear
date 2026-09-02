package jobspec

import (
	"net/url"

	"github.com/bmatcuk/doublestar/v4"
)

func parseSource(root map[string]any) (Source, *ValidationError) {
	raw, ok := root["source"]
	if !ok {
		return Source{}, vErr("source", "`source` is required")
	}
	m, err := asMapping(raw, "source", "urls", "crawl")
	if err != nil {
		return Source{}, err
	}

	_, hasURLs := m["urls"]
	_, hasCrawl := m["crawl"]
	switch {
	case hasURLs && hasCrawl:
		return Source{}, vErr("source", "`source` must set exactly one of `urls` or `crawl`, but both are set")
	case !hasURLs && !hasCrawl:
		return Source{}, vErr("source", "`source` must set exactly one of `urls` or `crawl`, but neither is set")
	case hasURLs:
		urls, err := parseURLList(m["urls"], "source.urls")
		if err != nil {
			return Source{}, err
		}
		return Source{URLs: urls}, nil
	default:
		crawl, err := parseCrawlSource(m["crawl"])
		if err != nil {
			return Source{}, err
		}
		return Source{Crawl: crawl}, nil
	}
}

func parseCrawlSource(raw any) (*CrawlSource, *ValidationError) {
	m, err := asMapping(raw, "source.crawl", "seeds", "depth", "maxPages", "maxDuration", "scope", "include", "exclude", "sitemap", "robots")
	if err != nil {
		return nil, err
	}

	seeds, err := parseURLList(m["seeds"], "source.crawl.seeds")
	if err != nil {
		return nil, err
	}

	depth := getInt(m, "depth", 0)
	if depth < 0 {
		return nil, vErr("source.crawl.depth", "`source.crawl.depth` must be >= 0, got %d", depth)
	}

	maxPages := getInt(m, "maxPages", 100)
	if maxPages <= 0 {
		return nil, vErr("source.crawl.maxPages", "`source.crawl.maxPages` must be > 0, got %d", maxPages)
	}

	maxDuration := getString(m, "maxDuration", "")
	if maxDuration != "" && !validDuration(maxDuration) {
		return nil, vErr("source.crawl.maxDuration", "`source.crawl.maxDuration` is not a valid duration: %q", maxDuration)
	}

	scope := getString(m, "scope", "same-host")
	if err := enumField("source.crawl.scope", scope, "same-host", "same-domain", "any"); err != nil {
		return nil, err
	}

	include, err := parseGlobList(m, "include", "source.crawl.include")
	if err != nil {
		return nil, err
	}
	exclude, err := parseGlobList(m, "exclude", "source.crawl.exclude")
	if err != nil {
		return nil, err
	}

	robots := getString(m, "robots", "respect")
	if err := enumField("source.crawl.robots", robots, "respect", "ignore"); err != nil {
		return nil, err
	}

	return &CrawlSource{
		Seeds:       seeds,
		Depth:       depth,
		MaxPages:    maxPages,
		MaxDuration: maxDuration,
		Scope:       scope,
		Include:     include,
		Exclude:     exclude,
		Sitemap:     getBool(m, "sitemap", false),
		Robots:      robots,
	}, nil
}

// parseURLList requires a non-empty list of absolute http(s) URLs — used for
// both source.urls and source.crawl.seeds, which share the exact same rule.
func parseURLList(raw any, field string) ([]string, *ValidationError) {
	list, _ := raw.([]any)
	if len(list) == 0 {
		return nil, vErr(field, "`%s` must contain at least one URL, got none", field)
	}

	urls := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			return nil, vErr(field, "`%s` must be absolute `http`/`https` URLs, got %v", field, item)
		}
		if !isAbsoluteHTTPURL(s) {
			return nil, vErr(field, "`%s` must be absolute `http`/`https` URLs, got %q", field, s)
		}
		urls = append(urls, s)
	}
	return urls, nil
}

// parseGlobList validates a source.crawl.include/exclude list's doublestar
// glob syntax — it does not match URLs, that's crawl's own runtime job.
func parseGlobList(m map[string]any, key, field string) ([]string, *ValidationError) {
	raw, ok := m[key]
	if !ok {
		return []string{}, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, vErr(field, "`%s` must be a list", field)
	}

	globs := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			return nil, vErr(field, "`%s` is not a valid glob pattern: %v", field, item)
		}
		if !doublestar.ValidatePattern(s) {
			return nil, vErr(field, "`%s` is not a valid glob pattern: %q", field, s)
		}
		globs = append(globs, s)
	}
	return globs, nil
}

func isAbsoluteHTTPURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.IsAbs() && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
