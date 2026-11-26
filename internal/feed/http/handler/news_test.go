package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/clemilsonazevedo/look-news/internal/feed"
	"github.com/go-fuego/fuego"
)

func TestHandleNewsWithoutCriterion(t *testing.T) {
	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = fmt.Fprintf(w, `<rss version="2.0"><channel><title>Example</title><link>https://example.com</link><description>Example feed</description><item><title>News</title><link>https://example.com/news</link><pubDate>%s</pubDate><description>Summary</description></item></channel></rss>`, time.Now().Format(time.RFC1123Z))
	}))
	defer feedServer.Close()

	cache := feed.NewCache()
	srv := fuego.NewServer()
	refresher := feed.NewRefresher(feed.NewFetcher(), feed.NewParser(), cache)
	NewHandler(srv, cache, refresher).BindRoutes()

	request := httptest.NewRequest(http.MethodGet, "/news?sources="+url.QueryEscape(feedServer.URL), nil)
	response := httptest.NewRecorder()
	srv.Mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET /news without criterion returned %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
}

func TestHandleNewsETagChangesWithResponse(t *testing.T) {
	source := "https://example.com/feed.xml"
	cache := feed.NewCache()
	cache.Set(feed.CacheKey(source), "unchanged-feed-hash", []feed.Article{{
		Title:     "Old title",
		Link:      "https://example.com/article",
		Date:      time.Now(),
		Published: "Thu, 26 Nov 2026 12:00:00 GMT",
	}})
	srv := newNewsTestServer(cache)

	first := serveNewsRequest(srv, source, "")
	if first.Code != http.StatusOK {
		t.Fatalf("initial request returned %d, want %d: %s", first.Code, http.StatusOK, first.Body.String())
	}
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("initial response did not include ETag")
	}

	cache.Set(feed.CacheKey(source), "unchanged-feed-hash", []feed.Article{{
		Title:     "Updated title",
		Link:      "https://example.com/article",
		Date:      time.Now(),
		Published: "Thu, 26 Nov 2026 12:00:00 GMT",
	}})
	updated := serveNewsRequest(srv, source, etag)
	if updated.Code != http.StatusOK {
		t.Fatalf("changed response returned %d, want %d: %s", updated.Code, http.StatusOK, updated.Body.String())
	}
	if !strings.Contains(updated.Body.String(), "Updated title") {
		t.Fatalf("response did not contain updated article: %s", updated.Body.String())
	}
}

func TestHandleNewsNotModifiedIncludesCacheHeaders(t *testing.T) {
	source := "https://example.com/feed.xml"
	cache := feed.NewCache()
	cache.Set(feed.CacheKey(source), "feed-hash", []feed.Article{{
		Title:     "News",
		Link:      "https://example.com/article",
		Date:      time.Now(),
		Published: "Thu, 26 Nov 2026 12:00:00 GMT",
	}})
	srv := newNewsTestServer(cache)

	first := serveNewsRequest(srv, source, "")
	if first.Code != http.StatusOK {
		t.Fatalf("initial request returned %d, want %d: %s", first.Code, http.StatusOK, first.Body.String())
	}

	validated := serveNewsRequest(srv, source, first.Header().Get("ETag"))
	if validated.Code != http.StatusNotModified {
		t.Fatalf("conditional request returned %d, want %d: %s", validated.Code, http.StatusNotModified, validated.Body.String())
	}
	if validated.Body.Len() != 0 {
		t.Errorf("304 response body = %q, want no body", validated.Body.String())
	}
	if got := validated.Header().Get("ETag"); got != first.Header().Get("ETag") {
		t.Errorf("304 ETag = %q, want %q", got, first.Header().Get("ETag"))
	}
	if got := validated.Header().Get("Cache-Control"); got != "max-age=60, stale-while-revalidate=300" {
		t.Errorf("304 Cache-Control = %q, want cache directives", got)
	}
}

func TestIfNoneMatchMatchesWeakAndListedValidators(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   bool
	}{
		{name: "exact", header: `"current"`, want: true},
		{name: "weak", header: `W/"current"`, want: true},
		{name: "list", header: `"other", W/"current"`, want: true},
		{name: "wildcard", header: `*`, want: true},
		{name: "different", header: `"other"`, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ifNoneMatchMatches(test.header, `"current"`); got != test.want {
				t.Errorf("ifNoneMatchMatches(%q) = %t, want %t", test.header, got, test.want)
			}
		})
	}
}

func newNewsTestServer(cache *feed.Cache) *fuego.Server {
	srv := fuego.NewServer()
	refresher := feed.NewRefresher(feed.NewFetcher(), feed.NewParser(), cache)
	NewHandler(srv, cache, refresher).BindRoutes()
	return srv
}

func serveNewsRequest(srv *fuego.Server, source, etag string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/news?sources="+url.QueryEscape(source), nil)
	if etag != "" {
		request.Header.Set("If-None-Match", etag)
	}
	response := httptest.NewRecorder()
	srv.Mux.ServeHTTP(response, request)
	return response
}
