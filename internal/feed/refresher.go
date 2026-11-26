package feed

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type Refresher struct {
	mu      sync.RWMutex
	known   map[string]struct{}
	fetcher *Fetcher
	parser  *Parser
	cache   *Cache
}

func NewRefresher(fetcher *Fetcher, parser *Parser, cache *Cache) *Refresher {
	return &Refresher{
		known:   make(map[string]struct{}),
		fetcher: fetcher,
		parser:  parser,
		cache:   cache,
	}
}

func (r *Refresher) EnsureSources(sources []string) {
	r.mu.Lock()
	var toRefresh []string
	for _, source := range sources {
		if _, known := r.known[source]; !known {
			r.known[source] = struct{}{}
			toRefresh = append(toRefresh, source)
		}
	}
	r.mu.Unlock()

	if len(toRefresh) > 0 {
		slog.Info("refresher: new sources registered", "quantity", len(toRefresh))
	}
	for _, source := range toRefresh {
		go r.refreshSource(source)
	}
}

func (r *Refresher) GetOrRefresh(source string) ([]Article, error) {
	key := CacheKey(source)

	r.mu.Lock()
	if _, known := r.known[source]; !known {
		r.known[source] = struct{}{}
	}
	r.mu.Unlock()

	if entry, ok := r.cache.Get(key); ok {
		return entry.Articles, nil
	}

	return r.refreshSource(source)
}

func (r *Refresher) Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("refresher: stopping")
			return
		case <-ticker.C:
			r.refreshAll()
		}
	}
}

func (r *Refresher) refreshAll() {
	r.mu.RLock()
	sources := make([]string, 0, len(r.known))
	for source := range r.known {
		sources = append(sources, source)
	}
	r.mu.RUnlock()

	slog.Info("refresher: refreshing all", "sources", len(sources))
	for _, source := range sources {
		go r.refreshSource(source)
	}
}

func (r *Refresher) refreshSource(source string) ([]Article, error) {
	key := CacheKey(source)

	results := r.fetcher.FetchFromURLs([]string{source})
	if len(results) == 0 {
		err := fmt.Errorf("no content returned")
		slog.Error("refresh: no content returned",
			"source", source,
		)
		return r.fallbackToCache(key), err
	}

	res := results[0]
	if res.Err != nil {
		slog.Error("refresh: fetch failed, will retry in next cycle",
			"source", source,
			"error", res.Err,
		)
		return r.fallbackToCache(key), res.Err
	}

	arts, err := r.parser.ParseFeed(res)
	if err != nil {
		slog.Error("refresh: parse failed, will retry in next cycle",
			"source", source,
			"error", err,
		)
		return r.fallbackToCache(key), err
	}

	if len(arts) == 0 {
		slog.Info("refresh: source has no items at the moment, keeping cache",
			"source", source,
		)
		return r.fallbackToCache(key), nil
	}

	hash := HashArticles(arts)
	if entry, ok := r.cache.Get(key); ok && entry.Hash == hash {
		slog.Info("refresh: no changes, keeping cache",
			"source", source,
		)
		return entry.Articles, nil
	}

	r.cache.Set(key, hash, arts)
	slog.Info("refresh: source updated",
		"source", source,
		"articles", len(arts),
	)

	return arts, nil
}

func (r *Refresher) fallbackToCache(key string) []Article {
	if entry, ok := r.cache.Get(key); ok {
		return entry.Articles
	}
	return nil
}
