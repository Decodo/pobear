package crawl

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type sitemapURLSet struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []struct {
		Loc string `xml:"loc"`
	} `xml:"url"`
}

type sitemapIndex struct {
	XMLName  xml.Name `xml:"sitemapindex"`
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

type HTTPSitemapFetcher struct {
	Client *http.Client
}

func NewHTTPSitemapFetcher(client *http.Client) *HTTPSitemapFetcher {
	return &HTTPSitemapFetcher{Client: client}
}

func (f *HTTPSitemapFetcher) URLs(ctx context.Context, rawURL string) ([]string, error) {
	origin, err := sitemapOrigin(rawURL)
	if err != nil {
		return nil, err
	}

	body, err := f.fetch(ctx, origin+"/sitemap.xml")
	if err != nil {
		return nil, err
	}

	return f.parse(ctx, body)
}

func sitemapOrigin(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return u.Scheme + "://" + u.Host, nil
}

func (f *HTTPSitemapFetcher) fetch(ctx context.Context, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("crawl: fetching %s: status %d", target, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (f *HTTPSitemapFetcher) parse(ctx context.Context, body []byte) ([]string, error) {
	var idx sitemapIndex
	if err := xml.Unmarshal(body, &idx); err == nil {
		var all []string
		for _, s := range idx.Sitemaps {
			childBody, ferr := f.fetch(ctx, s.Loc)
			if ferr != nil {
				continue
			}
			var set sitemapURLSet
			if perr := xml.Unmarshal(childBody, &set); perr != nil {
				continue
			}
			for _, u := range set.URLs {
				all = append(all, u.Loc)
			}
		}
		return all, nil
	}

	var set sitemapURLSet
	if err := xml.Unmarshal(body, &set); err != nil {
		return nil, err
	}
	urls := make([]string, 0, len(set.URLs))
	for _, u := range set.URLs {
		urls = append(urls, u.Loc)
	}
	return urls, nil
}
