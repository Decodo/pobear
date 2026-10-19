package extractors

import (
	"context"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/cascadia"

	"github.com/pobear/pobear/pkg/jobspec"
)

type CSS struct{}

func (CSS) Run(ctx context.Context, body []byte, cfg jobspec.Stage) ([]Result, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(cfg.Fields))
	for name := range cfg.Fields {
		names = append(names, name)
	}
	sort.Strings(names)

	results := make([]Result, 0, len(names))
	for _, name := range names {
		cssSelector, attr, _ := strings.Cut(cfg.Fields[name], "@")

		matcher, err := cascadia.Compile(cssSelector)
		if err != nil {
			return nil, err
		}

		values := []string{}
		doc.FindMatcher(matcher).Each(func(_ int, s *goquery.Selection) {
			if attr == "" {
				values = append(values, s.Text())
				return
			}
			if v, ok := s.Attr(attr); ok {
				values = append(values, v)
			}
		})

		results = append(results, Result{Key: name, Value: values})
	}
	return results, nil
}
