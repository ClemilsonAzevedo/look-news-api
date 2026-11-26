package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/clemilsonazevedo/look-news/internal/feed"
	"github.com/go-fuego/fuego"
)

func (h *Handler) HandleNews(c fuego.ContextNoBody) (any, error) {
	sources := c.QueryParamArr("sources")

	if len(sources) == 0 {
		slog.Error("no sources provided")
		return nil, fuego.BadRequestError{
			Title:  "Missing required param",
			Detail: "sources is required",
			Errors: []fuego.ErrorItem{
				{
					Name:   "sources",
					Reason: "This query parameter is required",
				},
			},
		}
	}

	var articles []feed.Article
	var failures []string

	for _, url := range sources {
		arts, err := h.refresher.GetOrRefresh(url)

		if err != nil {
			slog.Warn(
				"source failed, falling back to cache",
				"source", url,
				"error", err,
			)
			if len(arts) == 0 {
				failures = append(failures, url)
			}
		}

		if len(arts) > 0 {
			articles = append(articles, arts...)
		}

	}

	if len(failures) == len(sources) {
		slog.Error(
			"all sources failed, no cache available",
			"sources", failures,
		)
		return nil, fuego.HTTPError{
			Title:  "sources unavailable",
			Detail: "all sources failed, no cache available",
			Status: http.StatusServiceUnavailable,
		}
	}

	articles = feed.FilterCurrentWeek(articles, time.Now())

	result := NewsRes{
		Articles: articles,
		Total:    len(articles),
	}
	body, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("encode news response for ETag: %w", err)
	}
	hash := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(hash[:]) + `"`
	w := c.Response()
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "max-age=60, stale-while-revalidate=300")

	if ifNoneMatchMatches(strings.Join(c.Request().Header.Values("If-None-Match"), ","), etag) {
		c.SetStatus(http.StatusNotModified)
		return nil, nil
	}

	return result, nil
}

func ifNoneMatchMatches(header, etag string) bool {
	for index := 0; index < len(header); {
		for index < len(header) && (header[index] == ' ' || header[index] == '\t' || header[index] == ',') {
			index++
		}
		if index == len(header) {
			break
		}
		if header[index] == '*' {
			return true
		}

		start := index
		if strings.HasPrefix(header[index:], "W/") {
			index += 2
		}
		if index == len(header) || header[index] != '"' {
			for index < len(header) && header[index] != ',' {
				index++
			}
			continue
		}
		index++
		for index < len(header) && header[index] != '"' {
			index++
		}
		if index == len(header) {
			break
		}
		index++
		candidate := header[start:index]
		if strings.HasPrefix(candidate, "W/") {
			candidate = strings.TrimPrefix(candidate, "W/")
		}
		if candidate == etag {
			return true
		}
	}
	return false
}
