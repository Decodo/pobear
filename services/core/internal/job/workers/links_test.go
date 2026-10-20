package workers_test

import (
	"sort"
	"testing"

	"github.com/Decodo/pobear/services/core/internal/job/workers"
)

func TestHTMLLinksResolvesRelativeHrefsAgainstBaseURL(t *testing.T) {
	body := []byte(`<a href="/about">About</a><a href="page.html">Page</a>`)

	got := workers.HTMLLinks{}.Extract(body, "https://a.test/blog/index.html")

	sort.Strings(got)
	want := []string{"https://a.test/about", "https://a.test/blog/page.html"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestHTMLLinksResolvesAgainstFinalURLNotOriginalRequest(t *testing.T) {
	body := []byte(`<a href="sibling.html">Sibling</a>`)

	got := workers.HTMLLinks{}.Extract(body, "https://a.test/redirected-to/page.html")

	if len(got) != 1 || got[0] != "https://a.test/redirected-to/sibling.html" {
		t.Fatalf("got %v, want [https://a.test/redirected-to/sibling.html] (resolved against the final URL, not wherever the request originally targeted)", got)
	}
}

func TestHTMLLinksKeepsAlreadyAbsoluteHrefs(t *testing.T) {
	body := []byte(`<a href="https://other.test/page">Other</a>`)

	got := workers.HTMLLinks{}.Extract(body, "https://a.test/")

	if len(got) != 1 || got[0] != "https://other.test/page" {
		t.Fatalf("got %v, want [https://other.test/page]", got)
	}
}

func TestHTMLLinksDropsNonHTTPSchemes(t *testing.T) {
	body := []byte(`<a href="mailto:a@b.test">Mail</a><a href="tel:+1234">Tel</a><a href="javascript:void(0)">JS</a><a href="/real">Real</a>`)

	got := workers.HTMLLinks{}.Extract(body, "https://a.test/")

	if len(got) != 1 || got[0] != "https://a.test/real" {
		t.Fatalf("got %v, want only [https://a.test/real] -- mailto/tel/javascript hrefs were never crawlable candidates", got)
	}
}

func TestHTMLLinksIgnoresAnchorsWithoutHref(t *testing.T) {
	body := []byte(`<a name="section">No href</a><a href="/real">Real</a>`)

	got := workers.HTMLLinks{}.Extract(body, "https://a.test/")

	if len(got) != 1 || got[0] != "https://a.test/real" {
		t.Fatalf("got %v, want only [https://a.test/real]", got)
	}
}

func TestHTMLLinksReturnsNilOnUnparsableBaseURL(t *testing.T) {
	body := []byte(`<a href="/real">Real</a>`)

	got := workers.HTMLLinks{}.Extract(body, "://not a url")

	if got != nil {
		t.Fatalf("got %v, want nil for an unparsable baseURL", got)
	}
}

func TestHTMLLinksReturnsNilWhenNoLinksPresent(t *testing.T) {
	got := workers.HTMLLinks{}.Extract([]byte(`<p>no links here</p>`), "https://a.test/")

	if got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}
