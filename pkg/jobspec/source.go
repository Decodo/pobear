package jobspec

import (
	"github.com/bmatcuk/doublestar/v4"

	"github.com/pobear/pobear/pkg/utils"
)

func parseSource(root map[string]any) (Source, *ValidationError) {
	raw, ok := root["source"]
	if !ok {
		return Source{}, utils.Errorf("source", "`source` is required")
	}
	m, err := utils.AsMapping(raw, "source", "urls", "crawl")
	if err != nil {
		return Source{}, err
	}

	_, hasURLs := m["urls"]
	_, hasCrawl := m["crawl"]
	switch {
	case hasURLs && hasCrawl:
		return Source{}, utils.Errorf("source", "`source` must set exactly one of `urls` or `crawl`, but both are set")
	case !hasURLs && !hasCrawl:
		return Source{}, utils.Errorf("source", "`source` must set exactly one of `urls` or `crawl`, but neither is set")
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
	m, err := utils.AsMapping(raw, "source.crawl", "seeds", "depth", "maxPages", "maxDuration", "scope", "include", "exclude", "sitemap", "robots")
	if err != nil {
		return nil, err
	}

	seeds, err := parseURLList(m["seeds"], "source.crawl.seeds")
	if err != nil {
		return nil, err
	}

	depth := utils.GetInt(m, "depth", 0)
	if depth < 0 {
		return nil, utils.Errorf("source.crawl.depth", "`source.crawl.depth` must be >= 0, got %d", depth)
	}

	maxPages := utils.GetInt(m, "maxPages", 100)
	if maxPages <= 0 {
		return nil, utils.Errorf("source.crawl.maxPages", "`source.crawl.maxPages` must be > 0, got %d", maxPages)
	}

	maxDuration := utils.GetString(m, "maxDuration", "")
	if maxDuration != "" && !utils.ValidDuration(maxDuration) {
		return nil, utils.Errorf("source.crawl.maxDuration", "`source.crawl.maxDuration` is not a valid duration: %q", maxDuration)
	}

	scope := utils.GetString(m, "scope", "same-host")
	if err := utils.EnumField("source.crawl.scope", scope, "same-host", "same-domain", "any"); err != nil {
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

	robots := utils.GetString(m, "robots", "respect")
	if err := utils.EnumField("source.crawl.robots", robots, "respect", "ignore"); err != nil {
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
		Sitemap:     utils.GetBool(m, "sitemap", false),
		Robots:      robots,
	}, nil
}

func parseURLList(raw any, field string) ([]string, *ValidationError) {
	list, _ := raw.([]any)
	if len(list) == 0 {
		return nil, utils.Errorf(field, "`%s` must contain at least one URL, got none", field)
	}

	urls := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			return nil, utils.Errorf(field, "`%s` must be absolute `http`/`https` URLs, got %v", field, item)
		}
		if !utils.IsAbsoluteHTTPURL(s) {
			return nil, utils.Errorf(field, "`%s` must be absolute `http`/`https` URLs, got %q", field, s)
		}
		urls = append(urls, s)
	}
	return urls, nil
}

func parseGlobList(m map[string]any, key, field string) ([]string, *ValidationError) {
	raw, ok := m[key]
	if !ok {
		return []string{}, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, utils.Errorf(field, "`%s` must be a list", field)
	}

	globs := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			return nil, utils.Errorf(field, "`%s` is not a valid glob pattern: %v", field, item)
		}
		if !doublestar.ValidatePattern(s) {
			return nil, utils.Errorf(field, "`%s` is not a valid glob pattern: %q", field, s)
		}
		globs = append(globs, s)
	}
	return globs, nil
}
